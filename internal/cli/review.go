package cli

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/bvarnai/git-brx/internal/checks"
	"github.com/bvarnai/git-brx/internal/config"
	"github.com/bvarnai/git-brx/internal/domain"
	"github.com/bvarnai/git-brx/internal/errcode"
	"github.com/bvarnai/git-brx/internal/suggest"
	"github.com/spf13/cobra"
)

// ReviewOptions stores flags for the review command.
type ReviewOptions struct {
	Reviewers           []string
	Title               string
	Draft               bool
	MergeInstructions   bool
	NoMergeInstructions bool
	Strategy            string
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
			if cmd.Flags().Changed("merge-instructions") && cmd.Flags().Changed("no-merge-instructions") {
				return errcode.NewError(errcode.ErrUsage, "Cannot specify both --merge-instructions and --no-merge-instructions")
			}
			if opts.Strategy != "" {
				s := strings.ToLower(strings.TrimSpace(opts.Strategy))
				if s != "squash" && s != "merge" && s != "rebase" {
					return errcode.NewError(errcode.ErrUsage, "Unsupported merge strategy: %s (must be squash, merge, or rebase)", opts.Strategy)
				}
			}
			var mergeInstrOverride *bool
			if cmd.Flags().Changed("merge-instructions") {
				b := opts.MergeInstructions
				mergeInstrOverride = &b
			} else if cmd.Flags().Changed("no-merge-instructions") {
				b := !opts.NoMergeInstructions
				mergeInstrOverride = &b
			}
			return a.runReview(cmd.Context(), opts, mergeInstrOverride, args)
		},
		ValidArgsFunction: a.completeBranchNames,
	}

	cmd.Flags().StringArrayVarP(&opts.Reviewers, "reviewer", "r", nil, "Designate explicit reviewer(s) instead of component mapping (repeatable or comma-separated)")
	cmd.Flags().StringVarP(&opts.Title, "title", "t", "", "Override default pull request title")
	cmd.Flags().BoolVar(&opts.Draft, "draft", false, "Create pull request in draft state")
	cmd.Flags().BoolVar(&opts.MergeInstructions, "merge-instructions", false, "Append merge instructions to the pull request description")
	cmd.Flags().BoolVar(&opts.NoMergeInstructions, "no-merge-instructions", false, "Do not append merge instructions to the pull request description")
	cmd.Flags().StringVarP(&opts.Strategy, "strategy", "s", "", "Merge strategy override (squash, merge, rebase)")

	return cmd
}

func (a *App) runReview(ctx context.Context, opts ReviewOptions, mergeInstrOverride *bool, args []string) error {
	if len(args) > 1 {
		return errcode.NewError(errcode.ErrUsage, "Unexpected argument: %s", args[1])
	}

	workDir := "."
	if a.WorkingDir != "" {
		workDir = a.WorkingDir
	}

	rootDir, err := checks.InsideWorkTree(ctx, a.Inspector, workDir)
	if err != nil {
		return err
	}
	a.Logger.Debug("Repository root: %s", rootDir)

	if err := checks.HasCommits(ctx, a.Inspector, rootDir); err != nil {
		return err
	}

	hasOrigin, originURL, err := a.Inspector.HasOrigin(ctx, rootDir)
	if err != nil || !hasOrigin {
		return errcode.NewError(errcode.ErrPreconditionRepo, "Origin remote is not configured")
	}

	currBranch, err := a.Inspector.CurrentBranch(ctx, rootDir)
	if err != nil || currBranch.IsDetached {
		return errcode.NewError(errcode.ErrPreconditionRepo, "Cannot create review in detached HEAD state")
	}

	switch currBranch.Type {
	case domain.BranchTypeIssue, domain.BranchTypeFeature, domain.BranchTypeEpic:
		// Eligible topic branch
	default:
		return errcode.NewError(errcode.ErrPreconditionRepo, "You must be on an 'issue', 'feature', or 'epic' branch to create a review")
	}

	// 2. Publication & Up-to-Date Precondition Check
	remoteSHA, err := a.Inspector.RemoteBranchSHA(ctx, rootDir, currBranch.Name)
	if err != nil {
		return errcode.WrapError(errcode.ErrPreconditionRemote, err, "Unable to reach remote; are you offline?")
	}
	if remoteSHA == "" {
		return errcode.NewError(errcode.ErrPreconditionRemote, "Branch '%s' has not been pushed to remote origin", currBranch.Name).
			WithHint("Run 'git-brx publish' to push your branch before creating a review")
	}
	headSHA, headErr := a.Inspector.HeadSHA(ctx, rootDir)
	if headErr == nil && headSHA != "" && remoteSHA != headSHA {
		return errcode.NewError(errcode.ErrPreconditionRemote, "Branch '%s' has unpushed local commits (out of sync with origin)", currBranch.Name).
			WithHint("Run 'git-brx publish' to push your latest commits before creating a review")
	}

	// 3. Resolve Target Branch & Validate Remote Existence
	explicitTarget := ""
	if len(args) == 1 {
		explicitTarget = strings.TrimSpace(args[0])
	}
	base := a.resolveBaseBranch(ctx, rootDir, currBranch.Name, explicitTarget)
	targetBranch := base.Name
	if base.FromConfig {
		a.Logger.Log("Selecting recorded base '%s' as target branch", targetBranch)
	} else if base.FromDefault {
		a.Logger.Log("Selecting '%s' target branch by default", targetBranch)
	}
	a.Logger.Debug("Review branches: source=%s, target=%s", currBranch.Name, targetBranch)

	targetSHA, err := a.Inspector.RemoteBranchSHA(ctx, rootDir, targetBranch)
	if err != nil {
		return errcode.WrapError(errcode.ErrPreconditionRemote, err, "Unable to reach remote; are you offline?")
	}
	if targetSHA == "" {
		missingErr := errcode.NewError(errcode.ErrPreconditionRemote, "Target branch '%s' does not exist on remote origin", targetBranch)
		if candidates, listErr := a.Inspector.ListAllBranchNames(ctx, rootDir); listErr == nil {
			if closest := suggest.Closest(targetBranch, candidates, 3); closest != "" {
				missingErr.WithHint("Did you mean '%s'?", closest)
			}
		}
		return missingErr
	}

	// 4. Configuration & Issue Loading
	cfg, err := config.Load(rootDir, originURL)
	if err != nil {
		return err
	}

	if mergeInstrOverride != nil {
		cfg.Review.Merge.Instructions = mergeInstrOverride
	}

	scm, err := a.resolveSCM(cfg)
	if err != nil {
		return err
	}
	if scm == nil {
		return errcode.NewError(errcode.ErrConfig, "No SCM provider configured for pull request creation")
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
	if tracker != nil && issueKey != "" {
		fetchedIssue, issueErr := tracker.GetIssue(ctx, issueKey)
		if issueErr != nil {
			return issueErr
		}
		issue = fetchedIssue
	}

	// 5. Reviewer Resolution
	explicitReviewers := parseReviewers(opts.Reviewers)
	var reviewers []string
	if len(explicitReviewers) > 0 {
		reviewers = explicitReviewers
	} else if opts.Draft {
		reviewers = nil
	} else {
		var components []string
		if issue != nil {
			components = issue.Components
		}

		changedFiles, _ := a.Inspector.ChangedFiles(ctx, rootDir, targetBranch)

		author := ""
		if userProvider, ok := scm.(interface {
			GetCurrentUser(context.Context) (string, error)
		}); ok {
			author, _ = userProvider.GetCurrentUser(ctx)
		}
		if author == "" {
			author = a.Inspector.Author(ctx, rootDir)
		}

		resOpts := domain.ReviewerResolutionOptions{
			ChangedFiles: changedFiles,
			Labels:       components,
			Branch:       currBranch.Name,
			Author:       author,
		}
		reviewers = scm.ResolveReviewers(resOpts)
	}

	// 6. Title & Description Composition
	issueTitle := ""
	if issue != nil {
		issueTitle = issue.Title
	}
	prTitle := ComposePullRequestTitle(opts.Title, currBranch.Name, issueKey, issueTitle)

	summaryText := ""
	if issue != nil {
		summaryText = issue.Title
	}

	mergeStrategy, err := config.ResolveMergeStrategy(&cfg.Review.Merge, currBranch.Name, targetBranch, opts.Strategy)
	if err != nil {
		return err
	}

	prBody := a.ComposeReviewDescription(cfg, scm, issueKey, summaryText, currBranch.Name, targetBranch, prTitle, mergeStrategy)

	// 7. Pull Request Submission
	req := domain.PullRequestRequest{
		Title:        prTitle,
		Description:  prBody,
		SourceBranch: currBranch.Name,
		TargetBranch: targetBranch,
		Reviewers:    reviewers,
		Draft:        opts.Draft,
	}

	result, err := scm.CreatePullRequest(ctx, req)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "already exists") || strings.Contains(err.Error(), "409") {
			if existing, lookupErr := scm.GetPullRequestForBranch(ctx, currBranch.Name); lookupErr == nil && existing != nil && existing.URL != "" {
				return errcode.NewError(errcode.ErrAPI, "A pull request for branch '%s' already exists (%s)", currBranch.Name, existing.URL)
			}
			return errcode.NewError(errcode.ErrAPI, "A pull request for branch '%s' already exists", currBranch.Name)
		}
		return err
	}

	for _, w := range result.Warnings {
		a.Logger.Warn("%s", w)
	}
	a.Logger.Log("Created pull-request:")
	a.Logger.Out("%s", result.URL)

	return nil
}

// parseReviewers splits comma-separated entries, trims whitespace, drops empty strings,
// and deduplicates handles while preserving order.
func parseReviewers(raw []string) []string {
	if len(raw) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	var out []string
	for _, entry := range raw {
		for _, part := range strings.Split(entry, ",") {
			trimmed := strings.TrimSpace(part)
			if trimmed == "" || seen[trimmed] {
				continue
			}
			seen[trimmed] = true
			out = append(out, trimmed)
		}
	}
	return out
}

// ComposeReviewDescription constructs the PR description text.
func (a *App) ComposeReviewDescription(cfg *domain.ProjectConfig, scm domain.SCMProvider, issueKey, summary, branch, targetBranch string, prTitle string, strategy ...string) string {
	template := ""
	if cfg.Review.Template != "" {
		if content, err := os.ReadFile(cfg.Review.Template); err == nil {
			template = string(content)
		} else {
			template = cfg.Review.Template
		}
	} else {
		// Auto-discover standard repository pull request templates
		standardTemplates := []string{
			".github/pull_request_template.md",
			".github/PULL_REQUEST_TEMPLATE.md",
			"pull_request_template.md",
		}
		for _, path := range standardTemplates {
			if content, err := os.ReadFile(path); err == nil {
				template = string(content)
				break
			}
		}
	}
	template = strings.TrimSpace(template)

	closingRef := ""
	if issueKey != "" && scm != nil {
		closingRef = scm.FormatClosingIssue(issueKey)
	}

	// Append merge instructions unless explicitly disabled via review.merge.instructions: false
	appendInstructions := true
	if cfg.Review.Merge.Instructions != nil {
		appendInstructions = *cfg.Review.Merge.Instructions
	}

	var sb strings.Builder
	if template != "" {
		sb.WriteString(template)
	}

	hasFooter := closingRef != "" || appendInstructions
	if template != "" && hasFooter {
		sb.WriteString("\n\n---\n")
	}

	if closingRef != "" {
		sb.WriteString(closingRef)
		if appendInstructions {
			sb.WriteString("\n\n")
		} else {
			sb.WriteString("\n")
		}
	}

	if appendInstructions {
		resolvedStrategy := ""
		if len(strategy) > 0 && strategy[0] != "" {
			resolvedStrategy = strategy[0]
		} else {
			resolvedStrategy, _ = config.ResolveMergeStrategy(&cfg.Review.Merge, branch, targetBranch, "")
		}

		if prTitle == "" {
			prTitle = ComposePullRequestTitle("", branch, issueKey, summary)
		}

		opts := domain.MergeInstructionOptions{
			PRTitle:      prTitle,
			IssueKey:     issueKey,
			Summary:      summary,
			Branch:       branch,
			TargetBranch: targetBranch,
			Strategy:     resolvedStrategy,
		}
		if scm != nil {
			sb.WriteString(scm.FormatMergeInstructions(opts))
		} else {
			sb.WriteString(domain.FormatDefaultMergeInstructions(opts))
		}
	}

	return sb.String()
}

var convCommitRegex = regexp.MustCompile(`^([a-z]+)(?:\([^)]+\))?:\s*(.+)$`)

// ComposePullRequestTitle constructs the PR title using Conventional Commits with issue scope.
func ComposePullRequestTitle(customTitle, branchName, issueKey, issueTitle string) string {
	if customTitle != "" {
		return customTitle
	}
	if issueTitle == "" {
		return branchName
	}
	if issueKey == "" {
		if convCommitRegex.MatchString(issueTitle) {
			return issueTitle
		}
		return fmt.Sprintf("%s: %s", branchName, issueTitle)
	}

	scope := issueKey
	cleanKey := strings.TrimPrefix(issueKey, "#")
	if _, err := strconv.Atoi(cleanKey); err == nil {
		scope = "#" + cleanKey
	}

	matches := convCommitRegex.FindStringSubmatch(issueTitle)
	if len(matches) == 3 {
		return fmt.Sprintf("%s(%s): %s", matches[1], scope, matches[2])
	}

	if strings.HasPrefix(branchName, "epic/") {
		return fmt.Sprintf("epic(%s): %s", scope, issueTitle)
	}
	if strings.HasPrefix(branchName, "feature/") {
		return fmt.Sprintf("feat(%s): %s", scope, issueTitle)
	}
	if strings.HasPrefix(branchName, "issue/") {
		return fmt.Sprintf("fix(%s): %s", scope, issueTitle)
	}

	return fmt.Sprintf("%s: %s", scope, issueTitle)
}
