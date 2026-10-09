package cli

import (
	"context"

	"github.com/bvarnai/git-brx/internal/checks"
	"github.com/bvarnai/git-brx/internal/domain"
	"github.com/spf13/cobra"
)

// ResetOptions stores flags for the reset command.
type ResetOptions struct {
	Clean bool
}

// newResetCmd constructs the 'reset' subcommand.
func (a *App) newResetCmd() *cobra.Command {
	opts := ResetOptions{}

	cmd := &cobra.Command{
		Use:   "reset [flags]",
		Short: "Aborts in-flight rebase/merge and hard-resets working copy to origin",
		Long: `Aborts any active in-flight merge or rebase operation, and hard-resets
the working copy to the remote tracking branch (origin/<branch>).
Optionally cleans untracked files with --clean.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runReset(cmd.Context(), opts, args)
		},
	}

	cmd.Flags().BoolVarP(&opts.Clean, "clean", "c", false, "Remove untracked files and directories via git clean -fd")

	return cmd
}

func (a *App) runReset(ctx context.Context, opts ResetOptions, args []string) error {
	if len(args) > 0 {
		return domain.NewError(domain.ExitUsageError, "Unexpected argument: %s", args[0])
	}

	workDir := "."
	if a.WorkingDir != "" {
		workDir = a.WorkingDir
	}

	rootDir, err := checks.InsideWorkTree(ctx, a.Inspector, workDir)
	if err != nil {
		return err
	}

	if err := checks.HasOrigin(ctx, a.Inspector, rootDir); err != nil {
		return err
	}

	// Abort in-flight operations first so that any detached HEAD due to rebase is restored
	op, active := a.Inspector.ActiveOperation(ctx, rootDir)
	if active {
		if op == "merge" {
			a.UI.Log("You are in the middle of a merge, aborting")
			if abortErr := a.Operations.AbortMerge(ctx, rootDir); abortErr != nil {
				return domain.WrapError(domain.ExitConflict, abortErr, "Reset failed (git merge --abort failed)")
			}
		} else if op == "rebase" {
			a.UI.Log("You are in the middle of a rebase, aborting")
			if abortErr := a.Operations.AbortRebase(ctx, rootDir); abortErr != nil {
				return domain.WrapError(domain.ExitConflict, abortErr, "Reset failed (git rebase --abort failed)")
			}
		}
	}

	currBranch, err := checks.AttachedBranch(ctx, a.Inspector, rootDir)
	if err != nil {
		return err
	}

	// Verify remote tracking reference exists
	remoteRef := "origin/" + currBranch.Name
	_, remoteExists, _ := a.Inspector.BranchExists(ctx, rootDir, currBranch.Name)
	if !remoteExists {
		return domain.NewError(domain.ExitPreconditionRemote, "Reset failed: remote reference '%s' does not exist", remoteRef).
			WithHint("Your branch may not have been published yet. Use 'git-brx publish' to publish it")
	}

	// Dry run mode
	if a.Opts.DryRun {
		a.UI.Log("Would execute: git reset --hard %s", remoteRef)
		if opts.Clean {
			a.UI.Log("Would execute: git clean -fd")
		}
		return nil
	}

	// Hard reset to remote ref
	if resetErr := a.Operations.ResetHard(ctx, rootDir, remoteRef); resetErr != nil {
		return domain.WrapError(domain.ExitGeneralError, resetErr, "Reset failed (git reset --hard %s failed)", remoteRef)
	}

	// Clean untracked files if requested
	if opts.Clean {
		if cleanErr := a.Operations.Clean(ctx, rootDir); cleanErr != nil {
			return domain.WrapError(domain.ExitGeneralError, cleanErr, "Reset failed (git clean -fd failed)")
		}
	}

	a.UI.Log("Working tree successfully reset to '%s'", remoteRef)
	return nil
}
