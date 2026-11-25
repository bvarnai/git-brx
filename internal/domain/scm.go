package domain

import (
	"context"
	"regexp"
	"strings"
)

// PullRequestRequest defines the parameters for opening a pull request.
type PullRequestRequest struct {
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	SourceBranch string   `json:"source_branch"`
	TargetBranch string   `json:"target_branch"`
	Reviewers    []string `json:"reviewers,omitempty"`
	Draft        bool     `json:"draft,omitempty"`
}

// PullRequestResult captures the identifiers of a created pull request.
type PullRequestResult struct {
	ID       string   `json:"id"`
	URL      string   `json:"url"`
	Warnings []string `json:"warnings,omitempty"`
}

// PullRequestState defines the lifecycle state of a pull request.
type PullRequestState string

const (
	PRStateOpen   PullRequestState = "OPEN"
	PRStateMerged PullRequestState = "MERGED"
	PRStateClosed PullRequestState = "CLOSED"
)

// PullRequestDetail captures detailed metadata about an existing pull request.
type PullRequestDetail struct {
	ID     string           `json:"id"`
	Number int              `json:"number"`
	URL    string           `json:"url"`
	State  PullRequestState `json:"state"`
	Merged bool             `json:"merged"`
}

// SelectPullRequest chooses the most relevant pull request from multiple candidates
// for a reused branch, prioritizing state (OPEN > MERGED > CLOSED) and breaking ties
// by highest PR number. Returns nil when candidates is empty.
func SelectPullRequest(candidates []PullRequestDetail) *PullRequestDetail {
	if len(candidates) == 0 {
		return nil
	}

	best := candidates[0]
	bestPriority := prStatePriority(best.State)
	for _, pr := range candidates[1:] {
		prio := prStatePriority(pr.State)
		if prio > bestPriority || (prio == bestPriority && pr.Number > best.Number) {
			bestPriority = prio
			best = pr
		}
	}

	selected := best
	return &selected
}

func prStatePriority(state PullRequestState) int {
	switch state {
	case PRStateOpen:
		return 3
	case PRStateMerged:
		return 2
	case PRStateClosed:
		return 1
	default:
		return 0
	}
}

// MergeInstructionOptions captures metadata needed to format merge instructions.
type MergeInstructionOptions struct {
	PRTitle      string
	IssueKey     string
	Summary      string
	Branch       string
	TargetBranch string
	Strategy     string // "squash", "merge", "rebase"
}

// ReviewerResolutionOptions specifies parameters for multi-tier reviewer resolution.
type ReviewerResolutionOptions struct {
	ChangedFiles []string
	Labels       []string
	Branch       string
	Author       string
}

// SCMProvider defines the abstract interface for source control platforms (Bitbucket, GitHub, GitLab).
type SCMProvider interface {
	Name() string
	CreatePullRequest(ctx context.Context, req PullRequestRequest) (*PullRequestResult, error)
	GetPullRequestForBranch(ctx context.Context, branch string) (*PullRequestDetail, error)
	ResolveReviewers(opts ReviewerResolutionOptions) []string
	FormatMergeInstructions(opts MergeInstructionOptions) string
	FormatClosingIssue(issueKey string) string
}

// ChildClosingIssuesProvider is an optional interface implemented by SCM providers
// that can discover closing issue keys from pull requests merged into a given base branch.
type ChildClosingIssuesProvider interface {
	ListMergedChildClosingIssues(ctx context.Context, baseBranch string) ([]string, error)
}

var mergeConvPrefixRegex = regexp.MustCompile(`^[a-z]+(?:\([^)]+\))?:\s*(.+)$`)

func mergeCommitSummary(opts MergeInstructionOptions) string {
	candidate := strings.TrimSpace(opts.PRTitle)
	if candidate == "" || candidate == opts.Branch {
		candidate = strings.TrimSpace(opts.Summary)
	}
	if candidate == "" {
		return ""
	}

	if matches := mergeConvPrefixRegex.FindStringSubmatch(candidate); len(matches) == 2 {
		return strings.TrimSpace(matches[1])
	}

	if opts.IssueKey != "" {
		rawKey := strings.TrimPrefix(opts.IssueKey, "#")
		for _, prefix := range []string{"#" + rawKey + ":", rawKey + ":", opts.IssueKey + ":"} {
			if strings.HasPrefix(candidate, prefix) {
				return strings.TrimSpace(strings.TrimPrefix(candidate, prefix))
			}
		}
	}

	return candidate
}

// FormatDefaultMergeInstructions formats generic, UI-agnostic merge instructions.
func FormatDefaultMergeInstructions(opts MergeInstructionOptions) string {
	strategy := opts.Strategy
	if strategy == "" {
		strategy = "squash"
	}

	target := opts.TargetBranch
	if target == "" {
		target = "main"
	}

	switch strategy {
	case "merge":
		mergeMsg := "Merge branch '" + opts.Branch + "' into " + target
		if summary := mergeCommitSummary(opts); summary != "" {
			mergeMsg += ": " + summary
		}
		return "### Merge instructions\n" +
			"- **Strategy:** Create a merge commit\n" +
			"  - **Commit:** `" + mergeMsg + "` (merge commit preserving all constituent commits)\n" +
			"- **Branch cleanup:** Delete source branch `" + opts.Branch + "` after merging\n"
	case "rebase":
		return "### Merge instructions\n" +
			"- **Strategy:** Rebase and merge\n" +
			"  - **Commit:** Linear rebase onto `" + target + "` (all constituent commits preserved, no merge commit)\n" +
			"- **Branch cleanup:** Delete source branch `" + opts.Branch + "` after merging\n"
	default:
		squashTitle := opts.Branch
		if opts.PRTitle != "" {
			squashTitle = opts.PRTitle
		} else if opts.IssueKey != "" && opts.Summary != "" {
			squashTitle = opts.IssueKey + ": " + opts.Summary
		} else if opts.Summary != "" {
			squashTitle = opts.Summary
		}

		return "### Merge instructions\n" +
			"- **Strategy:** Squash and merge (recommended)\n" +
			"  - **Commit:** `" + squashTitle + "` (all commits squashed into a single commit)\n" +
			"- **Branch cleanup:** Delete source branch `" + opts.Branch + "` after merging\n"
	}
}
