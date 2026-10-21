package cli

import (
	"context"
	"errors"
	"io"

	"github.com/bvarnai/git-brx/internal/domain"
	"github.com/bvarnai/git-brx/internal/git"
	"github.com/bvarnai/git-brx/internal/term"
	"github.com/bvarnai/git-brx/internal/version"
	"github.com/spf13/cobra"
)

// GlobalOptions stores application-wide flag states.
type GlobalOptions struct {
	Verbose bool
	Quiet   bool
	NoColor bool
}

// App wires all dependencies and provides the root command.
type App struct {
	WorkingDir string
	RootCmd    *cobra.Command
	Logger     *term.Logger
	Runner     git.Runner
	Inspector  *git.Inspector
	Operations *git.Operations
	Opts       GlobalOptions
}

// NewApp initializes the Cobra command tree and sets up global flags.
func NewApp(stdout, stderr io.Writer, runner git.Runner) *App {
	opts := GlobalOptions{}

	logger := term.New(term.Options{
		Stdout:      stdout,
		Stderr:      stderr,
		Verbose:     false,
		Quiet:       false,
		NoColorFlag: false,
		IsTTY:       term.IsStreamTTY(stderr),
	})

	app := &App{
		Opts:   opts,
		Logger: logger,
	}

	if runner == nil {
		runner = git.NewExecRunner()
	}
	app.Runner = runner
	app.Inspector = git.NewInspector(runner)
	app.Operations = git.NewOperations(runner)

	rootCmd := &cobra.Command{
		Use:     "git-brx",
		Version: version.String(),
		Short:   "Opinionated, simple Git workflow with user-friendly automation",
		Long: `git-brx provides a simple, opinionated Git workflow with user-friendly automation
for local branch lifecycles, upstream synchronization, clean rebasing, and code reviews.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if app.Opts.Verbose && app.Opts.Quiet {
				return domain.NewError(domain.ExitUsageError, "cannot specify both --verbose and --quiet").
					WithHint("Choose either verbose or quiet output mode, not both.")
			}
			app.Logger = term.New(term.Options{
				Stdout:      stdout,
				Stderr:      stderr,
				Verbose:     app.Opts.Verbose,
				Quiet:       app.Opts.Quiet,
				NoColorFlag: app.Opts.NoColor,
				IsTTY:       term.IsStreamTTY(stderr),
			})
			return nil
		},
	}

	rootCmd.SetVersionTemplate("{{.Version}}\n")
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	// Persistent global flags
	rootCmd.PersistentFlags().BoolVarP(&app.Opts.Verbose, "verbose", "v", false, "Enable verbose diagnostic output")
	rootCmd.PersistentFlags().BoolVarP(&app.Opts.Quiet, "quiet", "q", false, "Suppress informational notices and hints")
	rootCmd.PersistentFlags().BoolVar(&app.Opts.NoColor, "no-color", false, "Disable ANSI color styling")

	// Register subcommands
	rootCmd.AddCommand(app.newSelectCmd())
	rootCmd.AddCommand(app.newSyncCmd())
	rootCmd.AddCommand(app.newUpdateCmd())
	rootCmd.AddCommand(app.newResetCmd())
	rootCmd.AddCommand(app.newResolveCmd())
	rootCmd.AddCommand(app.newNameCmd())
	rootCmd.AddCommand(app.newHistoryCmd())
	rootCmd.AddCommand(app.newCreateCmd())
	rootCmd.AddCommand(app.newPublishCmd())
	rootCmd.AddCommand(app.newDeleteCmd())
	rootCmd.AddCommand(app.newReviewCmd())
	rootCmd.AddCommand(app.newConfigCmd())
	rootCmd.AddCommand(app.newCompletionCmd())

	app.RootCmd = rootCmd
	return app
}

// Execute runs the Cobra command tree with the provided context.
func (a *App) Execute(ctx context.Context) error {
	return a.RootCmd.ExecuteContext(ctx)
}

// Run is the top-level application runner called by main.go.
// It executes the command tree, formats errors to stderr via Logger, and returns an exit code.
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

	if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
		return int(domain.ExitInterrupted)
	}

	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		if appErr.Code == domain.ExitUserAborted {
			if appErr.Message != "" {
				app.Logger.Log("%s", appErr.Message)
			}
			return int(appErr.Code)
		}
		app.Logger.Error("%s", appErr.Message)
		if appErr.Hint != "" {
			app.Logger.HintAlways("%s", appErr.Hint)
		}
		return int(appErr.Code)
	}

	app.Logger.Error("%v", err)
	return int(domain.ExitUsageError)
}
