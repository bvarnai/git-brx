package cli

import (
	"context"

	"github.com/bvarnai/git-brx/internal/checks"
	"github.com/bvarnai/git-brx/internal/errcode"
	"github.com/spf13/cobra"
)

// ResolveOptions stores flags for the resolve command.
type ResolveOptions struct {
	Tool   string
	NoTool bool
}

// newResolveCmd constructs the 'resolve' subcommand.
func (a *App) newResolveCmd() *cobra.Command {
	opts := ResolveOptions{}

	cmd := &cobra.Command{
		Use:   "resolve [flags]",
		Short: "Stage resolved conflicts or launch merge tool, then continue rebase/merge",
		Long: `Identifies unmerged files, auto-stages files whose conflict markers have been
cleared, or launches the configured merge tool when markers remain, then continues
the active rebase or completes the merge.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runResolve(cmd.Context(), opts, args)
		},
	}

	cmd.Flags().StringVarP(&opts.Tool, "tool", "t", "", "Specify explicit merge tool override")
	cmd.Flags().BoolVar(&opts.NoTool, "no-tool", false, "Never launch an external merge tool; fail listing conflicted files if markers remain")

	return cmd
}

func (a *App) runResolve(ctx context.Context, opts ResolveOptions, args []string) error {
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

	op, active := a.Inspector.ActiveOperation(ctx, rootDir)
	a.Logger.Debug("Active operation: %s (active: %v, overrideTool: %q, noTool: %v)", op, active, opts.Tool, opts.NoTool)
	if !active || (op != "merge" && op != "rebase") {
		a.Logger.Log("No merge or rebase operation is currently in progress")
		return nil
	}

	if op == "merge" {
		return a.resolveMerge(ctx, rootDir, opts)
	}

	return a.resolveRebase(ctx, rootDir, opts)
}

func (a *App) resolveMerge(ctx context.Context, rootDir string, opts ResolveOptions) error {
	a.Logger.Log("Resolving merge conflicts...")
	if err := a.runMergeToolAndStage(ctx, rootDir, opts); err != nil {
		return err
	}

	if err := a.Operations.CommitNoEdit(ctx, rootDir); err != nil {
		return errcode.WrapError(errcode.ErrConflict, err, "Failed to complete merge commit").
			WithHint("Use 'git-brx reset' to abort the merge without losing commits")
	}

	a.Logger.Log("Merge completed")
	return nil
}

func (a *App) resolveRebase(ctx context.Context, rootDir string, opts ResolveOptions) error {
	const maxIterations = 20
	iterations := 0

	for {
		iterations++
		if iterations > maxIterations {
			return errcode.NewError(errcode.ErrConflict, "Exceeded maximum rebase iterations (%d); aborting loop", maxIterations).
				WithHint("Use 'git-brx reset' to abort the rebase without losing commits")
		}

		progress, _ := a.Inspector.RebaseProgress(ctx, rootDir)
		if progress != nil && progress.TotalSteps > 0 {
			a.Logger.Log("Resolving rebase conflict at step %d of %d...", progress.CurrentStep, progress.TotalSteps)
		} else {
			a.Logger.Log("Resolving rebase conflicts...")
		}

		if err := a.runMergeToolAndStage(ctx, rootDir, opts); err != nil {
			return err
		}

		err := a.Operations.RebaseContinue(ctx, rootDir)
		if err == nil {
			// Rebase successfully completed
			a.Logger.Log("Rebase completed")
			return nil
		}

		// Check if rebase is still active due to conflicts on subsequent commits
		op, active := a.Inspector.ActiveOperation(ctx, rootDir)
		if active && op == "rebase" {
			a.Logger.Log("Conflicts encountered on subsequent commit in rebase series")
			continue
		}

		// Rebase failed for another reason
		return errcode.WrapError(errcode.ErrConflict, err, "Failed to continue rebase").
			WithHint("Use 'git-brx reset' to abort the rebase without losing commits")
	}
}

func (a *App) runMergeToolAndStage(ctx context.Context, rootDir string, opts ResolveOptions) error {
	unmerged, err := a.Inspector.UnmergedFiles(ctx, rootDir)
	if err != nil {
		return errcode.WrapError(errcode.ErrGeneral, err, "Failed to query unmerged files")
	}

	if len(unmerged) == 0 {
		return nil
	}

	conflicted, err := a.findConflictedFiles(rootDir, unmerged)
	if err != nil {
		return errcode.WrapError(errcode.ErrGeneral, err, "Failed to inspect conflict markers")
	}

	if len(conflicted) > 0 {
		skipTool := opts.NoTool
		if !skipTool && opts.Tool == "" {
			_, configured := a.Inspector.MergeToolConfigured(ctx, rootDir)
			skipTool = !configured
		}
		if skipTool {
			return a.unresolvedConflictsError(conflicted)
		}

		if err := a.Operations.MergeTool(ctx, rootDir, opts.Tool); err != nil {
			return errcode.WrapError(errcode.ErrConflict, err, "Merge tool execution failed")
		}

		// Verify unmerged index entries and conflict markers after running mergetool
		remaining, err := a.Inspector.UnmergedFiles(ctx, rootDir)
		if err != nil {
			return errcode.WrapError(errcode.ErrGeneral, err, "Failed to check unmerged files")
		}
		if len(remaining) > 0 {
			return a.unresolvedConflictsError(remaining)
		}

		postToolConflicted, err := a.findConflictedFiles(rootDir, unmerged)
		if err != nil {
			return errcode.WrapError(errcode.ErrGeneral, err, "Failed to inspect conflict markers")
		}
		if len(postToolConflicted) > 0 {
			return a.unresolvedConflictsError(postToolConflicted)
		}
	} else {
		a.Logger.Log("All conflict markers resolved; staging %d file(s)", len(unmerged))
	}

	// Explicitly stage only the resolved unmerged paths (never git add .)
	if err := a.Operations.Add(ctx, rootDir, unmerged...); err != nil {
		return errcode.WrapError(errcode.ErrGeneral, err, "Failed to stage resolved files")
	}

	return nil
}

func (a *App) findConflictedFiles(rootDir string, paths []string) ([]string, error) {
	var conflicted []string
	for _, p := range paths {
		hasMarkers, scannable, err := a.Inspector.HasConflictMarkers(rootDir, p)
		if err != nil {
			return nil, err
		}
		if hasMarkers || !scannable {
			conflicted = append(conflicted, p)
		}
	}
	return conflicted, nil
}

func (a *App) unresolvedConflictsError(paths []string) error {
	a.Logger.Log("Unresolved conflict(s) remain in:")
	for _, p := range paths {
		a.Logger.Log("  - %s", p)
	}
	return errcode.NewError(errcode.ErrConflict, "Conflicts remain unresolved; cannot continue").
		WithHint("Resolve conflict markers in the listed files and re-run 'git-brx resolve', or run 'git-brx reset' to abort")
}
