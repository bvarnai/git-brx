package cli

import (
	"context"
	"strings"

	"github.com/bvarnai/git-brx/internal/domain"
	"github.com/spf13/cobra"
)

// SelectOptions stores flags for the select command.
type SelectOptions struct {
	Offline bool
	NoFetch bool
}

// newSelectCmd constructs the 'select' subcommand.
func (a *App) newSelectCmd() *cobra.Command {
	opts := SelectOptions{}

	cmd := &cobra.Command{
		Use:   "select [flags] [<branch>]",
		Short: "Select and switch to an existing branch ('master' by default)",
		Long: `Select and switch to an existing branch. In online mode, fetches updates
from origin before switching.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runSelect(cmd.Context(), opts, args)
		},
	}

	cmd.Flags().BoolVarP(&opts.Offline, "offline", "o", false, "Skip remote fetch and switch between existing local references only")
	cmd.Flags().BoolVar(&opts.NoFetch, "no-fetch", false, "Do not trigger remote synchronization before checkout")

	return cmd
}

func (a *App) runSelect(ctx context.Context, opts SelectOptions, args []string) error {
	if len(args) > 1 {
		return domain.NewError(domain.ExitUsageError, "Unexpected argument: %s", args[1])
	}

	workDir := "."
	if a.WorkingDir != "" {
		workDir = a.WorkingDir
	}

	rootDir, err := a.Inspector.AssertWorkTree(ctx, workDir)
	if err != nil {
		return err
	}

	rebaseActive, mergeActive := a.Inspector.InFlightOperations(ctx, rootDir)
	if rebaseActive || mergeActive {
		return domain.NewError(domain.ExitConflict, "Cannot switch branches during an active merge/rebase")
	}

	targetBranch := "master"
	if len(args) == 1 && args[0] != "" {
		targetBranch = args[0]
	} else {
		a.UI.Log("Selecting 'master' branch by default")
	}

	// Remote synchronization (online mode)
	if !opts.Offline && !opts.NoFetch {
		hasOrigin, _, _ := a.Inspector.HasOrigin(ctx, rootDir)
		if hasOrigin {
			fetchErr := a.Operations.Fetch(ctx, rootDir, "origin")
			if fetchErr != nil {
				localExists, _, _ := a.Inspector.BranchExists(ctx, rootDir, targetBranch)
				if localExists {
					a.UI.Warn("Unable to reach remote; falling back to local references")
				} else {
					return domain.NewError(domain.ExitPreconditionRemote, "Select failed (git fetch failed)")
				}
			}
		}
	}

	localExists, remoteExists, _ := a.Inspector.BranchExists(ctx, rootDir, targetBranch)

	// Dry run mode
	if a.Opts.DryRun {
		if !localExists && !remoteExists {
			return domain.NewError(domain.ExitPreconditionRepo, "Branch '%s' not found", targetBranch)
		}
		a.UI.Log("Would checkout branch '%s'", targetBranch)
		return nil
	}

	if !localExists && !remoteExists {
		return domain.NewError(domain.ExitPreconditionRepo, "Branch '%s' not found", targetBranch)
	}

	checkoutErr := a.Operations.Checkout(ctx, rootDir, targetBranch)
	if checkoutErr != nil {
		if strings.Contains(checkoutErr.Error(), "would be overwritten by checkout") {
			return domain.NewError(domain.ExitConflict, "Select failed: local modifications would be overwritten by checkout").
				WithHint("Commit or stash your changes before switching branches")
		}
		return domain.WrapError(domain.ExitGeneralError, checkoutErr, "Select failed (git checkout %s failed)", targetBranch)
	}

	return nil
}
