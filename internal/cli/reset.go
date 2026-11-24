package cli

import (
	"context"

	"github.com/bvarnai/git-brx/internal/checks"
	"github.com/bvarnai/git-brx/internal/errcode"
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
		Short: "Abort an active rebase/merge safely, or reset idle branch to origin",
		Long: `When a merge or rebase conflict is in progress, aborts the active operation
while preserving local commits. When idle (no active conflict), resets the
working copy to the remote tracking branch (origin/<branch>), optionally
cleaning untracked files with --clean.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runReset(cmd.Context(), opts, args)
		},
	}

	cmd.Flags().BoolVarP(&opts.Clean, "clean", "c", false, "Remove untracked files and directories via git clean -fd (idle mode only)")

	return cmd
}

func (a *App) runReset(ctx context.Context, opts ResetOptions, args []string) error {
	if len(args) > 0 {
		return errcode.NewError(errcode.ErrUsage, "Unexpected argument: %s", args[0])
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

	// Abort in-flight merge or rebase first, preserving unpushed local commits.
	op, active := a.Inspector.ActiveOperation(ctx, rootDir)
	a.Logger.Debug("Active in-flight operation: %s (active: %v)", op, active)
	if active && (op == "merge" || op == "rebase") {
		if opts.Clean {
			return errcode.NewError(errcode.ErrUsage, "Cannot use '--clean' while a %s is in progress", op).
				WithHint("Run 'git-brx reset' to abort the active %s first before using '--clean'", op)
		}

		if op == "merge" {
			a.Logger.Log("Aborting in-progress merge")
			if abortErr := a.Operations.AbortMerge(ctx, rootDir); abortErr != nil {
				return errcode.WrapError(errcode.ErrConflict, abortErr, "Reset failed: unable to abort merge")
			}
		} else {
			a.Logger.Log("Aborting in-progress rebase")
			if abortErr := a.Operations.AbortRebase(ctx, rootDir); abortErr != nil {
				return errcode.WrapError(errcode.ErrConflict, abortErr, "Reset failed: unable to abort rebase")
			}
		}

		a.Logger.Log("Aborted in-progress %s; local commits preserved", op)
		if curr, currErr := a.Inspector.CurrentBranch(ctx, rootDir); currErr == nil && !curr.IsDetached {
			if _, remoteExists, _ := a.Inspector.BranchExists(ctx, rootDir, curr.Name); remoteExists {
				a.Logger.Hint("Run 'git-brx reset' again to discard local changes and match 'origin/%s'", curr.Name)
			}
		}
		return nil
	}

	if err := checks.HasOrigin(ctx, a.Inspector, rootDir); err != nil {
		return err
	}

	currBranch, err := checks.AttachedBranch(ctx, a.Inspector, rootDir)
	if err != nil {
		return err
	}

	// Verify remote tracking reference exists
	remoteRef := "origin/" + currBranch.Name
	a.Logger.Debug("Active branch: %s, target remoteRef: %s (clean=%v)", currBranch.Name, remoteRef, opts.Clean)
	_, remoteExists, _ := a.Inspector.BranchExists(ctx, rootDir, currBranch.Name)
	if !remoteExists {
		return errcode.NewError(errcode.ErrPreconditionRemote, "Reset failed: remote reference '%s' does not exist", remoteRef).
			WithHint("Your branch may not have been published yet. Use 'git-brx publish' to publish it")
	}

	// Hard reset to remote ref
	if resetErr := a.Operations.ResetHard(ctx, rootDir, remoteRef); resetErr != nil {
		return errcode.WrapError(errcode.ErrGeneral, resetErr, "Reset failed: unable to reset working copy to '%s'", remoteRef)
	}

	// Clean untracked files if requested
	if opts.Clean {
		if cleanErr := a.Operations.Clean(ctx, rootDir); cleanErr != nil {
			return errcode.WrapError(errcode.ErrGeneral, cleanErr, "Reset failed: unable to clean untracked files")
		}
	}

	a.Logger.Log("Working tree successfully reset to '%s'", remoteRef)
	return nil
}
