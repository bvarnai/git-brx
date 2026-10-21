package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/bvarnai/git-brx/internal/checks"
	"github.com/bvarnai/git-brx/internal/domain"
	"github.com/spf13/cobra"
)

// SyncOptions stores flags for the sync command.
type SyncOptions struct {
	Merge            bool
	Rebase           bool
	Autostash        bool
	Interactive      bool
	NoAutoUpdateBase bool
}

// newSyncCmd constructs the 'sync' subcommand.
func (a *App) newSyncCmd() *cobra.Command {
	opts := SyncOptions{}

	cmd := &cobra.Command{
		Use:   "sync [flags] [<branch>]",
		Short: "Synchronize active branch with target base branch (default: repository base branch)",
		Long: `Synchronizes current branch with base branch using rebase (for issue branches)
or merge (for feature/epic branches).`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runSync(cmd.Context(), opts, args)
		},
		ValidArgsFunction: a.completeBranchNames,
	}

	cmd.Flags().BoolVarP(&opts.Merge, "merge", "m", false, "Force merge strategy regardless of branch type default")
	cmd.Flags().BoolVarP(&opts.Rebase, "rebase", "r", false, "Force rebase strategy regardless of branch type default")
	cmd.Flags().BoolVarP(&opts.Autostash, "autostash", "a", false, "Enable automatic stashing of uncommitted changes (rebase only)")
	cmd.Flags().BoolVarP(&opts.Interactive, "interactive", "i", false, "Launch interactive rebase (rebase only)")
	cmd.Flags().BoolVar(&opts.NoAutoUpdateBase, "no-auto-update-base", false, "Disable automatic fast-forwarding of clean base branch")

	return cmd
}

func (a *App) runSync(ctx context.Context, opts SyncOptions, args []string) error {
	if len(args) > 1 {
		return domain.NewError(domain.ExitUsageError, "Unexpected argument: %s", args[1])
	}

	// Strategy flag validation
	if opts.Merge && opts.Rebase {
		return domain.NewError(domain.ExitUsageError, "Multiple override sync strategies specified").
			WithHint("Choose either --merge or --rebase, not both")
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

	// Invariant checks
	if err := checks.NotShallow(ctx, a.Inspector, rootDir); err != nil {
		return err
	}
	if err := checks.NoActiveOperation(ctx, a.Inspector, rootDir); err != nil {
		return err
	}

	currBranch, err := checks.AttachedBranch(ctx, a.Inspector, rootDir)
	if err != nil {
		return err
	}

	// Determine sync strategy based on branch type and overrides
	strategy, err := resolveSyncStrategy(currBranch.Type, opts)
	if err != nil {
		return err
	}

	// Options 'autostash/interactive' are rebase only
	if strategy == "merge" && (opts.Autostash || opts.Interactive) {
		return domain.NewError(domain.ExitUsageError, "Flags --autostash and --interactive are only supported with rebase strategy")
	}

	// Target base branch resolution
	baseBranch := "main"
	isDefaultBase := false
	if len(args) == 1 && args[0] != "" {
		baseBranch = args[0]
	} else {
		// If base is omitted, use main (or default branch)
		defBranch := a.Inspector.DefaultBranch(ctx, rootDir)
		if defBranch != "" {
			baseBranch = defBranch
		}
		isDefaultBase = true
	}
	a.Logger.Debug("Sync configuration: branch=%s, base=%s, strategy=%s", currBranch.Name, baseBranch, strategy)

	if isDefaultBase {
		a.Logger.Log("Syncing '%s' with '%s' base branch by default (strategy: %s)", currBranch.Name, baseBranch, strategy)
	} else {
		a.Logger.Log("Syncing '%s' with '%s' (strategy: %s)", currBranch.Name, baseBranch, strategy)
	}

	if currBranch.Name == baseBranch {
		return domain.NewError(domain.ExitPreconditionRepo, "Cannot sync branch '%s' onto itself", baseBranch)
	}

	// Verify local base branch exists
	localExists, _, _ := a.Inspector.BranchExists(ctx, rootDir, baseBranch)
	if !localExists {
		return domain.NewError(domain.ExitPreconditionRepo, "Base branch '%s' does not exist locally", baseBranch)
	}

	// Remote fetch
	hasOrigin, _, _ := a.Inspector.HasOrigin(ctx, rootDir)
	if hasOrigin {
		if fetchErr := a.Operations.Fetch(ctx, rootDir, "origin"); fetchErr != nil {
			return domain.NewError(domain.ExitPreconditionRemote, "Sync failed: unable to fetch from origin")
		}

		// Verify base branch is up-to-date with origin/<baseBranch>
		_, remoteBaseExists, _ := a.Inspector.BranchExists(ctx, rootDir, baseBranch)
		if remoteBaseExists {
			delta, deltaErr := a.Inspector.ComputeDelta(ctx, rootDir, baseBranch, "origin/"+baseBranch)
			if deltaErr == nil && delta != nil {
				a.Logger.Debug("Base branch delta against origin/%s: ahead=%d, behind=%d", baseBranch, delta.AheadCount, delta.BehindCount)
				if delta.AheadCount > 0 {
					return domain.NewError(domain.ExitConflict, "Base branch '%s' is not up-to-date with 'origin/%s'", baseBranch, baseBranch).
						WithHint("Switch to '%s' and run 'git-brx update' first", baseBranch)
				}
				if delta.BehindCount > 0 {
					if opts.NoAutoUpdateBase {
						return domain.NewError(domain.ExitConflict, "Base branch '%s' is not up-to-date with 'origin/%s'", baseBranch, baseBranch).
							WithHint("Switch to '%s' and run 'git-brx update' first", baseBranch)
					}

					commitUnit := "commits"
					if delta.BehindCount == 1 {
						commitUnit = "commit"
					}

					if err := a.Operations.FetchRef(ctx, rootDir, "origin", fmt.Sprintf("%s:%s", baseBranch, baseBranch)); err != nil {
						return domain.WrapError(domain.ExitConflict, err, "Failed to fast-forward local base branch '%s'", baseBranch)
					}
					a.Logger.Log("Fast-forwarded local '%s' to match 'origin/%s' (%d %s behind)", baseBranch, baseBranch, delta.BehindCount, commitUnit)
				}
			}
		}
	}

	// Execution
	if strategy == "rebase" {
		out, rebaseErr := a.Operations.Rebase(ctx, rootDir, baseBranch, opts.Autostash, opts.Interactive)
		if rebaseErr != nil {
			return domain.NewError(domain.ExitConflict, "Sync failed due to merge conflicts").
				WithHint("Use 'git-brx resolve' to resolve conflicts, or 'git-brx reset' to abort")
		}
		if strings.Contains(strings.ToLower(out), "up to date") {
			a.Logger.Log("Branch '%s' is already up to date with '%s'", currBranch.Name, baseBranch)
		} else {
			a.Logger.Log("Successfully rebased '%s' onto '%s'", currBranch.Name, baseBranch)
		}
	} else {
		mergeMsg := fmt.Sprintf("Merge branch '%s' into '%s'", baseBranch, currBranch.Name)
		out, mergeErr := a.Operations.Merge(ctx, rootDir, baseBranch, mergeMsg)
		if mergeErr != nil {
			return domain.NewError(domain.ExitConflict, "Sync failed due to merge conflicts").
				WithHint("Use 'git-brx resolve' to resolve conflicts, or 'git-brx reset' to abort")
		}
		if strings.Contains(strings.ToLower(out), "already up to date") {
			a.Logger.Log("Branch '%s' is already up to date with '%s'", currBranch.Name, baseBranch)
		} else {
			a.Logger.Log("Successfully merged '%s' into '%s'", baseBranch, currBranch.Name)
		}
	}

	return nil
}

func resolveSyncStrategy(bType domain.BranchType, opts SyncOptions) (string, error) {
	if opts.Merge {
		return "merge", nil
	}
	if opts.Rebase {
		return "rebase", nil
	}

	switch bType {
	case domain.BranchTypeIssue:
		return "rebase", nil
	case domain.BranchTypeFeature, domain.BranchTypeEpic:
		return "merge", nil
	default:
		return "", domain.NewError(domain.ExitPreconditionRepo, "You must be on an 'issue', 'feature', or 'epic' branch to sync")
	}
}
