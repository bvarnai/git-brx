package cli

import (
	"context"

	"github.com/bvarnai/git-brx/internal/checks"
	"github.com/bvarnai/git-brx/internal/domain"
	"github.com/spf13/cobra"
)

// ResolveOptions stores flags for the resolve command.
type ResolveOptions struct {
	Tool string
}

// newResolveCmd constructs the 'resolve' subcommand.
func (a *App) newResolveCmd() *cobra.Command {
	opts := ResolveOptions{}

	cmd := &cobra.Command{
		Use:   "resolve [flags]",
		Short: "Launches interactive merge tool and continues in-flight rebase or merge",
		Long: `Identifies unmerged files, launches the configured merge tool without prompt,
stages resolved files specifically, and continues the active rebase or completes the merge.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runResolve(cmd.Context(), opts, args)
		},
	}

	cmd.Flags().StringVarP(&opts.Tool, "tool", "t", "", "Specify explicit merge tool override")

	return cmd
}

func (a *App) runResolve(ctx context.Context, opts ResolveOptions, args []string) error {
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

	op, active := a.Inspector.ActiveOperation(ctx, rootDir)
	if !active || (op != "merge" && op != "rebase") {
		a.UI.Log("No merge/rebase is ongoing")
		return nil
	}

	if op == "merge" {
		return a.resolveMerge(ctx, rootDir, opts.Tool)
	}

	return a.resolveRebase(ctx, rootDir, opts.Tool)
}

func (a *App) resolveMerge(ctx context.Context, rootDir, tool string) error {
	a.UI.Log("Resolving merge conflicts...")
	if err := a.runMergeToolAndStage(ctx, rootDir, tool); err != nil {
		return err
	}

	if err := a.Operations.CommitNoEdit(ctx, rootDir); err != nil {
		return domain.WrapError(domain.ExitConflict, err, "Failed to complete merge commit").
			WithHint("Use 'git-brx reset' to abort the merge and restore your branch")
	}

	a.UI.Log("Merge completed")
	return nil
}

func (a *App) resolveRebase(ctx context.Context, rootDir, tool string) error {
	const maxIterations = 20
	iterations := 0

	for {
		iterations++
		if iterations > maxIterations {
			return domain.NewError(domain.ExitConflict, "Exceeded maximum rebase iterations (%d); aborting loop", maxIterations).
				WithHint("Use 'git-brx reset' to abort the rebase and restore your branch")
		}

		progress, _ := a.Inspector.RebaseProgress(ctx, rootDir)
		if progress != nil && progress.TotalSteps > 0 {
			a.UI.Log("Resolving rebase conflict at step %d of %d...", progress.CurrentStep, progress.TotalSteps)
		} else {
			a.UI.Log("Resolving rebase conflicts...")
		}

		if err := a.runMergeToolAndStage(ctx, rootDir, tool); err != nil {
			return err
		}

		err := a.Operations.RebaseContinue(ctx, rootDir)
		if err == nil {
			// Rebase successfully completed
			a.UI.Log("Rebase completed")
			return nil
		}

		// Check if rebase is still active due to conflicts on subsequent commits
		op, active := a.Inspector.ActiveOperation(ctx, rootDir)
		if active && op == "rebase" {
			a.UI.Log("Conflicts encountered on subsequent commit in rebase series")
			continue
		}

		// Rebase failed for another reason
		return domain.WrapError(domain.ExitConflict, err, "Failed to continue rebase").
			WithHint("Use 'git-brx reset' to abort the rebase and restore your branch")
	}
}

func (a *App) runMergeToolAndStage(ctx context.Context, rootDir, tool string) error {
	unmerged, err := a.Inspector.UnmergedFiles(ctx, rootDir)
	if err != nil {
		return domain.WrapError(domain.ExitGeneralError, err, "Failed to query unmerged files")
	}

	if len(unmerged) > 0 {
		if err := a.Operations.MergeTool(ctx, rootDir, tool); err != nil {
			return domain.WrapError(domain.ExitConflict, err, "Merge tool execution failed")
		}

		// Verify unmerged files after running mergetool
		remaining, err := a.Inspector.UnmergedFiles(ctx, rootDir)
		if err != nil {
			return domain.WrapError(domain.ExitGeneralError, err, "Failed to check unmerged files")
		}
		if len(remaining) > 0 {
			return domain.NewError(domain.ExitConflict, "Conflicts remain unresolved; cannot continue")
		}

		// Explicitly stage only the resolved unmerged paths (never git add .)
		if err := a.Operations.Add(ctx, rootDir, unmerged...); err != nil {
			return domain.WrapError(domain.ExitGeneralError, err, "Failed to stage resolved files")
		}
	}

	return nil
}
