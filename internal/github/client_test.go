package github_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bvarnai/git-brx/internal/domain"
	"github.com/bvarnai/git-brx/internal/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_GetIssue_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/repos/my-org/my-repo/issues/42", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"number": 42,
			"title": "Fix login crash",
			"state": "open",
			"assignee": {"login": "octocat"},
			"labels": [{"name": "bug"}, {"name": "ui"}]
		}`))
	}))
	defer server.Close()

	client := github.NewClient("my-org", "my-repo", "test-token", github.WithBaseURL(server.URL))

	issue, err := client.GetIssue(context.Background(), "#42")
	require.NoError(t, err)
	assert.Equal(t, "42", issue.Key)
	assert.Equal(t, "Fix login crash", issue.Title)
	assert.Equal(t, "Bug", issue.Type)
	assert.Equal(t, "open", issue.Status)
	assert.Equal(t, "octocat", issue.Assignee)
	assert.Equal(t, []string{"bug", "ui"}, issue.Components)
}

func TestClient_GetIssue_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message": "Not Found"}`))
	}))
	defer server.Close()

	client := github.NewClient("my-org", "my-repo", "test-token", github.WithBaseURL(server.URL))

	_, err := client.GetIssue(context.Background(), "999")
	require.Error(t, err)
	appErr, ok := err.(*domain.AppError)
	require.True(t, ok)
	assert.Equal(t, domain.ExitAPIError, appErr.Code)
	assert.Contains(t, appErr.Message, "not found")
}

func TestClient_GetIssue_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message": "Bad credentials"}`))
	}))
	defer server.Close()

	client := github.NewClient("my-org", "my-repo", "bad-token", github.WithBaseURL(server.URL))

	_, err := client.GetIssue(context.Background(), "42")
	require.Error(t, err)
	appErr, ok := err.(*domain.AppError)
	require.True(t, ok)
	assert.Equal(t, domain.ExitAPIError, appErr.Code)
	assert.Contains(t, appErr.Message, "Bad credentials")
	assert.Contains(t, appErr.Hint, "GITHUB_TOKEN")
}

func TestClient_CreatePullRequest_Success(t *testing.T) {
	reviewersRequested := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/my-org/my-repo/pulls" {
			assert.Equal(t, http.MethodPost, r.Method)
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			assert.Equal(t, "issue/42", body["title"])
			assert.Equal(t, "issue/42", body["head"])
			assert.Equal(t, "master", body["base"])

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{
				"number": 105,
				"html_url": "https://github.com/my-org/my-repo/pull/105"
			}`))
			return
		}

		if r.URL.Path == "/repos/my-org/my-repo/pulls/105/requested_reviewers" {
			assert.Equal(t, http.MethodPost, r.Method)
			reviewersRequested = true
			w.WriteHeader(http.StatusCreated)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := github.NewClient("my-org", "my-repo", "test-token", github.WithBaseURL(server.URL))

	req := domain.PullRequestRequest{
		Title:        "issue/42",
		Description:  "Closes #42",
		SourceBranch: "issue/42",
		TargetBranch: "master",
		Reviewers:    []string{"alice", "bob"},
	}

	result, err := client.CreatePullRequest(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "105", result.ID)
	assert.Equal(t, "https://github.com/my-org/my-repo/pull/105", result.URL)
	assert.True(t, reviewersRequested, "reviewers must be requested after PR creation")
}

func TestClient_ResolveReviewers(t *testing.T) {
	reviewCfg := &domain.ReviewConfig{
		Mapping: domain.ReviewMappingConfig{
			Paths: map[string]any{
				"internal/ui/**": []string{"alice", "bob"},
			},
			Labels: map[string]any{
				"ui":      []string{"alice", "bob"},
				"backend": "charlie",
			},
			Default: "lead",
		},
	}

	client := github.NewClient("my-org", "my-repo", "token", github.WithReviewConfig(reviewCfg))

	assert.Equal(t, []string{"alice", "bob"}, client.ResolveReviewers(domain.ReviewerResolutionOptions{Labels: []string{"ui"}}))
	assert.Equal(t, []string{"charlie"}, client.ResolveReviewers(domain.ReviewerResolutionOptions{Labels: []string{"backend"}}))
	assert.Equal(t, []string{"lead"}, client.ResolveReviewers(domain.ReviewerResolutionOptions{Labels: []string{"unknown"}}))
	assert.Equal(t, []string{"alice", "bob"}, client.ResolveReviewers(domain.ReviewerResolutionOptions{ChangedFiles: []string{"internal/ui/app.go"}}))
}

func TestClient_GetCurrentUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/user", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"login": "octocat"}`))
	}))
	defer server.Close()

	client := github.NewClient("my-org", "my-repo", "test-token", github.WithBaseURL(server.URL))
	login, err := client.GetCurrentUser(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "octocat", login)
}

func TestClient_GetPullRequestForBranch(t *testing.T) {
	t.Run("OpenPullRequest", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/repos/my-org/my-repo/pulls", r.URL.Path)
			assert.Equal(t, "my-org:issue/42", r.URL.Query().Get("head"))
			assert.Equal(t, "all", r.URL.Query().Get("state"))

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[
				{
					"number": 42,
					"html_url": "https://github.com/my-org/my-repo/pull/42",
					"state": "open",
					"merged_at": null
				}
			]`))
		}))
		defer server.Close()

		client := github.NewClient("my-org", "my-repo", "token", github.WithBaseURL(server.URL))
		pr, err := client.GetPullRequestForBranch(context.Background(), "issue/42")
		require.NoError(t, err)
		require.NotNil(t, pr)
		assert.Equal(t, 42, pr.Number)
		assert.Equal(t, domain.PRStateOpen, pr.State)
		assert.False(t, pr.Merged)
	})

	t.Run("MergedPullRequest", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[
				{
					"number": 42,
					"html_url": "https://github.com/my-org/my-repo/pull/42",
					"state": "closed",
					"merged_at": "2026-10-09T08:00:00Z"
				}
			]`))
		}))
		defer server.Close()

		client := github.NewClient("my-org", "my-repo", "token", github.WithBaseURL(server.URL))
		pr, err := client.GetPullRequestForBranch(context.Background(), "issue/42")
		require.NoError(t, err)
		require.NotNil(t, pr)
		assert.Equal(t, 42, pr.Number)
		assert.Equal(t, domain.PRStateMerged, pr.State)
		assert.True(t, pr.Merged)
	})

	t.Run("NoPullRequestFound", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))
		}))
		defer server.Close()

		client := github.NewClient("my-org", "my-repo", "token", github.WithBaseURL(server.URL))
		pr, err := client.GetPullRequestForBranch(context.Background(), "feature/none")
		require.NoError(t, err)
		assert.Nil(t, pr)
	})
}

func TestClient_WithBaseURL_NormalizesGithubCom(t *testing.T) {
	c1 := github.NewClient("my-org", "my-repo", "token", github.WithBaseURL("https://github.com"))
	assert.Equal(t, "https://api.github.com", c1.BaseURL)

	c2 := github.NewClient("my-org", "my-repo", "token", github.WithBaseURL("http://github.com/"))
	assert.Equal(t, "https://api.github.com", c2.BaseURL)

	c3 := github.NewClient("my-org", "my-repo", "token", github.WithBaseURL("https://ghe.company.com/api/v3"))
	assert.Equal(t, "https://ghe.company.com/api/v3", c3.BaseURL)
}

func TestClient_FormatClosingIssue(t *testing.T) {
	client := github.NewClient("my-org", "my-repo", "token")

	tests := []struct {
		input    string
		expected string
	}{
		{"8", "Closes #8"},
		{"#42", "Closes #42"},
		{"issue/99", "Closes #99"},
		{"feature/100", "Closes #100"},
		{"", ""},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, client.FormatClosingIssue(tt.input))
	}
}

func TestClient_FormatMergeInstructions(t *testing.T) {
	client := github.NewClient("my-org", "my-repo", "token")
	opts := domain.MergeInstructionOptions{
		IssueKey:     "10",
		Summary:      "My summary",
		Branch:       "issue/10",
		TargetBranch: "main",
	}

	instructions := client.FormatMergeInstructions(opts)
	assert.Contains(t, instructions, "Squash and merge (recommended)")
	assert.Contains(t, instructions, "10: My summary")
	assert.Contains(t, instructions, "Delete source branch `issue/10` after merging")

	// Merge strategy
	opts.Strategy = "merge"
	opts.Branch = "epic/1"
	mergeInstructions := client.FormatMergeInstructions(opts)
	assert.Contains(t, mergeInstructions, "Create a merge commit")
	assert.Contains(t, mergeInstructions, "Merge branch 'epic/1' into main")

	// Rebase strategy
	opts.Strategy = "rebase"
	rebaseInstructions := client.FormatMergeInstructions(opts)
	assert.Contains(t, rebaseInstructions, "Rebase and merge")
	assert.Contains(t, rebaseInstructions, "Linear rebase onto `main`")
}
