package cli

import (
	"context"
	"errors"
	"io"

	"github.com/bvarnai/git-brx/internal/domain"
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
	WorkingDir string
	RootCmd    *cobra.Command
	UI         *ui.UI
	Runner     git.Runner
	Inspector  *git.Inspector
	Operations *git.Operations
	Opts       GlobalOptions
}

// NewApp initializes the Cobra command tree and sets up global flags.
func NewApp(stdout, stderr io.Writer, runner git.Runner) *App {
	opts := GlobalOptions{}

	app := &App{
		Opts: opts,
		UI:   ui.New(stdout, stderr, false, false, false),
	}

	if runner == nil {
		runner = git.NewExecRunner(func(gitCmd string) {
			if app.UI != nil {
				app.UI.Debug("executing: %s", gitCmd)
			}
		})
	}
	app.Runner = runner
	app.Inspector = git.NewInspector(runner)
	app.Operations = git.NewOperations(runner)

	rootCmd := &cobra.Command{
		Use:   "git-brx",
		Short: "Opinionated Git extension for local branch workflows",
		Long: `git-brx provides fast, opinionated automation for local branch lifecycles,
upstream synchronization, clean rebasing, and Atlassian toolchain integration.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			app.UI = ui.New(stdout, stderr, app.Opts.NoColor, app.Opts.Quiet, app.Opts.Verbose)
		},
	}

	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)

	// Persistent global flags
	rootCmd.PersistentFlags().BoolVarP(&app.Opts.Verbose, "verbose", "v", false, "Enable verbose diagnostic output")
	rootCmd.PersistentFlags().BoolVarP(&app.Opts.Quiet, "quiet", "q", false, "Suppress informational notices and hints")
	rootCmd.PersistentFlags().BoolVarP(&app.Opts.DryRun, "dry-run", "n", false, "Simulate execution without modifying Git state or remote services")
	rootCmd.PersistentFlags().BoolVar(&app.Opts.NoColor, "no-color", false, "Disable ANSI color styling")

	// Register subcommands
	rootCmd.AddCommand(app.newSelectCmd())
	rootCmd.AddCommand(app.newSyncCmd())
	rootCmd.AddCommand(app.newUpdateCmd())
	rootCmd.AddCommand(app.newResetCmd())
	rootCmd.AddCommand(app.newResolveCmd())

	app.RootCmd = rootCmd
	return app
}

// Execute runs the Cobra command tree with the provided context.
func (a *App) Execute(ctx context.Context) error {
	return a.RootCmd.ExecuteContext(ctx)
}

// Run is the top-level application runner called by main.go.
// It executes the command tree, formats errors to stderr via UI, and returns an exit code.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	return RunInDir(ctx, "", args, stdout, stderr)
}

// RunInDir runs the application within a specific working directory.
func RunInDir(ctx context.Context, dir string, args []string, stdout, stderr io.Writer) int {
	app := NewApp(stdout, stderr, nil)
	app.WorkingDir = dir
	app.RootCmd.SetArgs(args)

	err := app.Execute(ctx)
	if err == nil {
		return int(domain.ExitSuccess)
	}

	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		app.UI.Error("%s", appErr.Message)
		if appErr.Hint != "" {
			app.UI.Hint("%s", appErr.Hint)
		}
		return int(appErr.Code)
	}

	app.UI.Error("%v", err)
	return int(domain.ExitUsageError)
}
