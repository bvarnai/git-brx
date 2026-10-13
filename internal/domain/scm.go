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
		return "### Merge instructions\n" +
			"- **Strategy:** Create a merge commit\n" +
			"  - **Commit:** `Merge branch '" + opts.Branch + "' into " + target + "` (merge commit preserving all constituent commits)\n" +
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
