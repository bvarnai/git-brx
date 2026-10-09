package cli

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/bvarnai/git-brx/internal/checks"
	"github.com/bvarnai/git-brx/internal/config"
	"github.com/bvarnai/git-brx/internal/domain"
	"github.com/bvarnai/git-brx/internal/github"
	"github.com/spf13/cobra"
)

// CreateOptions stores flags for the create command.
type CreateOptions struct {
	Offline bool
	Yes     bool
}

// newCreateCmd constructs the 'create' subcommand.
func (a *App) newCreateCmd() *cobra.Command {
	opts := CreateOptions{}

	cmd := &cobra.Command{
		Use:   "create [flags] <branch>",
		Short: "Create and checkout a new local development branch",
		Long: `Validate an issue against the issue tracker and provision a new topic branch
following repository branching conventions.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runCreate(cmd.Context(), opts, args)
		},
	}

	cmd.Flags().BoolVarP(&opts.Offline, "offline", "o", false, "Skip issue validation and remote Git availability checks")
	cmd.Flags().BoolVarP(&opts.Yes, "yes", "y", false, "Automatically accept confirmation prompts without interactive input")
	cmd.Flags().BoolVarP(&a.Opts.DryRun, "dry-run", "n", false, "Simulate execution without modifying Git state or remote services")

	return cmd
}

func (a *App) runCreate(ctx context.Context, opts CreateOptions, args []string) error {
	// 1. Argument & Repository Assertions
	if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
		return domain.NewError(domain.ExitUsageError, "No branch name specified")
	}
	if len(args) > 1 {
		return domain.NewError(domain.ExitUsageError, "Unexpected argument: %s", args[1])
	}
	targetBranch := strings.TrimSpace(args[0])

	workDir := "."
	if a.WorkingDir != "" {
		workDir = a.WorkingDir
	}

	rootDir, err := checks.InsideWorkTree(ctx, a.Inspector, workDir)
	if err != nil {
		return err
	}

	if err := checks.NoActiveOperation(ctx, a.Inspector, rootDir); err != nil {
		return err
	}

	// 2. Local Branch Collision Check
	localExists, _, _ := a.Inspector.BranchExists(ctx, rootDir, targetBranch)
	if localExists {
		a.UI.Log("Branch '%s' found (local)", targetBranch)
		a.UI.Log("Hint: To select that branch, use 'git-brx select %s' instead", targetBranch)
		return nil
	}

	// 3. Remote Branch Collision Check (Online Mode)
	hasOrigin, originURL, _ := a.Inspector.HasOrigin(ctx, rootDir)
	if !opts.Offline && hasOrigin {
		a.UI.Log("Checking for existing branches (remote)")
		remoteExists, err := a.Inspector.RemoteBranchExistsOnServer(ctx, rootDir, targetBranch)
		if err != nil {
			a.UI.Error("Unable to reach remote")
			a.UI.Log("Hint: Try --offline if working without network access")
			return domain.NewError(domain.ExitPreconditionRemote, "Unable to reach remote; try --offline if working without network access")
		}
		if remoteExists {
			a.UI.Log("Branch '%s' found (remote)", targetBranch)
			a.UI.Log("Hint: To select this branch, use 'git-brx select %s' instead", targetBranch)
			return nil
		}
		a.UI.Log("No branch '%s' found (remote)", targetBranch)
	} else if opts.Offline {
		a.UI.Log("Offline mode selected (git remote checks skipped...)")
	}

	// 4. Configuration & Template Matching
	cfg, err := config.Load(rootDir, originURL)
	if err != nil {
		var appErr *domain.AppError
		if errors.As(err, &appErr) {
			return appErr
		}
		return domain.WrapError(domain.ExitConfigError, err, "Configuration error")
	}

	templatePattern := cfg.Branch.Template
	re, err := regexp.Compile(templatePattern)
	if err != nil {
		return domain.NewError(domain.ExitConfigError, "Invalid branch template regular expression: %s", templatePattern)
	}

	matches := re.FindStringSubmatch(targetBranch)
	if matches == nil {
		return domain.NewError(domain.ExitConfigError, "Branch name '%s' doesn't match pattern %s", targetBranch, templatePattern)
	}

	branchPath, issueKey := extractBranchPathAndKey(matches, targetBranch)

	// 5. Issue Tracker Validation (Online Mode)
	if !opts.Offline {
		tracker, err := a.resolveTracker(cfg)
		if err != nil {
			return err
		}

		if tracker != nil {
			issue, err := tracker.GetIssue(ctx, issueKey)
			if err != nil {
				return err
			}

			// Validate issue type mapping
			if len(cfg.Branch.Mapping) > 0 {
				mappedBranchPath, ok := cfg.Branch.Mapping[issue.Type]
				if !ok {
					return domain.NewError(domain.ExitConfigError, "No branch mapping defined for issue type '%s'", issue.Type)
				}
				if mappedBranchPath != branchPath {
					a.UI.Error("Issue type '%s' is not allowed on '%s' branch", issue.Type, branchPath)
					a.UI.Log("Hint: use '%s' branch or check mapping rules", mappedBranchPath)
					return domain.NewError(domain.ExitConfigError, "Issue type '%s' is not allowed on '%s' branch", issue.Type, branchPath)
				}
			}

			assignee := issue.Assignee
			if assignee == "" {
				assignee = "nobody (not yet assigned)"
			}
			a.UI.Log("Issue '%s' is assigned to '%s' in status '%s'", issue.Key, assignee, issue.Status)

			// Confirmation prompt unless --yes
			if !opts.Yes {
				confirmed, err := a.UI.Confirm("Are you sure [y/n]?")
				if err != nil || !confirmed {
					return domain.NewError(domain.ExitUserAborted, "Branch creation aborted by user")
				}
			}
		}
	} else {
		a.UI.Log("Offline mode selected (issue tracker checks skipped...)")
	}

	// 6. Branch Provisioning
	if a.Opts.DryRun {
		a.UI.Log("Would create branch '%s'", targetBranch)
		return nil
	}

	a.UI.Log("Creating branch '%s'", targetBranch)
	if err := a.Operations.CreateAndCheckout(ctx, rootDir, targetBranch); err != nil {
		return domain.WrapError(domain.ExitGeneralError, err, "Failed to create branch '%s'", targetBranch)
	}

	a.UI.Log("Hint: Use 'git-brx publish' to publish a new branch")
	return nil
}

func extractBranchPathAndKey(matches []string, targetBranch string) (string, string) {
	if len(matches) >= 3 {
		return matches[1], matches[2]
	}
	parts := strings.SplitN(targetBranch, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "issue", targetBranch
}

func (a *App) resolveTracker(cfg *domain.ProjectConfig) (domain.IssueTracker, error) {
	provider := strings.ToLower(cfg.Tracker.Provider)
	switch provider {
	case "github":
		token := github.ResolveToken(context.Background(), a.Runner, "github.com")
		opts := []github.Option{}
		if cfg.Tracker.URI != "" {
			opts = append(opts, github.WithBaseURL(cfg.Tracker.URI))
		}
		return github.NewClient(cfg.Tracker.Owner, cfg.Tracker.Repo, token, opts...), nil
	case "", "none":
		return nil, nil
	default:
		return nil, domain.NewError(domain.ExitConfigError, "Unsupported issue tracker provider: %s", provider)
	}
}
