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

// SCMProvider defines the abstract interface for source control platforms (Bitbucket, GitHub, GitLab).
type SCMProvider interface {
	Name() string
	CreatePullRequest(ctx context.Context, req PullRequestRequest) (*PullRequestResult, error)
	ResolveReviewers(components []string) []string
}
