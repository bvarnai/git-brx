package cli

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/bvarnai/git-brx/internal/checks"
	"github.com/bvarnai/git-brx/internal/config"
	"github.com/bvarnai/git-brx/internal/domain"
	"github.com/bvarnai/git-brx/internal/errcode"
	"github.com/bvarnai/git-brx/internal/github"
	"github.com/spf13/cobra"
)

// CreateOptions stores flags for the create command.
type CreateOptions struct {
	Base    string
	Offline bool
	Yes     bool
}

// newCreateCmd constructs the 'create' subcommand.
func (a *App) newCreateCmd() *cobra.Command {
	opts := CreateOptions{}

	cmd := &cobra.Command{
		Use:   "create [flags] <branch>",
		Short: "Create and checkout a new local development branch",
		Long: `Validate an issue against the issue tracker and provision a new local branch
following repository branching conventions.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runCreate(cmd.Context(), opts, args)
		},
	}

	cmd.Flags().StringVarP(&opts.Base, "base", "b", "", "Explicit base branch to fork from and record as parent base")
	cmd.Flags().BoolVarP(&opts.Offline, "offline", "o", false, "Skip issue validation and remote Git availability checks")
	cmd.Flags().BoolVarP(&opts.Yes, "yes", "y", false, "Automatically accept confirmation prompts without interactive input")

	_ = cmd.RegisterFlagCompletionFunc("base", a.completeBranchNames)

	return cmd
}

func (a *App) runCreate(ctx context.Context, opts CreateOptions, args []string) error {
	// 1. Argument & Repository Assertions
	if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
		return errcode.NewError(errcode.ErrUsage, "No branch name specified")
	}
	if len(args) > 1 {
		return errcode.NewError(errcode.ErrUsage, "Unexpected argument: %s", args[1])
	}
	targetBranch := strings.TrimSpace(args[0])

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

	a.Logger.Debug("Repository root: %s", rootDir)
	a.Logger.Debug("Target branch to create: %s (offline=%v)", targetBranch, opts.Offline)

	// 2. Local & Remote Branch Collision Auto-Switch
	localExists, _, _ := a.Inspector.BranchExists(ctx, rootDir, targetBranch)
	hasOrigin, originURL, _ := a.Inspector.HasOrigin(ctx, rootDir)
	remoteExists := false
	if !localExists && !opts.Offline && hasOrigin {
		a.Logger.Log("Checking for existing branches (remote)")
		remoteExists, err = a.Inspector.RemoteBranchExistsOnServer(ctx, rootDir, targetBranch)
		if err != nil {
			return errcode.NewError(errcode.ErrPreconditionRemote, "Unable to reach remote").
				WithHint("Try --offline if working without network access")
		}
		if !remoteExists {
			a.Logger.Log("No branch '%s' found (remote)", targetBranch)
		}
	} else if !localExists && opts.Offline {
		a.Logger.Log("Offline mode selected (git remote checks skipped...)")
	}

	if localExists || remoteExists {
		return a.switchToExistingBranch(ctx, rootDir, targetBranch, localExists, hasOrigin, opts.Offline)
	}

	// 4. Configuration & Template Matching
	cfg, err := config.Load(rootDir, originURL)
	if err != nil {
		var appErr *errcode.AppError
		if errors.As(err, &appErr) {
			return appErr
		}
		return errcode.WrapError(errcode.ErrConfig, err, "Configuration error")
	}

	templatePattern := cfg.Branch.Template
	re, err := regexp.Compile(templatePattern)
	if err != nil {
		return errcode.NewError(errcode.ErrConfig, "Invalid branch template regular expression: %s", templatePattern)
	}

	matches := re.FindStringSubmatch(targetBranch)
	if matches == nil {
		return errcode.NewError(errcode.ErrConfig, "Branch name '%s' doesn't match pattern %s", targetBranch, templatePattern)
	}

	branchPath, issueKey := extractBranchPathAndKey(matches, targetBranch)

	// 5. Issue Tracker Validation (Online Mode)
	if !opts.Offline {
		tracker, err := a.resolveTracker(cfg)
		if err != nil {
			return err
		}

		if tracker != nil {
			issue, err := tracker.GetIssue(ctx, issueKey)
			if err != nil {
				return err
			}

			// Validate issue type and direct label mapping
			if len(cfg.Branch.Mapping) > 0 {
				var allowedTargets []string
				var matchedKey string
				matched := false

				// 1. Direct label/component match (takes precedence over issue.Type)
				for _, comp := range issue.Components {
					compLower := strings.ToLower(comp)
					for mapKey, rawVal := range cfg.Branch.Mapping {
						if strings.ToLower(mapKey) == compLower {
							targets := config.ParseBranchTargets(rawVal)
							if containsTarget(targets, branchPath) {
								allowedTargets = targets
								matchedKey = comp
								matched = true
								break
							} else if len(allowedTargets) == 0 {
								allowedTargets = targets
								matchedKey = comp
							}
						}
					}
					if matched {
						break
					}
				}

				// 2. Fall back to domain issue.Type match if no direct label matched the branchPath
				if !matched {
					for mapKey, rawVal := range cfg.Branch.Mapping {
						if strings.ToLower(mapKey) == strings.ToLower(issue.Type) {
							targets := config.ParseBranchTargets(rawVal)
							if containsTarget(targets, branchPath) {
								allowedTargets = targets
								matchedKey = issue.Type
								matched = true
								break
							} else if len(allowedTargets) == 0 {
								allowedTargets = targets
								matchedKey = issue.Type
							}
						}
					}
				}

				if !matched && len(allowedTargets) > 0 {
					return errcode.NewError(errcode.ErrConfig, "Issue type '%s' is not allowed on '%s' branch", matchedKey, branchPath).
						WithHint("Use '%s' branch or check mapping rules", strings.Join(allowedTargets, "' or '"))
				} else if !matched && branchPath != "issue" {
					// Unmapped custom issue type is allowed on generic 'issue/' branch, but restricted from specialized branches
					return errcode.NewError(errcode.ErrConfig, "Issue type '%s' has no mapping defined for '%s' branch", issue.Type, branchPath).
						WithHint("Use 'issue/' branch or configure branch.mapping in .git-brx.yaml")
				}
			}

			assignee := issue.Assignee
			if assignee == "" {
				assignee = "nobody (not yet assigned)"
			}

			// Display rich issue details with clean word-wrapping
			if issue.Title != "" {
				prefix := fmt.Sprintf("Issue #%s: ", issue.Key)
				indent := strings.Repeat(" ", len(prefix))
				for _, line := range a.Logger.WrapText(prefix, indent, issue.Title, 0) {
					a.Logger.Log("%s", line)
				}
			} else {
				a.Logger.Log("Issue #%s", issue.Key)
			}
			a.Logger.Log("  Type:     %s", issue.Type)
			a.Logger.Log("  Status:   %s", issue.Status)
			a.Logger.Log("  Assignee: %s", assignee)
			if len(issue.Components) > 0 {
				prefix := "  Labels:   "
				indent := "            "
				labelsStr := strings.Join(issue.Components, ", ")
				for _, line := range a.Logger.WrapText(prefix, indent, labelsStr, 0) {
					a.Logger.Log("%s", line)
				}
			}
			if issue.URL != "" {
				a.Logger.Log("  URL:      %s", a.Logger.Hyperlink(issue.URL, issue.URL))
			}

			// Warn if the issue is closed or resolved
			st := strings.ToLower(strings.TrimSpace(issue.Status))
			if st == "closed" || st == "resolved" || st == "done" || st == "completed" {
				a.Logger.Warn("Issue #%s is currently marked as '%s'", issue.Key, issue.Status)
			}

			// Confirmation prompt unless --yes
			if !opts.Yes {
				prompt := fmt.Sprintf("Create branch '%s' [Y/n]?", targetBranch)
				confirmed, err := a.Logger.Confirm(prompt)
				if err != nil || !confirmed {
					return errcode.NewError(errcode.ErrUserAborted, "Aborted")
				}
			}
		}
	} else {
		a.Logger.Log("Offline mode selected (issue tracker checks skipped...)")
	}

	// 5.5. Base Branch Validation & Guardrails
	resolvedBase, startPoint, err := a.prepareCreateBase(ctx, rootDir, targetBranch, opts.Base, hasOrigin, opts.Offline)
	if err != nil {
		return err
	}

	// 6. Branch Provisioning
	if dirty, dirtyErr := a.Inspector.IsDirty(ctx, rootDir); dirtyErr == nil && dirty {
		a.Logger.Warn("Uncommitted local changes will be carried over to '%s'", targetBranch)
	}

	a.Logger.Log("Creating branch '%s'", targetBranch)
	if err := a.Operations.CreateAndCheckoutFrom(ctx, rootDir, targetBranch, startPoint); err != nil {
		return errcode.WrapError(errcode.ErrGeneral, err, "Failed to create branch '%s'", targetBranch)
	}

	if resolvedBase != "" && resolvedBase != targetBranch {
		if baseErr := a.Operations.SetBranchBase(ctx, rootDir, targetBranch, resolvedBase); baseErr != nil {
			a.Logger.Debug("Failed to record parent base branch '%s' for '%s': %v", resolvedBase, targetBranch, baseErr)
		}
	}

	a.Logger.Hint("Use 'git-brx publish' to publish your new branch")
	return nil
}

func (a *App) switchToExistingBranch(ctx context.Context, rootDir, targetBranch string, localExists, hasOrigin, offline bool) error {
	if err := checks.NoOrphanedCommits(ctx, a.Inspector, rootDir); err != nil {
		return err
	}

	wasDirty, _ := a.Inspector.IsDirty(ctx, rootDir)
	prevBranch := ""
	if curr, err := a.Inspector.CurrentBranch(ctx, rootDir); err == nil && !curr.IsDetached {
		prevBranch = curr.Name
	}

	fetchOK := false
	if prevBranch != targetBranch {
		if !localExists && hasOrigin && !offline {
			if fetchErr := a.Operations.Fetch(ctx, rootDir, "origin"); fetchErr != nil {
				return errcode.NewError(errcode.ErrPreconditionRemote, "Unable to reach remote").
					WithHint("Try --offline if working without network access")
			}
			fetchOK = true
		}

		if checkoutErr := a.Operations.Checkout(ctx, rootDir, targetBranch); checkoutErr != nil {
			if strings.Contains(checkoutErr.Error(), "would be overwritten by checkout") {
				return errcode.NewError(errcode.ErrConflict, "Local modifications would be overwritten by checkout").
					WithHint("Commit or stash your changes before switching branches")
			}
			return errcode.WrapError(errcode.ErrGeneral, checkoutErr, "Failed to switch to existing branch '%s'", targetBranch)
		}

		a.warnDirtyWorktree(wasDirty, prevBranch, targetBranch)
	}

	if !offline && hasOrigin {
		if !fetchOK && localExists {
			fetchOK = a.Operations.Fetch(ctx, rootDir, "origin") == nil
		}
		if fetchOK {
			a.syncOrHintUpstream(ctx, rootDir, targetBranch, false, false)
		}
	}

	a.Logger.Log("Branch '%s' already exists; switched to '%s'", targetBranch, targetBranch)
	return nil
}

func (a *App) prepareCreateBase(ctx context.Context, rootDir, targetBranch, explicitBase string, hasOrigin, offline bool) (resolvedBase string, startPoint string, err error) {
	baseName := strings.TrimSpace(explicitBase)
	if baseName != "" {
		var fetchErr error
		if !offline && hasOrigin {
			fetchErr = a.Operations.Fetch(ctx, rootDir, "origin")
		}

		baseLocal, baseRemote, _ := a.Inspector.BranchExists(ctx, rootDir, baseName)
		if !baseLocal && !baseRemote {
			if fetchErr != nil {
				return "", "", errcode.NewError(errcode.ErrPreconditionRemote, "Unable to reach remote").
					WithHint("Try --offline if working without network access")
			}
			return "", "", errcode.NewError(errcode.ErrPreconditionRepo, "Base branch '%s' not found", baseName)
		}

		if !baseLocal {
			return baseName, "origin/" + baseName, nil
		}

		startPoint = baseName
		if !offline && hasOrigin && baseRemote {
			delta, deltaErr := a.Inspector.ComputeDelta(ctx, rootDir, baseName, "origin/"+baseName)
			if deltaErr == nil && delta != nil && delta.BehindCount > 0 {
				curr, _ := a.Inspector.CurrentBranch(ctx, rootDir)
				checkedOutHere := curr != nil && !curr.IsDetached && curr.Name == baseName
				checkedOutElsewhere := a.Inspector.IsBranchCheckedOutInOtherWorktree(ctx, rootDir, baseName)

				if delta.AheadCount == 0 && !checkedOutHere && !checkedOutElsewhere {
					if ffErr := a.Operations.FetchRef(ctx, rootDir, "origin", baseName+":"+baseName); ffErr != nil {
						startPoint = "origin/" + baseName
					}
				} else if delta.AheadCount == 0 && checkedOutElsewhere {
					startPoint = "origin/" + baseName
				} else {
					a.Logger.Warn("Base branch '%s' is behind 'origin/%s' by %d commit(s)", baseName, baseName, delta.BehindCount)
					a.Logger.Hint("Run 'git-brx update' before branching if you need the latest commits")
				}
			}
		}
		return baseName, startPoint, nil
	}

	curr, currErr := a.Inspector.CurrentBranch(ctx, rootDir)
	if currErr == nil && !curr.IsDetached {
		resolvedBase = curr.Name
		if curr.Type == domain.BranchTypeIssue || curr.Type == domain.BranchTypeFeature {
			def := a.Inspector.DefaultBranch(ctx, rootDir)
			a.Logger.Warn("Creating '%s' from '%s' instead of default branch", targetBranch, curr.Name)
			a.Logger.Hint("Pass '--base %s' or run 'git-brx select %s' first if this was unintentional", def, def)
		}
		if !offline && hasOrigin {
			if _, remoteExists, _ := a.Inspector.BranchExists(ctx, rootDir, curr.Name); remoteExists {
				if delta, deltaErr := a.Inspector.ComputeDelta(ctx, rootDir, curr.Name, "origin/"+curr.Name); deltaErr == nil && delta != nil && delta.BehindCount > 0 {
					a.Logger.Warn("Base branch '%s' is behind 'origin/%s' by %d commit(s)", curr.Name, curr.Name, delta.BehindCount)
					a.Logger.Hint("Run 'git-brx update' before branching if you need the latest commits")
				}
			}
		}
	}
	return resolvedBase, "", nil
}

func extractBranchPathAndKey(matches []string, targetBranch string) (string, string) {
	if len(matches) >= 3 {
		return matches[1], matches[2]
	}
	parts := strings.SplitN(targetBranch, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "issue", targetBranch
}

func (a *App) resolveTracker(cfg *domain.ProjectConfig) (domain.IssueTracker, error) {
	provider := strings.ToLower(cfg.Tracker.Provider)
	switch provider {
	case "github":
		token := github.ResolveToken(context.Background(), a.Runner, "github.com")
		opts := []github.Option{}
		if cfg.Tracker.URI != "" && cfg.Tracker.URI != "https://github.com" && cfg.Tracker.URI != "http://github.com" {
			opts = append(opts, github.WithBaseURL(cfg.Tracker.URI))
		}
		return github.NewClient(cfg.Tracker.Owner, cfg.Tracker.Repo, token, opts...), nil
	case "", "none":
		return nil, nil
	default:
		return nil, errcode.NewError(errcode.ErrConfig, "Unsupported issue tracker provider: %s", provider)
	}
}

func containsTarget(targets []string, branchPath string) bool {
	for _, t := range targets {
		if strings.EqualFold(t, branchPath) {
			return true
		}
	}
	return false
}
