package cli

import (
	"context"
	"io"

	"github.com/bvarnai/git-brx/internal/git"
	"github.com/bvarnai/git-brx/internal/ui"
	"github.com/spf13/cobra"
)

// GlobalOptions stores application-wide flag states.
type GlobalOptions struct {
	Verbose bool
	Quiet   bool
	DryRun  bool
	NoColor bool
}

// App wires all dependencies and provides the root command.
type App struct {
	RootCmd   *cobra.Command
	UI        *ui.UI
	Runner    git.Runner
	Inspector *git.Inspector
	Opts      GlobalOptions
}

// NewApp initializes the Cobra command tree and sets up global flags.
func NewApp(stdout, stderr io.Writer, runner git.Runner) *App {
	opts := GlobalOptions{}

	app := &App{
		Opts:   opts,
		Runner: runner,
	}

	rootCmd := &cobra.Command{
		Use:   "git-brx",
		Short: "Opinionated Git extension for local branch workflows",
		Long: `git-brx provides fast, opinionated automation for local branch lifecycles,
upstream synchronization, clean rebasing, and Atlassian toolchain integration.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			app.UI = ui.New(stdout, stderr, app.Opts.NoColor, app.Opts.Quiet, app.Opts.Verbose)
			if runner == nil {
				app.Runner = git.NewExecRunner(func(gitCmd string) {
					app.UI.Debug("executing: %s", gitCmd)
				})
			}
			app.Inspector = git.NewInspector(app.Runner)
		},
	}

	// Persistent global flags
	rootCmd.PersistentFlags().BoolVarP(&app.Opts.Verbose, "verbose", "v", false, "Enable verbose diagnostic output")
	rootCmd.PersistentFlags().BoolVarP(&app.Opts.Quiet, "quiet", "q", false, "Suppress informational notices and hints")
	rootCmd.PersistentFlags().BoolVarP(&app.Opts.DryRun, "dry-run", "n", false, "Simulate execution without modifying Git state or remote services")
	rootCmd.PersistentFlags().BoolVar(&app.Opts.NoColor, "no-color", false, "Disable ANSI color styling")

	app.RootCmd = rootCmd
	return app
}

// Execute runs the Cobra command tree with the provided context.
func (a *App) Execute(ctx context.Context) error {
	return a.RootCmd.ExecuteContext(ctx)
}
