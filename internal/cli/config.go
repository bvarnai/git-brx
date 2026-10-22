package cli

import (
	"context"
	"strings"

	"github.com/bvarnai/git-brx/internal/config"
	"github.com/bvarnai/git-brx/internal/errcode"
	"github.com/spf13/cobra"
)

// ConfigOptions stores flags for the 'config' command.
type ConfigOptions struct {
	Sources bool
}

// newConfigCmd constructs the 'config' subcommand.
func (a *App) newConfigCmd() *cobra.Command {
	opts := ConfigOptions{}

	cmd := &cobra.Command{
		Use:   "config [flags]",
		Short: "Print the effective resolved configuration",
		Long: `Print the effective cascading configuration resolved across all four tiers
(User Global, Shared Global, Repo Local, and Remote Origin Auto-Discovery) in YAML format.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runConfig(cmd.Context(), opts, args)
		},
	}

	cmd.Flags().BoolVar(&opts.Sources, "sources", false, "Output only the resolved configuration sources hierarchy")

	return cmd
}

func (a *App) runConfig(ctx context.Context, opts ConfigOptions, args []string) error {
	if len(args) > 0 {
		return errcode.NewError(errcode.ErrUsage, "Unexpected argument: %s", args[0])
	}

	workDir := "."
	if a.WorkingDir != "" {
		workDir = a.WorkingDir
	}

	targetDir := workDir
	var originURL string

	if a.Inspector != nil && a.Inspector.IsWorkTree(ctx, workDir) {
		if root, err := a.Inspector.RepoRoot(ctx, workDir); err == nil {
			targetDir = root
			_, url, _ := a.Inspector.HasOrigin(ctx, root)
			originURL = url
		}
	}
	a.Logger.Debug("Inspecting configuration for targetDir=%s (origin=%s, sourcesOnly=%v)", targetDir, originURL, opts.Sources)

	report, err := config.Inspect(targetDir, originURL)
	if err != nil {
		return err
	}

	out, err := config.FormatReport(report, opts.Sources)
	if err != nil {
		return errcode.WrapError(errcode.ErrGeneral, err, "failed to format configuration report")
	}

	a.Logger.Out("%s", strings.TrimRight(out, "\n"))
	return nil
}
