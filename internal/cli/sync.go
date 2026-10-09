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
	Merge       bool
	Rebase      bool
	Autostash   bool
	Interactive bool
}

// newSyncCmd constructs the 'sync' subcommand.
func (a *App) newSyncCmd() *cobra.Command {
	opts := SyncOptions{}

	cmd := &cobra.Command{
		Use:   "sync [flags] [<branch>]",
		Short: "Synchronize active branch with target base branch (default: 'master')",
		Long: `Synchronizes current branch with base branch using rebase (for issue branches)
or merge (for feature/epic branches).`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runSync(cmd.Context(), opts, args)
		},
	}

	cmd.Flags().BoolVarP(&opts.Merge, "merge", "m", false, "Force merge strategy regardless of branch type default")
	cmd.Flags().BoolVarP(&opts.Rebase, "rebase", "r", false, "Force rebase strategy regardless of branch type default")
	cmd.Flags().BoolVarP(&opts.Autostash, "autostash", "a", false, "Enable automatic stashing of uncommitted changes (rebase only)")
	cmd.Flags().BoolVarP(&opts.Interactive, "interactive", "i", false, "Launch interactive rebase (rebase only)")

	return cmd
}

func (a *App) runSync(ctx context.Context, opts SyncOptions, args []string) error {
	if len(args) > 1 {
		return domain.NewError(domain.ExitUsageError, "Unexpected argument: %s", args[1])
	}

	// Strategy flag validation
	if opts.Merge && opts.Rebase {
		return domain.NewError(domain.ExitUsageError, "Multiple override sync strategies specified")
	}

	workDir := "."
	if a.WorkingDir != "" {
		workDir = a.WorkingDir
	}

	rootDir, err := checks.InsideWorkTree(ctx, a.Inspector, workDir)
	if err != nil {
		return err
	}

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
		return domain.NewError(domain.ExitUsageError, "Options 'autostash/interactive' are rebase only")
	}

	// Target base branch resolution
	baseBranch := "master"
	if len(args) == 1 && args[0] != "" {
		baseBranch = args[0]
	} else {
		// If base is omitted, use master (or default branch if master not found)
		defBranch := a.Inspector.DefaultBranch(ctx, rootDir)
		if defBranch != "" {
			baseBranch = defBranch
		}
		a.UI.Log("Syncing '%s' branch by default", baseBranch)
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
			return domain.NewError(domain.ExitPreconditionRemote, "Sync failed (git fetch failed)")
		}

		// Verify base branch is up-to-date with origin/<baseBranch>
		_, remoteBaseExists, _ := a.Inspector.BranchExists(ctx, rootDir, baseBranch)
		if remoteBaseExists {
			delta, deltaErr := a.Inspector.ComputeDelta(ctx, rootDir, baseBranch, "origin/"+baseBranch)
			if deltaErr == nil && delta != nil {
				if delta.AheadCount != 0 || delta.BehindCount != 0 {
					return domain.NewError(domain.ExitConflict, "Sync branch '%s' is not up-to-date", baseBranch).
						WithHint("Switch to '%s' branch and use 'git-brx update' to update all changes", baseBranch)
				}
			}
		}
	}

	// Dry run mode
	if a.Opts.DryRun {
		if strategy == "rebase" {
			rebaseArgs := []string{"git", "rebase"}
			if opts.Autostash {
				rebaseArgs = append(rebaseArgs, "--autostash")
			}
			if opts.Interactive {
				rebaseArgs = append(rebaseArgs, "--interactive")
			}
			rebaseArgs = append(rebaseArgs, baseBranch)
			a.UI.Log("Would execute: %s", strings.Join(rebaseArgs, " "))
		} else {
			a.UI.Log("Would execute: git merge -m \"Merge branch '%s' into '%s'\" %s", baseBranch, currBranch.Name, baseBranch)
		}
		return nil
	}

	// Execution
	if strategy == "rebase" {
		if rebaseErr := a.Operations.Rebase(ctx, rootDir, baseBranch, opts.Autostash, opts.Interactive); rebaseErr != nil {
			return domain.NewError(domain.ExitConflict, "Sync failed due to merge conflicts").
				WithHint("Use 'git-brx resolve' in case of merge conflicts or 'git-brx reset' to abort")
		}
	} else {
		mergeMsg := fmt.Sprintf("Merge branch '%s' into '%s'", baseBranch, currBranch.Name)
		if mergeErr := a.Operations.Merge(ctx, rootDir, baseBranch, mergeMsg); mergeErr != nil {
			return domain.NewError(domain.ExitConflict, "Sync failed due to merge conflicts").
				WithHint("Use 'git-brx resolve' in case of merge conflicts or 'git-brx reset' to abort")
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
		return "", domain.NewError(domain.ExitPreconditionRepo, "You must be on 'issue', 'feature' or 'epic' branch")
	}
}
