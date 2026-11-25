package cli

import (
	"context"
	"strings"

	"github.com/bvarnai/git-brx/internal/checks"
	"github.com/bvarnai/git-brx/internal/config"
	"github.com/bvarnai/git-brx/internal/domain"
	"github.com/bvarnai/git-brx/internal/errcode"
	"github.com/bvarnai/git-brx/internal/git"
	"github.com/spf13/cobra"
)

// DeleteOptions stores flags for the delete command.
type DeleteOptions struct {
	Force        bool
	DeleteRemote bool
}

// newDeleteCmd constructs the 'delete' subcommand.
func (a *App) newDeleteCmd() *cobra.Command {
	opts := DeleteOptions{}

	cmd := &cobra.Command{
		Use:   "delete [flags] [<branch>]",
		Short: "Delete a work branch after remote merge and prune origin",
		Long: `Safely deletes a work branch (issue/*, feature/*, or epic/*)
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
	cmd.Flags().BoolVar(&opts.DeleteRemote, "delete-remote", false, "Delete remote branch on origin if its pull request is merged")

	return cmd
}

func (a *App) runDelete(ctx context.Context, opts DeleteOptions, args []string) error {
	if len(args) > 1 {
		return errcode.NewError(errcode.ErrUsage, "Unexpected argument: %s", args[1])
	}

	workDir := "."
	if a.WorkingDir != "" {
		workDir = a.WorkingDir
	}

	rootDir, err := checks.InsideWorkTree(ctx, a.Inspector, workDir)
	if err != nil {
		return err
	}
	a.Logger.Debug("Repository root: %s", rootDir)

	if err := checks.HasCommits(ctx, a.Inspector, rootDir); err != nil {
		return err
	}

	hasOrigin, _, err := a.Inspector.HasOrigin(ctx, rootDir)
	if err != nil || !hasOrigin {
		return errcode.NewError(errcode.ErrPreconditionRepo, "Origin remote is not configured")
	}

	currBranch, err := a.Inspector.CurrentBranch(ctx, rootDir)
	if err != nil {
		return errcode.WrapError(errcode.ErrGeneral, err, "Failed to determine current branch")
	}

	var targetBranch string
	var shouldSwitch bool

	if len(args) == 1 {
		targetBranch = strings.TrimSpace(args[0])
		if targetBranch == "" {
			return errcode.NewError(errcode.ErrUsage, "Branch name cannot be empty")
		}

		localExists, _, err := a.Inspector.BranchExists(ctx, rootDir, targetBranch)
		if err != nil {
			return errcode.WrapError(errcode.ErrGeneral, err, "Failed to verify local branch existence")
		}
		if !localExists {
			return errcode.NewError(errcode.ErrPreconditionRepo, "Branch '%s' does not exist locally", targetBranch)
		}

		targetType := git.ClassifyBranch(targetBranch)
		switch targetType {
		case domain.BranchTypeIssue, domain.BranchTypeFeature, domain.BranchTypeEpic:
			// Allowed work branch types
		default:
			return errcode.NewError(errcode.ErrPreconditionRepo, "You can only delete an 'issue', 'feature', or 'epic' branch")
		}

		if !currBranch.IsDetached && currBranch.Name == targetBranch {
			shouldSwitch = true
		}
	} else {
		if currBranch.IsDetached {
			return errcode.NewError(errcode.ErrPreconditionRepo, "Cannot delete branch in detached HEAD state")
		}

		switch currBranch.Type {
		case domain.BranchTypeIssue, domain.BranchTypeFeature, domain.BranchTypeEpic:
			// Allowed work branch types
		default:
			return errcode.NewError(errcode.ErrPreconditionRepo, "You must be on an 'issue', 'feature', or 'epic' branch to delete it")
		}

		targetBranch = currBranch.Name
		shouldSwitch = true
	}
	a.Logger.Debug("Target branch to delete: %s (shouldSwitch=%v, force=%v, deleteRemote=%v)", targetBranch, shouldSwitch, opts.Force, opts.DeleteRemote)

	if !opts.Force {
		existsOnServer, err := a.Inspector.RemoteBranchExistsOnServer(ctx, rootDir, targetBranch)
		if err != nil {
			return errcode.WrapError(errcode.ErrPreconditionRemote, err, "Unable to reach remote; are you offline?")
		}
		if existsOnServer {
			a.Logger.Log("Branch '%s' found on remote origin", targetBranch)

			// Try SCM diagnostic lookup (and optional --delete-remote when MERGED) if configured
			deletedRemote, scmErr := a.checkSCMPullRequestState(ctx, rootDir, targetBranch, opts.DeleteRemote)
			if scmErr != nil {
				return scmErr
			}
			if !deletedRemote {
				return errcode.NewError(errcode.ErrPreconditionRepo, "Branch must be deleted on remote first (e.g., after merging pull request)").
					WithHint("Delete the remote branch on origin first, or use '--force' to delete the local branch anyway")
			}
		}
	}

	// Resolve return branch before deleting targetBranch so branch.<target>.brx-base is still readable
	returnBase := a.resolveBaseBranch(ctx, rootDir, targetBranch, "")
	baseBranch := returnBase.Name

	detached := false
	fastForwarded := false
	if shouldSwitch {
		localReturnExists, remoteReturnExists, _ := a.Inspector.BranchExists(ctx, rootDir, baseBranch)
		inOtherWorktree := localReturnExists && a.Inspector.IsBranchCheckedOutInOtherWorktree(ctx, rootDir, baseBranch)

		if inOtherWorktree || !localReturnExists {
			detachRef := baseBranch
			if remoteReturnExists {
				detachRef = "origin/" + baseBranch
			}
			if detachErr := a.Operations.CheckoutDetach(ctx, rootDir, detachRef); detachErr != nil {
				return errcode.WrapError(errcode.ErrConflict, detachErr, "Unable to switch to '%s' branch", baseBranch)
			}
			detached = true
			a.Logger.Log("Branch '%s' is unavailable for checkout (checked out in another worktree or remote-only); detached HEAD before deleting '%s'", baseBranch, targetBranch)
		} else {
			if checkoutErr := a.Operations.Checkout(ctx, rootDir, baseBranch); checkoutErr != nil {
				return errcode.WrapError(errcode.ErrConflict, checkoutErr, "Unable to switch to '%s' branch", baseBranch)
			}

			// Best-effort auto-fast-forward when clean and strictly behind origin/<baseBranch>
			if isDirty, dirtyErr := a.Inspector.IsDirty(ctx, rootDir); dirtyErr == nil && !isDirty {
				_ = a.Operations.FetchRef(ctx, rootDir, "origin", baseBranch)
				_, hasRemoteReturn, _ := a.Inspector.BranchExists(ctx, rootDir, baseBranch)
				if hasRemoteReturn {
					delta, deltaErr := a.Inspector.ComputeDelta(ctx, rootDir, baseBranch, "origin/"+baseBranch)
					if deltaErr == nil && delta != nil && delta.AheadCount == 0 && delta.BehindCount > 0 {
						if _, ffErr := a.Operations.MergeFFOnly(ctx, rootDir, "origin/"+baseBranch); ffErr == nil {
							fastForwarded = true
							unit := "commits"
							if delta.BehindCount == 1 {
								unit = "commit"
							}
							a.Logger.Log("Fast-forwarded '%s' to match 'origin/%s' (%d new %s)", baseBranch, baseBranch, delta.BehindCount, unit)
						}
					}
				}
			}
		}
	}

	_, branchDelErr := a.Runner.Run(ctx, rootDir, "branch", "-D", targetBranch)
	if branchDelErr != nil {
		return errcode.WrapError(errcode.ErrConflict, branchDelErr, "Failed to delete local branch '%s'", targetBranch)
	}

	_, _ = a.Runner.Run(ctx, rootDir, "remote", "prune", "origin")

	a.Logger.Log("Deleted local branch '%s' and pruned origin", targetBranch)
	if shouldSwitch && !detached && !fastForwarded {
		a.Logger.Hint("Your local '%s' may not be up-to-date. Use 'git-brx update' to update changes", baseBranch)
	}

	return nil
}

func (a *App) checkSCMPullRequestState(ctx context.Context, rootDir, branch string, deleteRemote bool) (bool, error) {
	_, originURL, _ := a.Inspector.HasOrigin(ctx, rootDir)
	cfg, err := config.Load(rootDir, originURL)
	if err != nil || cfg == nil {
		return false, nil
	}

	scm, err := a.resolveSCM(cfg)
	if err != nil || scm == nil {
		return false, nil
	}

	pr, err := scm.GetPullRequestForBranch(ctx, branch)
	if err != nil || pr == nil {
		return false, nil
	}

	switch pr.State {
	case domain.PRStateOpen:
		return false, errcode.NewError(errcode.ErrPreconditionRepo, "Pull request #%d is still OPEN (%s)", pr.Number, pr.URL).
			WithHint("Merge and close the pull request on %s before deleting the branch", scm.Name())
	case domain.PRStateMerged:
		if deleteRemote {
			if delErr := a.Operations.DeleteRemoteBranch(ctx, rootDir, "origin", branch); delErr != nil {
				return false, errcode.WrapError(errcode.ErrPreconditionRemote, delErr, "Unable to delete remote branch 'origin/%s'", branch)
			}
			a.Logger.Log("Deleted remote branch 'origin/%s'", branch)
			return true, nil
		}
		return false, errcode.NewError(errcode.ErrPreconditionRepo, "Pull request #%d is MERGED, but remote branch '%s' has not been deleted yet", pr.Number, branch).
			WithHint("Re-run with '--delete-remote', delete the branch on %s (%s), or use '--force'", scm.Name(), pr.URL)
	case domain.PRStateClosed:
		return false, errcode.NewError(errcode.ErrPreconditionRepo, "Pull request #%d was CLOSED without merging (%s)", pr.Number, pr.URL).
			WithHint("Use '--force' to bypass the remote check if you intend to delete this abandoned branch")
	}

	return false, nil
}
