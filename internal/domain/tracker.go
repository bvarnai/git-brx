package domain

import "context"

// Issue represents provider-agnostic issue metadata.
type Issue struct {
	Key        string   `json:"key"`
	Title      string   `json:"title"`
	Type       string   `json:"type"`
	Status     string   `json:"status"`
	Assignee   string   `json:"assignee,omitempty"`
	Components []string `json:"components,omitempty"`
	URL        string   `json:"url,omitempty"`
}

// IssueTracker defines the abstract interface for validating issues across trackers (Jira, GitHub, etc.).
type IssueTracker interface {
	Name() string
	GetIssue(ctx context.Context, key string) (*Issue, error)
}
