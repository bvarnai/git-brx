package cli

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/bvarnai/git-brx/internal/checks"
	"github.com/bvarnai/git-brx/internal/config"
	"github.com/bvarnai/git-brx/internal/domain"
	"github.com/spf13/cobra"
)

// ReviewOptions stores flags for the review command.
type ReviewOptions struct {
	Reviewer string
	Title    string
}

// newReviewCmd constructs the 'review' subcommand.
func (a *App) newReviewCmd() *cobra.Command {
	opts := ReviewOptions{}

	cmd := &cobra.Command{
		Use:   "review [flags] [<target_branch>]",
		Short: "Create a code review / pull request on the configured SCM platform",
		Long: `Assembles issue metadata, resolves designated reviewers, formats a
standardized pull request description with merge instructions, and creates
the pull request on the configured SCM platform (e.g. GitHub or Bitbucket).`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runReview(cmd.Context(), opts, args)
		},
		ValidArgsFunction: a.completeBranchNames,
	}

	cmd.Flags().StringVarP(&opts.Reviewer, "reviewer", "r", "", "Designate an explicit reviewer instead of component mapping")
	cmd.Flags().StringVarP(&opts.Title, "title", "t", "", "Override default pull request title")

	return cmd
}

func (a *App) runReview(ctx context.Context, opts ReviewOptions, args []string) error {
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

	hasOrigin, originURL, err := a.Inspector.HasOrigin(ctx, rootDir)
	if err != nil || !hasOrigin {
		return domain.NewError(domain.ExitPreconditionRepo, "Origin remote is not configured")
	}

	currBranch, err := a.Inspector.CurrentBranch(ctx, rootDir)
	if err != nil || currBranch.IsDetached {
		return domain.NewError(domain.ExitPreconditionRepo, "Cannot create review in detached HEAD state")
	}

	switch currBranch.Type {
	case domain.BranchTypeIssue, domain.BranchTypeFeature, domain.BranchTypeEpic:
		// Eligible topic branch
	default:
		return domain.NewError(domain.ExitPreconditionRepo, "You must be on an 'issue', 'feature', or 'epic' branch to create a review")
	}

	// 2. Publication Precondition Check
	existsOnServer, err := a.Inspector.RemoteBranchExistsOnServer(ctx, rootDir, currBranch.Name)
	if err != nil {
		return domain.WrapError(domain.ExitPreconditionRemote, err, "Unable to reach remote; are you offline?")
	}
	if !existsOnServer {
		a.UI.Error("Branch '%s' has not been pushed to remote origin", currBranch.Name)
		return domain.NewError(domain.ExitPreconditionRemote, "Branch '%s' has not been pushed to remote origin", currBranch.Name).
			WithHint("Run 'git-brx publish' to push your branch before creating a review")
	}

	// 3. Resolve Target Branch
	targetBranch := ""
	if len(args) == 1 && strings.TrimSpace(args[0]) != "" {
		targetBranch = args[0]
	} else {
		targetBranch = a.Inspector.DefaultBranch(ctx, rootDir)
		if targetBranch == "" {
			targetBranch = "master"
		}
		a.UI.Log("Selecting '%s' target branch by default", targetBranch)
	}

	// 4. Configuration & Issue Loading
	cfg, err := config.Load(rootDir, originURL)
	if err != nil {
		return err
	}

	scm, err := a.resolveSCM(cfg)
	if err != nil {
		return err
	}
	if scm == nil {
		return domain.NewError(domain.ExitConfigError, "No SCM provider configured for pull request creation")
	}

	templatePattern := cfg.Branch.Template
	if templatePattern == "" {
		templatePattern = `^(issue|feature|epic)/([A-Za-z0-9_-]+)$`
	}
	re, _ := regexp.Compile(templatePattern)
	var issueKey string
	if re != nil {
		matches := re.FindStringSubmatch(currBranch.Name)
		if len(matches) >= 3 {
			issueKey = matches[2]
		}
	}
	if issueKey == "" {
		parts := strings.SplitN(currBranch.Name, "/", 2)
		if len(parts) == 2 {
			issueKey = parts[1]
		}
	}

	var issue *domain.Issue
	tracker, _ := a.resolveTracker(cfg)
	if tracker != nil && issueKey != "" && !a.Opts.DryRun {
		fetchedIssue, issueErr := tracker.GetIssue(ctx, issueKey)
		if issueErr != nil {
			return issueErr
		}
		issue = fetchedIssue
	}

	// 5. Reviewer Resolution
	var reviewers []string
	if opts.Reviewer != "" {
		reviewers = []string{opts.Reviewer}
	} else {
		var components []string
		if issue != nil {
			components = issue.Components
		}
		reviewers = scm.ResolveReviewers(components)
	}

	// 6. Title & Description Composition
	prTitle := opts.Title
	if prTitle == "" {
		if issue != nil && issue.Title != "" {
			prTitle = fmt.Sprintf("%s: %s", currBranch.Name, issue.Title)
		} else {
			prTitle = currBranch.Name
		}
	}

	summaryText := ""
	if issue != nil {
		summaryText = issue.Title
	}
	prBody := a.composeReviewDescription(cfg, scm, issueKey, summaryText, currBranch.Name, targetBranch)

	// 7. Dry-Run Check
	if a.Opts.DryRun {
		a.UI.Log("Would create pull request on %s", scm.Name())
		a.UI.Log("  Source:    %s", currBranch.Name)
		a.UI.Log("  Target:    %s", targetBranch)
		a.UI.Log("  Title:     %s", prTitle)
		if len(reviewers) > 0 {
			a.UI.Log("  Reviewers: %s", strings.Join(reviewers, ", "))
		}
		return nil
	}

	// 8. Pull Request Submission
	req := domain.PullRequestRequest{
		Title:        prTitle,
		Description:  prBody,
		SourceBranch: currBranch.Name,
		TargetBranch: targetBranch,
		Reviewers:    reviewers,
	}

	result, err := scm.CreatePullRequest(ctx, req)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "409") {
			return domain.NewError(domain.ExitAPIError, "A pull request for branch '%s' already exists", currBranch.Name)
		}
		return err
	}

	a.UI.Log("Created pull-request %s", result.URL)
	a.UI.Out("%s", result.URL)

	return nil
}

func (a *App) composeReviewDescription(cfg *domain.ProjectConfig, scm domain.SCMProvider, issueKey, summary, branch, targetBranch string) string {
	template := ""
	if cfg.Review.Template != "" {
		if content, err := os.ReadFile(cfg.Review.Template); err == nil {
			template = string(content)
		} else {
			template = cfg.Review.Template
		}
	}

	var sb strings.Builder
	if template != "" {
		sb.WriteString(template)
		sb.WriteString("\n\n")
	} else {
		sb.WriteString("### Description\n")
		if summary != "" {
			sb.WriteString(fmt.Sprintf("%s\n\n", summary))
		}
		sb.WriteString("### Verification & Testing\n- [ ] Automated tests pass\n- [ ] Manual verification completed\n\n")
	}

	// Append merge instructions unless explicitly disabled via review.instructions: false
	if cfg.Review.Instructions == nil || *cfg.Review.Instructions {
		opts := domain.MergeInstructionOptions{
			IssueKey:     issueKey,
			Summary:      summary,
			Branch:       branch,
			TargetBranch: targetBranch,
		}
		if scm != nil {
			sb.WriteString(scm.FormatMergeInstructions(opts))
		} else {
			sb.WriteString(domain.FormatDefaultMergeInstructions(opts))
		}
	}

	return sb.String()
}
