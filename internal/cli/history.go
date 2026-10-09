package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bvarnai/git-brx/internal/checks"
	"github.com/bvarnai/git-brx/internal/domain"
	"github.com/spf13/cobra"
)

// HistoryOptions stores flags for the history command.
type HistoryOptions struct {
	MaxCount int
	Limit    int
	Branch   bool
	Search   string
	Grep     string
	Stat     bool
	Patch    bool
}

// newHistoryCmd constructs the 'history' subcommand.
func (a *App) newHistoryCmd() *cobra.Command {
	opts := HistoryOptions{}

	cmd := &cobra.Command{
		Use:   "history [flags] [<path>]",
		Short: "Display a formatted commit graph",
		Long: `Renders a formatted commit graph / DAG. Streams formatted commit history
to stdout using standard git log formatting.

Supports scoping to a specific file or directory path (following renames),
filtering to active topic branch commits (--branch / -b), searching commit
messages (--search / -s), and peeking file stats (--stat) or patches (-p).`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runHistory(cmd.Context(), opts, args)
		},
	}

	cmd.Flags().IntVarP(&opts.MaxCount, "max-count", "n", 0, "Limit the number of commits rendered in the graph")
	cmd.Flags().IntVarP(&opts.Limit, "limit", "l", 0, "Alias for --max-count")
	cmd.Flags().BoolVarP(&opts.Branch, "branch", "b", false, "Show only commits on active topic branch relative to base branch")
	cmd.Flags().StringVarP(&opts.Search, "search", "s", "", "Filter commits whose message matches the query string")
	cmd.Flags().StringVar(&opts.Grep, "grep", "", "Alias for --search")
	cmd.Flags().BoolVar(&opts.Stat, "stat", false, "Show diffstat summary of changed files for each commit")
	cmd.Flags().BoolVarP(&opts.Patch, "patch", "p", false, "Show code diff patch for each commit")

	return cmd
}

func (a *App) runHistory(ctx context.Context, opts HistoryOptions, args []string) error {
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

	if err := checks.HasCommits(ctx, a.Inspector, rootDir); err != nil {
		return err
	}

	effectiveCount := opts.MaxCount
	if opts.Limit > 0 && effectiveCount == 0 {
		effectiveCount = opts.Limit
	}

	logArgs := []string{
		"log",
		"--pretty=format:%h %ad | %s%d [%an]",
		"--graph",
		"--decorate",
		"--date=short",
	}

	if effectiveCount > 0 {
		logArgs = append(logArgs, fmt.Sprintf("--max-count=%d", effectiveCount))
	}

	searchQuery := opts.Search
	if searchQuery == "" && opts.Grep != "" {
		searchQuery = opts.Grep
	}
	if searchQuery != "" {
		logArgs = append(logArgs, fmt.Sprintf("--grep=%s", searchQuery), "-i")
	}

	if opts.Stat {
		logArgs = append(logArgs, "--stat")
	}

	if opts.Patch {
		logArgs = append(logArgs, "-p")
	}

	if !a.UI.Color {
		logArgs = append(logArgs, "--no-color")
	}

	pathArg := ""
	pathIsDir := false
	if len(args) == 1 && args[0] != "" {
		targetPath := args[0]
		targetFullPath := filepath.Join(workDir, targetPath)
		fi, statErr := os.Stat(targetFullPath)
		pathExists := statErr == nil

		if !pathExists {
			// Check if path exists in git history at HEAD
			relPath := targetPath
			if rel, err := filepath.Rel(rootDir, targetFullPath); err == nil && !strings.HasPrefix(rel, "..") {
				relPath = rel
			}
			_, catErr := a.Runner.Run(ctx, rootDir, "cat-file", "-e", fmt.Sprintf("HEAD:%s", relPath))
			if catErr == nil {
				pathExists = true
			}
		}

		if !pathExists {
			localExists, remoteExists, _ := a.Inspector.BranchExists(ctx, rootDir, targetPath)
			if localExists || remoteExists {
				return domain.NewError(domain.ExitUsageError, "'%s' is a branch name, not a file path. '-b' does not accept arguments", targetPath)
			}
			return domain.NewError(domain.ExitUsageError, "Path '%s' does not exist in repository", targetPath)
		}

		pathArg = targetPath
		pathIsDir = statErr == nil && fi.IsDir()
	}

	if opts.Branch {
		currBranch, branchErr := a.Inspector.CurrentBranch(ctx, rootDir)
		if branchErr == nil {
			baseBranch := a.Inspector.DefaultBranch(ctx, rootDir)
			if baseBranch == "" {
				baseBranch = "master"
			}

			if currBranch.IsDetached {
				logArgs = append(logArgs, fmt.Sprintf("%s..HEAD", baseBranch))
			} else if currBranch.Name != baseBranch {
				logArgs = append(logArgs, fmt.Sprintf("%s..%s", baseBranch, currBranch.Name))
			} else {
				a.UI.Log("Already on base branch '%s'", baseBranch)
				logArgs = append(logArgs, baseBranch)
			}
		}
	}

	if pathArg != "" {
		if pathIsDir {
			logArgs = append(logArgs, "--", pathArg)
		} else {
			logArgs = append(logArgs, "--follow", "--", pathArg)
		}
	}

	cw := &countingWriter{writer: a.UI.Stdout}
	err = a.Runner.RunStream(ctx, workDir, cw, a.UI.Stderr, nil, logArgs...)
	if err != nil {
		return domain.WrapError(domain.ExitGeneralError, err, "Getting history failed (git log failed)")
	}

	if cw.bytesWritten == 0 {
		a.UI.Log("No commits found matching the specified criteria")
	}

	return nil
}

type countingWriter struct {
	writer       io.Writer
	bytesWritten int64
}

func (c *countingWriter) Write(p []byte) (n int, err error) {
	n, err = c.writer.Write(p)
	c.bytesWritten += int64(n)
	return n, err
}
