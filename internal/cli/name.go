package cli

import (
	"context"

	"github.com/bvarnai/git-brx/internal/checks"
	"github.com/bvarnai/git-brx/internal/domain"
	"github.com/spf13/cobra"
)

// newNameCmd constructs the 'name' subcommand.
func (a *App) newNameCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "name",
		Short: "Prints the current active branch name",
		Long: `Prints the current active branch name to stdout. If HEAD is detached,
prints the short commit hash and logs a warning on stderr.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runName(cmd.Context(), args)
		},
	}

	return cmd
}

func (a *App) runName(ctx context.Context, args []string) error {
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

	curr, err := a.Inspector.CurrentBranch(ctx, rootDir)
	if err != nil {
		return domain.WrapError(domain.ExitGeneralError, err, "failed to resolve branch name")
	}

	if curr.IsDetached {
		a.UI.Warn("HEAD is detached at %s", curr.CommitHash)
		a.UI.Out("%s", curr.CommitHash)
		return nil
	}

	a.UI.Out("%s", curr.Name)
	return nil
}
