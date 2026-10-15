package cli

import (
	"context"
	"strings"

	"github.com/bvarnai/git-brx/internal/checks"
	"github.com/bvarnai/git-brx/internal/config"
	"github.com/bvarnai/git-brx/internal/domain"
	"github.com/bvarnai/git-brx/internal/git"
	"github.com/bvarnai/git-brx/internal/github"
	"github.com/spf13/cobra"
)

// DeleteOptions stores flags for the delete command.
type DeleteOptions struct {
	Force bool
}

// newDeleteCmd constructs the 'delete' subcommand.
func (a *App) newDeleteCmd() *cobra.Command {
	opts := DeleteOptions{}

	cmd := &cobra.Command{
		Use:   "delete [flags] [<branch>]",
		Short: "Delete a topic branch after remote merge and prune origin",
		Long: `Safely deletes a topic branch (issue/*, feature/*, or epic/*)
after it has been merged and removed on origin.

If no branch is specified, the current active branch is deleted. Checks that
the remote branch has already been deleted on origin before deleting the local
branch and pruning stale remote tracking references. Use --force to override
remote verification if deleting an abandoned branch.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runDelete(cmd.Context(), opts, args)
		},
		ValidArgsFunction: a.completeBranchNames,
	}

	cmd.Flags().BoolVarP(&opts.Force, "force", "f", false, "Skip verification that the remote branch has already been deleted on origin")

	return cmd
}

func (a *App) runDelete(ctx context.Context, opts DeleteOptions, args []string) error {
	if len(args) > 1 {
		return domain.NewError(domain.ExitUsageError, "Unexpected argument: %s", args[1])
	}

	workDir := "."
	if a.WorkingDir != "" {
		workDir = a.WorkingDir
	}

	rootDir, err := checks.InsideWorkTree(ctx, a.Inspector, workDir)
	if err != nil {
		return err
	}

	if err := checks.HasCommits(ctx, a.Inspector, rootDir); err != nil {
		return err
	}

	hasOrigin, _, err := a.Inspector.HasOrigin(ctx, rootDir)
	if err != nil || !hasOrigin {
		return domain.NewError(domain.ExitPreconditionRepo, "Origin remote is not configured")
	}

	currBranch, err := a.Inspector.CurrentBranch(ctx, rootDir)
	if err != nil {
		return domain.WrapError(domain.ExitGeneralError, err, "Failed to determine current branch")
	}

	var targetBranch string
	var shouldSwitch bool

	if len(args) == 1 {
		targetBranch = strings.TrimSpace(args[0])
		if targetBranch == "" {
			return domain.NewError(domain.ExitUsageError, "Branch name cannot be empty")
		}

		localExists, _, err := a.Inspector.BranchExists(ctx, rootDir, targetBranch)
		if err != nil {
			return domain.WrapError(domain.ExitGeneralError, err, "Failed to verify local branch existence")
		}
		if !localExists {
			return domain.NewError(domain.ExitPreconditionRepo, "Branch '%s' does not exist locally", targetBranch)
		}

		targetType := git.ClassifyBranch(targetBranch)
		switch targetType {
		case domain.BranchTypeIssue, domain.BranchTypeFeature, domain.BranchTypeEpic:
			// Allowed topic branch types
		default:
			return domain.NewError(domain.ExitPreconditionRepo, "You can only delete an 'issue', 'feature', or 'epic' branch")
		}

		if !currBranch.IsDetached && currBranch.Name == targetBranch {
			shouldSwitch = true
		}
	} else {
		if currBranch.IsDetached {
			return domain.NewError(domain.ExitPreconditionRepo, "Cannot delete branch in detached HEAD state")
		}

		switch currBranch.Type {
		case domain.BranchTypeIssue, domain.BranchTypeFeature, domain.BranchTypeEpic:
			// Allowed topic branch types
		default:
			return domain.NewError(domain.ExitPreconditionRepo, "You must be on an 'issue', 'feature', or 'epic' branch to delete it")
		}

		targetBranch = currBranch.Name
		shouldSwitch = true
	}

	if !opts.Force {
		existsOnServer, err := a.Inspector.RemoteBranchExistsOnServer(ctx, rootDir, targetBranch)
		if err != nil {
			return domain.WrapError(domain.ExitPreconditionRemote, err, "Unable to reach remote; are you offline?")
		}
		if existsOnServer {
			a.UI.Log("Branch '%s' found on remote origin", targetBranch)

			// Try SCM diagnostic lookup if configured
			if scmErr := a.checkSCMPullRequestState(ctx, rootDir, targetBranch); scmErr != nil {
				return scmErr
			}

			return domain.NewError(domain.ExitPreconditionRepo, "Branch must be deleted on remote first (e.g., after merging pull request)").
				WithHint("Delete the remote branch on origin first, or use '--force' to delete the local branch anyway")
		}
	}

	baseBranch := a.Inspector.DefaultBranch(ctx, rootDir)
	if baseBranch == "" {
		baseBranch = "main"
	}

	if a.Opts.DryRun {
		if shouldSwitch {
			a.UI.Log("Would checkout branch '%s'", baseBranch)
		}
		a.UI.Log("Would delete local branch '%s'", targetBranch)
		a.UI.Log("Would prune stale tracking references from origin")
		return nil
	}

	if shouldSwitch {
		checkoutErr := a.Operations.Checkout(ctx, rootDir, baseBranch)
		if checkoutErr != nil {
			return domain.WrapError(domain.ExitConflict, checkoutErr, "Unable to switch to '%s' branch", baseBranch)
		}
	}

	_, branchDelErr := a.Runner.Run(ctx, rootDir, "branch", "-D", targetBranch)
	if branchDelErr != nil {
		return domain.WrapError(domain.ExitConflict, branchDelErr, "Failed to delete local branch '%s'", targetBranch)
	}

	_, _ = a.Runner.Run(ctx, rootDir, "remote", "prune", "origin")

	a.UI.Log("Deleted local branch '%s' and pruned origin", targetBranch)
	if shouldSwitch {
		a.UI.Hint("Your local '%s' may not be up-to-date. Use 'git-brx update' to update changes", baseBranch)
	}

	return nil
}

func (a *App) checkSCMPullRequestState(ctx context.Context, rootDir, branch string) error {
	_, originURL, _ := a.Inspector.HasOrigin(ctx, rootDir)
	cfg, err := config.Load(rootDir, originURL)
	if err != nil || cfg == nil {
		return nil
	}

	scm, err := a.resolveSCM(cfg)
	if err != nil || scm == nil {
		return nil
	}

	pr, err := scm.GetPullRequestForBranch(ctx, branch)
	if err != nil || pr == nil {
		return nil
	}

	switch pr.State {
	case domain.PRStateOpen:
		return domain.NewError(domain.ExitPreconditionRepo, "Pull request #%d is still OPEN (%s)", pr.Number, pr.URL).
			WithHint("Merge and close the pull request on %s before deleting the branch", scm.Name())
	case domain.PRStateMerged:
		return domain.NewError(domain.ExitPreconditionRepo, "Pull request #%d is MERGED, but remote branch '%s' has not been deleted yet", pr.Number, branch).
			WithHint("Delete the branch on %s (%s) or use '--force' to clean up the local branch now", scm.Name(), pr.URL)
	case domain.PRStateClosed:
		return domain.NewError(domain.ExitPreconditionRepo, "Pull request #%d was CLOSED without merging (%s)", pr.Number, pr.URL).
			WithHint("Use '--force' to bypass the remote check if you intend to delete this abandoned branch")
	}

	return nil
}

func (a *App) resolveSCM(cfg *domain.ProjectConfig) (domain.SCMProvider, error) {
	provider := strings.ToLower(cfg.SCM.Provider)
	switch provider {
	case "github":
		token := github.ResolveToken(context.Background(), a.Runner, "github.com")
		opts := []github.Option{}
		if cfg.SCM.URI != "" && cfg.SCM.URI != "https://github.com" && cfg.SCM.URI != "http://github.com" {
			opts = append(opts, github.WithBaseURL(cfg.SCM.URI))
		}
		opts = append(opts, github.WithReviewConfig(&cfg.Review))
		return github.NewClient(cfg.SCM.Owner, cfg.SCM.Repo, token, opts...), nil
	case "", "none":
		return nil, nil
	default:
		return nil, nil
	}
}
