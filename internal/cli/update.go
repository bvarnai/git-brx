package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/bvarnai/git-brx/internal/checks"
	"github.com/bvarnai/git-brx/internal/domain"
	"github.com/bvarnai/git-brx/internal/git"
	"github.com/spf13/cobra"
)

// UpdateOptions stores flags for the update command.
type UpdateOptions struct {
	Autostash bool
}

// newUpdateCmd constructs the 'update' subcommand.
func (a *App) newUpdateCmd() *cobra.Command {
	opts := UpdateOptions{}

	cmd := &cobra.Command{
		Use:   "update [flags]",
		Short: "Pulls and rebases remote changes for the current branch from origin",
		Long: `Pulls remote changes using rebase onto the active local branch.
Ensures current branch exists on origin and is not in an in-flight conflict.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runUpdate(cmd.Context(), opts, args)
		},
	}

	cmd.Flags().BoolVarP(&opts.Autostash, "autostash", "a", false, "Automatically stash unstaged/staged changes before rebasing and pop stash afterwards")
	cmd.Flags().BoolVarP(&a.Opts.DryRun, "dry-run", "n", false, "Simulate execution without modifying Git state or remote services")

	return cmd
}

func (a *App) runUpdate(ctx context.Context, opts UpdateOptions, args []string) error {
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

	if err := checks.NoActiveOperation(ctx, a.Inspector, rootDir); err != nil {
		return err
	}

	currBranch, err := checks.AttachedBranch(ctx, a.Inspector, rootDir)
	if err != nil {
		return err
	}

	if err := checks.HasOrigin(ctx, a.Inspector, rootDir); err != nil {
		return err
	}

	// Verify current branch exists on origin
	_, remoteExists, _ := a.Inspector.BranchExists(ctx, rootDir, currBranch.Name)
	if !remoteExists {
		return domain.NewError(domain.ExitPreconditionRemote, "Branch '%s' has not been published to origin yet", currBranch.Name).
			WithHint("Use 'git-brx publish' to publish your branch first")
	}

	remoteRef := "origin/" + currBranch.Name

	// Dry run mode
	if a.Opts.DryRun {
		cmdStr := fmt.Sprintf("git rebase %s", remoteRef)
		if opts.Autostash {
			cmdStr = fmt.Sprintf("git rebase --autostash %s", remoteRef)
		}
		a.UI.Log("Would fetch 'origin/%s' and execute: %s", currBranch.Name, cmdStr)
		return nil
	}

	// Fetch remote reference
	if fetchErr := a.Operations.FetchRef(ctx, rootDir, "origin", currBranch.Name); fetchErr != nil {
		return domain.NewError(domain.ExitPreconditionRemote, "Update failed (git fetch origin %s failed)", currBranch.Name)
	}

	// Rebase onto remote tracking ref
	if rebaseErr := a.Operations.Rebase(ctx, rootDir, remoteRef, opts.Autostash, false); rebaseErr != nil {
		if strings.Contains(rebaseErr.Error(), "conflict") || isRebaseConflict(ctx, a.Inspector, rootDir) {
			return domain.NewError(domain.ExitConflict, "Update failed due to conflicts").
				WithHint("Use 'git-brx resolve' to resolve conflicts or 'git-brx reset' to abort")
		}
		return domain.NewError(domain.ExitConflict, "Update failed due to conflicts").
			WithHint("Use 'git-brx resolve' to resolve conflicts or 'git-brx reset' to abort")
	}

	a.UI.Log("Current branch '%s' is up to date", currBranch.Name)
	return nil
}

func isRebaseConflict(ctx context.Context, inspector *git.Inspector, rootDir string) bool {
	op, active := inspector.ActiveOperation(ctx, rootDir)
	return active && op == "rebase"
}
