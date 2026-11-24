package cli

import (
	"context"
	"strings"

	"github.com/bvarnai/git-brx/internal/checks"
	"github.com/bvarnai/git-brx/internal/errcode"
	"github.com/bvarnai/git-brx/internal/suggest"
	"github.com/spf13/cobra"
)

// SelectOptions stores flags for the select command.
type SelectOptions struct {
	Offline bool
	NoFF    bool
}

// newSelectCmd constructs the 'select' subcommand.
func (a *App) newSelectCmd() *cobra.Command {
	opts := SelectOptions{}

	cmd := &cobra.Command{
		Use:   "select [flags] [<branch>]",
		Short: "Select and switch to an existing branch (default: repository base branch)",
		Long: `Select and switch to an existing branch. In online mode, fetches updates
from origin before switching and fast-forwards clean branches when strictly behind.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runSelect(cmd.Context(), opts, args)
		},
		ValidArgsFunction: a.completeBranchNames,
	}

	cmd.Flags().BoolVarP(&opts.Offline, "offline", "o", false, "Skip remote fetch and switch between existing local references only")
	cmd.Flags().BoolVar(&opts.NoFF, "no-ff", false, "Skip automatic fast-forward when the selected branch is strictly behind origin")

	return cmd
}

func (a *App) runSelect(ctx context.Context, opts SelectOptions, args []string) error {
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

	// Preflight checks: ensure no active operations and detached HEAD won't orphan commits
	if err := checks.NoActiveOperation(ctx, a.Inspector, rootDir); err != nil {
		return err
	}
	if err := checks.NoOrphanedCommits(ctx, a.Inspector, rootDir); err != nil {
		return err
	}

	// Capture state before switching
	wasDirty, _ := a.Inspector.IsDirty(ctx, rootDir)
	prevBranch := ""
	if curr, err := a.Inspector.CurrentBranch(ctx, rootDir); err == nil && !curr.IsDetached {
		prevBranch = curr.Name
	}

	// Resolve target branch
	targetBranch := ""
	if len(args) == 1 && args[0] != "" {
		targetBranch = args[0]
	} else {
		targetBranch = a.Inspector.DefaultBranch(ctx, rootDir)
		a.Logger.Log("Selecting '%s' branch by default", targetBranch)
	}
	a.Logger.Debug("Branch switch: prev=%s, target=%s, dirty=%v", prevBranch, targetBranch, wasDirty)

	// Handle case where user is already on the target branch
	if prevBranch != "" && prevBranch == targetBranch {
		fetchFailed := false
		if !opts.Offline {
			if hasOrigin, _, _ := a.Inspector.HasOrigin(ctx, rootDir); hasOrigin {
				if err := a.Operations.Fetch(ctx, rootDir, "origin"); err != nil {
					fetchFailed = true
				}
			}
		}
		a.Logger.Log("Already on '%s'", targetBranch)
		a.syncOrHintUpstream(ctx, rootDir, targetBranch, opts.Offline || fetchFailed, opts.NoFF)
		return nil
	}

	// Remote synchronization (online mode)
	fetchOK := !opts.Offline
	if !opts.Offline {
		hasOrigin, _, _ := a.Inspector.HasOrigin(ctx, rootDir)
		if hasOrigin {
			fetchErr := a.Operations.Fetch(ctx, rootDir, "origin")
			if fetchErr != nil {
				fetchOK = false
				localExists, _, _ := a.Inspector.BranchExists(ctx, rootDir, targetBranch)
				if localExists {
					a.Logger.Warn("Unable to reach remote; falling back to local references")
				} else {
					return errcode.NewError(errcode.ErrPreconditionRemote, "Select failed: unable to fetch from origin")
				}
			}
		} else {
			fetchOK = false
		}
	}

	localExists, remoteExists, _ := a.Inspector.BranchExists(ctx, rootDir, targetBranch)
	a.Logger.Debug("Target branch %s presence: local=%v, remote=%v", targetBranch, localExists, remoteExists)

	if !localExists && !remoteExists {
		return a.branchNotFoundError(ctx, rootDir, targetBranch)
	}

	checkoutErr := a.Operations.Checkout(ctx, rootDir, targetBranch)
	if checkoutErr != nil {
		if strings.Contains(checkoutErr.Error(), "would be overwritten by checkout") {
			return errcode.NewError(errcode.ErrConflict, "Select failed: local modifications would be overwritten by checkout").
				WithHint("Commit or stash your changes before switching branches")
		}
		return errcode.WrapError(errcode.ErrGeneral, checkoutErr, "Select failed: could not checkout branch '%s'", targetBranch)
	}

	// Postflight feedback: warn if uncommitted changes were carried over to the new branch
	a.warnDirtyWorktree(wasDirty, prevBranch, targetBranch)
	a.syncOrHintUpstream(ctx, rootDir, targetBranch, opts.Offline || !fetchOK, opts.NoFF)

	return nil
}

func (a *App) warnDirtyWorktree(wasDirty bool, prevBranch, newBranch string) {
	if wasDirty && prevBranch != "" && newBranch != "" && prevBranch != newBranch {
		a.Logger.Warn("You have uncommitted local changes that were carried over to '%s'.", newBranch)
		a.Logger.Hint("If this was unintentional, run 'git-brx select %s' and commit or stash first.", prevBranch)
	}
}

func (a *App) syncOrHintUpstream(ctx context.Context, rootDir, branch string, offline, noFF bool) {
	if offline {
		return
	}
	_, remoteExists, _ := a.Inspector.BranchExists(ctx, rootDir, branch)
	if !remoteExists {
		return
	}

	delta, err := a.Inspector.ComputeDelta(ctx, rootDir, branch, "origin/"+branch)
	if err != nil || delta == nil || delta.BehindCount == 0 {
		return
	}

	if !noFF && delta.AheadCount == 0 {
		if dirty, dirtyErr := a.Inspector.IsDirty(ctx, rootDir); dirtyErr == nil && !dirty {
			if _, ffErr := a.Operations.MergeFFOnly(ctx, rootDir, "origin/"+branch); ffErr == nil {
				unit := "commits"
				if delta.BehindCount == 1 {
					unit = "commit"
				}
				a.Logger.Log("Fast-forwarded '%s' to match 'origin/%s' (%d new %s)", branch, branch, delta.BehindCount, unit)
				return
			}
		}
	}

	a.Logger.Hint("Your branch is behind 'origin/%s' by %d commit(s). Run 'git-brx update' to update.", branch, delta.BehindCount)
}

func (a *App) branchNotFoundError(ctx context.Context, rootDir, targetBranch string) error {
	appErr := errcode.NewError(errcode.ErrPreconditionRepo, "Branch '%s' not found", targetBranch)
	candidates, err := a.Inspector.ListAllBranchNames(ctx, rootDir)
	if err == nil && len(candidates) > 0 {
		closest := suggest.Closest(targetBranch, candidates, 3)
		if closest != "" {
			appErr.WithHint("Did you mean '%s'?", closest)
		}
	}
	return appErr
}
