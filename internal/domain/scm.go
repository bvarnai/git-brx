package domain

import "context"

// PullRequestRequest defines the parameters for opening a pull request.
type PullRequestRequest struct {
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	SourceBranch string   `json:"source_branch"`
	TargetBranch string   `json:"target_branch"`
	Reviewers    []string `json:"reviewers,omitempty"`
}

// PullRequestResult captures the identifiers of a created pull request.
type PullRequestResult struct {
	ID  string `json:"id"`
	URL string `json:"url"`
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

// MergeInstructionOptions captures metadata needed to format merge instructions.
type MergeInstructionOptions struct {
	IssueKey     string
	Summary      string
	Branch       string
	TargetBranch string
	Strategy     string // "squash", "merge", "rebase"
}

// SCMProvider defines the abstract interface for source control platforms (Bitbucket, GitHub, GitLab).
type SCMProvider interface {
	Name() string
	CreatePullRequest(ctx context.Context, req PullRequestRequest) (*PullRequestResult, error)
	GetPullRequestForBranch(ctx context.Context, branch string) (*PullRequestDetail, error)
	ResolveReviewers(components []string) []string
	FormatMergeInstructions(opts MergeInstructionOptions) string
	FormatClosingIssue(issueKey string) string
}

// FormatDefaultMergeInstructions formats generic, UI-agnostic merge instructions.
func FormatDefaultMergeInstructions(opts MergeInstructionOptions) string {
	strategy := opts.Strategy
	if strategy == "" {
		strategy = "squash"
	}

	switch strategy {
	case "merge":
		return "# Merge instructions\n" +
			"- **Strategy:** Create a merge commit (`--no-ff`)\n" +
			"  - **Reason:** Multi-commit integration / release branch; preserve constituent commit history\n" +
			"- **Branch Cleanup:** Delete source branch `" + opts.Branch + "` after merging (or follow team retention policy)\n"
	case "rebase":
		target := opts.TargetBranch
		if target == "" {
			target = "main"
		}
		return "# Merge instructions\n" +
			"- **Strategy:** Rebase & Merge\n" +
			"  - **Commit History:** Ensure individual commits are rebased linearly onto `" + target + "`\n" +
			"- **Branch Cleanup:** Delete source branch `" + opts.Branch + "` after merging\n"
	default:
		squashTitle := opts.Branch
		if opts.IssueKey != "" && opts.Summary != "" {
			squashTitle = opts.IssueKey + ": " + opts.Summary
		} else if opts.Summary != "" {
			squashTitle = opts.Summary
		}

		return "# Merge instructions\n" +
			"- **Strategy:** Squash & Merge (recommended)\n" +
			"  - **Commit Title:** `" + squashTitle + "`\n" +
			"- **Branch Cleanup:** Delete source branch `" + opts.Branch + "` after merging\n"
	}
}
