package cli

import (
	"context"
	"strings"

	"github.com/bvarnai/git-brx/internal/checks"
	"github.com/bvarnai/git-brx/internal/domain"
	"github.com/bvarnai/git-brx/internal/suggest"
	"github.com/spf13/cobra"
)

// SelectOptions stores flags for the select command.
type SelectOptions struct {
	Offline bool
}

// newSelectCmd constructs the 'select' subcommand.
func (a *App) newSelectCmd() *cobra.Command {
	opts := SelectOptions{}

	cmd := &cobra.Command{
		Use:   "select [flags] [<branch>]",
		Short: "Select and switch to an existing branch ('master' by default)",
		Long: `Select and switch to an existing branch. In online mode, fetches updates
from origin before switching.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runSelect(cmd.Context(), opts, args)
		},
	}

	cmd.Flags().BoolVarP(&opts.Offline, "offline", "o", false, "Skip remote fetch and switch between existing local references only")

	return cmd
}

func (a *App) runSelect(ctx context.Context, opts SelectOptions, args []string) error {
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
		a.UI.Log("Selecting '%s' branch by default", targetBranch)
	}

	// Handle case where user is already on the target branch
	if prevBranch != "" && prevBranch == targetBranch {
		if !opts.Offline {
			if hasOrigin, _, _ := a.Inspector.HasOrigin(ctx, rootDir); hasOrigin {
				_ = a.Operations.Fetch(ctx, rootDir, "origin")
			}
		}
		a.UI.Log("Already on '%s'", targetBranch)
		checks.HintUpstreamSync(ctx, a.Inspector, a.UI, rootDir, targetBranch)
		return nil
	}

	// Remote synchronization (online mode)
	if !opts.Offline {
		hasOrigin, _, _ := a.Inspector.HasOrigin(ctx, rootDir)
		if hasOrigin {
			fetchErr := a.Operations.Fetch(ctx, rootDir, "origin")
			if fetchErr != nil {
				localExists, _, _ := a.Inspector.BranchExists(ctx, rootDir, targetBranch)
				if localExists {
					a.UI.Warn("Unable to reach remote; falling back to local references")
				} else {
					return domain.NewError(domain.ExitPreconditionRemote, "Select failed (git fetch failed)")
				}
			}
		}
	}

	localExists, remoteExists, _ := a.Inspector.BranchExists(ctx, rootDir, targetBranch)

	// Dry run mode
	if a.Opts.DryRun {
		if !localExists && !remoteExists {
			return a.branchNotFoundError(ctx, rootDir, targetBranch)
		}
		a.UI.Log("Would checkout branch '%s'", targetBranch)
		return nil
	}

	if !localExists && !remoteExists {
		return a.branchNotFoundError(ctx, rootDir, targetBranch)
	}

	checkoutErr := a.Operations.Checkout(ctx, rootDir, targetBranch)
	if checkoutErr != nil {
		if strings.Contains(checkoutErr.Error(), "would be overwritten by checkout") {
			return domain.NewError(domain.ExitConflict, "Select failed: local modifications would be overwritten by checkout").
				WithHint("Commit or stash your changes before switching branches")
		}
		return domain.WrapError(domain.ExitGeneralError, checkoutErr, "Select failed (git checkout %s failed)", targetBranch)
	}

	// Postflight feedback: warn if uncommitted changes were carried over to the new branch
	checks.WarnDirtyWorktree(a.UI, wasDirty, prevBranch, targetBranch)

	return nil
}

func (a *App) branchNotFoundError(ctx context.Context, rootDir, targetBranch string) error {
	appErr := domain.NewError(domain.ExitPreconditionRepo, "Branch '%s' not found", targetBranch)
	candidates, err := a.Inspector.ListAllBranchNames(ctx, rootDir)
	if err == nil && len(candidates) > 0 {
		closest := suggest.Closest(targetBranch, candidates, 3)
		if closest != "" {
			appErr.WithHint("Did you mean '%s'?", closest)
		}
	}
	return appErr
}
