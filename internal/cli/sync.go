package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/bvarnai/git-brx/internal/checks"
	"github.com/bvarnai/git-brx/internal/domain"
	"github.com/bvarnai/git-brx/internal/errcode"
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
		return errcode.NewError(errcode.ErrUsage, "Unexpected argument: %s", args[1])
	}

	// Strategy flag validation
	if opts.Merge && opts.Rebase {
		return errcode.NewError(errcode.ErrUsage, "Multiple override sync strategies specified").
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
		return errcode.NewError(errcode.ErrUsage, "Flags --autostash and --interactive are only supported with rebase strategy")
	}

	// Fetch from origin upfront so remote tracking refs and recorded remote-only bases are current.
	hasOrigin, _, _ := a.Inspector.HasOrigin(ctx, rootDir)
	if hasOrigin {
		if fetchErr := a.Operations.Fetch(ctx, rootDir, "origin"); fetchErr != nil {
			return errcode.NewError(errcode.ErrPreconditionRemote, "Unable to fetch from origin")
		}
	}

	// Target base branch resolution
	explicitBase := ""
	if len(args) == 1 {
		explicitBase = args[0]
	}
	base := a.resolveBaseBranch(ctx, rootDir, currBranch.Name, explicitBase)
	a.Logger.Debug("Sync configuration: branch=%s, base=%s, ref=%s, strategy=%s", currBranch.Name, base.Name, base.Ref, strategy)

	switch {
	case base.FromConfig:
		a.Logger.Log("Syncing '%s' with recorded base '%s' (strategy: %s)", currBranch.Name, base.Name, strategy)
	case base.FromDefault:
		a.Logger.Log("Syncing '%s' with '%s' base branch by default (strategy: %s)", currBranch.Name, base.Name, strategy)
	default:
		a.Logger.Log("Syncing '%s' with '%s' (strategy: %s)", currBranch.Name, base.Name, strategy)
	}

	if currBranch.Name == base.Name {
		return errcode.NewError(errcode.ErrPreconditionRepo, "Cannot sync branch '%s' onto itself", base.Name)
	}

	// Verify local or remote base branch exists
	localExists, remoteBaseExists, _ := a.Inspector.BranchExists(ctx, rootDir, base.Name)
	if !localExists && !remoteBaseExists {
		return errcode.NewError(errcode.ErrPreconditionRepo, "Base branch '%s' does not exist locally or on origin", base.Name)
	}

	syncRef := base.Name
	if !localExists && remoteBaseExists {
		syncRef = "origin/" + base.Name
	}

	// Verify local base branch is up-to-date with origin/<base> when both exist
	if hasOrigin && localExists && remoteBaseExists {
		delta, deltaErr := a.Inspector.ComputeDelta(ctx, rootDir, base.Name, "origin/"+base.Name)
		if deltaErr == nil && delta != nil {
			a.Logger.Debug("Base branch delta against origin/%s: ahead=%d, behind=%d", base.Name, delta.AheadCount, delta.BehindCount)
			if delta.AheadCount > 0 {
				if delta.BehindCount > 0 {
					return errcode.NewError(errcode.ErrConflict, "Base branch '%s' is not up-to-date with 'origin/%s'", base.Name, base.Name).
						WithHint("Switch to '%s', run 'git-brx update', and then 'git-brx publish' first", base.Name)
				}
				return errcode.NewError(errcode.ErrConflict, "Base branch '%s' is not up-to-date with 'origin/%s'", base.Name, base.Name).
					WithHint("Switch to '%s' and run 'git-brx publish' first", base.Name)
			}
			if delta.BehindCount > 0 {
				if opts.NoAutoUpdateBase {
					return errcode.NewError(errcode.ErrConflict, "Base branch '%s' is not up-to-date with 'origin/%s'", base.Name, base.Name).
						WithHint("Switch to '%s' and run 'git-brx update' first", base.Name)
				}

				if a.Inspector.IsBranchCheckedOutInOtherWorktree(ctx, rootDir, base.Name) {
					a.Logger.Log("Base branch '%s' is checked out in another worktree; syncing directly against 'origin/%s'", base.Name, base.Name)
					syncRef = "origin/" + base.Name
				} else {
					commitUnit := "commits"
					if delta.BehindCount == 1 {
						commitUnit = "commit"
					}

					if err := a.Operations.FetchRef(ctx, rootDir, "origin", fmt.Sprintf("%s:%s", base.Name, base.Name)); err != nil {
						errLower := strings.ToLower(err.Error())
						if strings.Contains(errLower, "checked out at") || strings.Contains(errLower, "worktree") {
							a.Logger.Log("Base branch '%s' is checked out in another worktree; syncing directly against 'origin/%s'", base.Name, base.Name)
							syncRef = "origin/" + base.Name
						} else {
							return errcode.WrapError(errcode.ErrConflict, err, "Failed to fast-forward local base branch '%s'", base.Name)
						}
					} else {
						a.Logger.Log("Fast-forwarded local '%s' to match 'origin/%s' (%d %s behind)", base.Name, base.Name, delta.BehindCount, commitUnit)
					}
				}
			}
		}
	}

	// Execution
	if strategy == "rebase" {
		out, rebaseErr := a.Operations.Rebase(ctx, rootDir, syncRef, opts.Autostash, opts.Interactive)
		if rebaseErr != nil {
			return errcode.NewError(errcode.ErrConflict, "Sync failed due to merge conflicts").
				WithHint("Use 'git-brx resolve' to resolve conflicts, or 'git-brx reset' to abort without losing commits")
		}
		if strings.Contains(strings.ToLower(out), "up to date") {
			a.Logger.Log("Branch '%s' is already up to date with '%s'", currBranch.Name, syncRef)
		} else {
			a.Logger.Log("Successfully rebased '%s' onto '%s'", currBranch.Name, syncRef)
		}
	} else {
		mergeMsg := fmt.Sprintf("Merge branch '%s' into '%s'", syncRef, currBranch.Name)
		out, mergeErr := a.Operations.Merge(ctx, rootDir, syncRef, mergeMsg)
		if mergeErr != nil {
			return errcode.NewError(errcode.ErrConflict, "Sync failed due to merge conflicts").
				WithHint("Use 'git-brx resolve' to resolve conflicts, or 'git-brx reset' to abort without losing commits")
		}
		if strings.Contains(strings.ToLower(out), "already up to date") {
			a.Logger.Log("Branch '%s' is already up to date with '%s'", currBranch.Name, syncRef)
		} else {
			a.Logger.Log("Successfully merged '%s' into '%s'", syncRef, currBranch.Name)
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
		return "", errcode.NewError(errcode.ErrPreconditionRepo, "You must be on an 'issue', 'feature', or 'epic' branch to sync")
	}
}
