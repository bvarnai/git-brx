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
	Offline bool
	Yes     bool
}

// newCreateCmd constructs the 'create' subcommand.
func (a *App) newCreateCmd() *cobra.Command {
	opts := CreateOptions{}

	cmd := &cobra.Command{
		Use:   "create [flags] <branch>",
		Short: "Create and checkout a new local development branch",
		Long: `Validate an issue against the issue tracker and provision a new topic branch
following repository branching conventions.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runCreate(cmd.Context(), opts, args)
		},
	}

	cmd.Flags().BoolVarP(&opts.Offline, "offline", "o", false, "Skip issue validation and remote Git availability checks")
	cmd.Flags().BoolVarP(&opts.Yes, "yes", "y", false, "Automatically accept confirmation prompts without interactive input")

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

	// 2. Local Branch Collision Check
	localExists, _, _ := a.Inspector.BranchExists(ctx, rootDir, targetBranch)
	if localExists {
		a.Logger.Log("Branch '%s' found (local)", targetBranch)
		a.Logger.Hint("To select this branch, use 'git-brx select %s' instead", targetBranch)
		return nil
	}

	// 3. Remote Branch Collision Check (Online Mode)
	hasOrigin, originURL, _ := a.Inspector.HasOrigin(ctx, rootDir)
	if !opts.Offline && hasOrigin {
		a.Logger.Log("Checking for existing branches (remote)")
		remoteExists, err := a.Inspector.RemoteBranchExistsOnServer(ctx, rootDir, targetBranch)
		if err != nil {
			return errcode.NewError(errcode.ErrPreconditionRemote, "Unable to reach remote").
				WithHint("Try --offline if working without network access")
		}
		if remoteExists {
			a.Logger.Log("Branch '%s' found (remote)", targetBranch)
			a.Logger.Hint("To select this branch, use 'git-brx select %s' instead", targetBranch)
			return nil
		}
		a.Logger.Log("No branch '%s' found (remote)", targetBranch)
	} else if opts.Offline {
		a.Logger.Log("Offline mode selected (git remote checks skipped...)")
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

	// 6. Branch Provisioning
	a.Logger.Log("Creating branch '%s'", targetBranch)
	if err := a.Operations.CreateAndCheckout(ctx, rootDir, targetBranch); err != nil {
		return errcode.WrapError(errcode.ErrGeneral, err, "Failed to create branch '%s'", targetBranch)
	}

	a.Logger.Hint("Use 'git-brx publish' to publish your new branch")
	return nil
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
