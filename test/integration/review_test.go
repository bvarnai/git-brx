package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bvarnai/git-brx/internal/cli"
	"github.com/bvarnai/git-brx/internal/errcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReviewIntegration(t *testing.T) {
	t.Run("RejectsUnpublishedBranch", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")

		originDir := t.TempDir()
		h.Git("init", "--bare", originDir)
		h.Git("remote", "add", "origin", originDir)
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("issue/99")
		h.CommitFile("feature.go", "package main", "Add feature")
		// Not pushed!

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"review", "--no-color"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitPreconditionRemote), code)
		assert.Contains(t, stderr.String(), "Branch 'issue/99' has not been pushed to remote origin")
		assert.Contains(t, stderr.String(), "Run 'git-brx publish'")
	})

	t.Run("CreatePullRequestSuccessWithMockServer", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")

		originDir := t.TempDir()
		h.Git("init", "--bare", originDir)
		h.Git("remote", "add", "origin", originDir)
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("issue/101")
		h.CommitFile("code.go", "package code", "Write code")
		h.Git("push", "-u", "origin", "issue/101")

		prCreated := false
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/repos/test-org/test-repo/issues/101" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{
					"number": 101,
					"title": "Authentication Refactor",
					"state": "open",
					"labels": [{"name": "auth"}]
				}`))
				return
			}

			if r.URL.Path == "/repos/test-org/test-repo/pulls" && r.Method == http.MethodPost {
				prCreated = true
				var body map[string]string
				_ = json.NewDecoder(r.Body).Decode(&body)
				assert.Equal(t, "issue/101", body["head"])
				assert.Equal(t, "master", body["base"])
				assert.Equal(t, "fix(#101): Authentication Refactor", body["title"])
				assert.Contains(t, body["body"], "Squash and merge (recommended)")
				assert.Contains(t, body["body"], "fix(#101): Authentication Refactor")

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{
					"number": 12,
					"html_url": "https://github.com/test-org/test-repo/pull/12"
				}`))
				return
			}

			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		// Write .git-brx.yaml with test SCM & Tracker pointing to mock server
		configContent := []byte(`
platform: github
tracker:
  provider: github
  uri: ` + server.URL + `
  owner: test-org
  repo: test-repo
scm:
  provider: github
  uri: ` + server.URL + `
  owner: test-org
  repo: test-repo
`)
		err := os.WriteFile(filepath.Join(h.RepoDir, ".git-brx.yaml"), configContent, 0644)
		require.NoError(t, err)

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"review", "--no-color"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code)
		assert.True(t, prCreated, "PR must have been submitted to mock server")
		assert.Contains(t, stderr.String(), "Created pull-request:")
		assert.NotContains(t, stderr.String(), "Created pull-request https://github.com/test-org/test-repo/pull/12")
		assert.Equal(t, "https://github.com/test-org/test-repo/pull/12\n", stdout.String())
	})

	t.Run("ReviewSuppressesMergeInstructionsViaFlag", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")

		originDir := t.TempDir()
		h.Git("init", "--bare", originDir)
		h.Git("remote", "add", "origin", originDir)
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("issue/102")
		h.CommitFile("code.go", "package code", "Write code")
		h.Git("push", "-u", "origin", "issue/102")

		var capturedBody string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/repos/test-org/test-repo/issues/") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"number": 102, "title": "Test Issue", "state": "open"}`))
				return
			}
			if r.URL.Path == "/repos/test-org/test-repo/pulls" && r.Method == http.MethodPost {
				var body map[string]string
				_ = json.NewDecoder(r.Body).Decode(&body)
				capturedBody = body["body"]

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"number": 13, "html_url": "https://github.com/test-org/test-repo/pull/13"}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		configContent := []byte(`
platform: github
tracker:
  provider: github
  uri: ` + server.URL + `
  owner: test-org
  repo: test-repo
scm:
  provider: github
  uri: ` + server.URL + `
  owner: test-org
  repo: test-repo
`)
		err := os.WriteFile(filepath.Join(h.RepoDir, ".git-brx.yaml"), configContent, 0644)
		require.NoError(t, err)

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"review", "--no-merge-instructions"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code, "stderr: %s", stderr.String())
		assert.False(t, strings.Contains(capturedBody, "### Merge instructions"))
	})

	t.Run("ReviewEnforcesMergeInstructionsViaFlagOverridingConfig", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")

		originDir := t.TempDir()
		h.Git("init", "--bare", originDir)
		h.Git("remote", "add", "origin", originDir)
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("issue/103")
		h.CommitFile("code.go", "package code", "Write code")
		h.Git("push", "-u", "origin", "issue/103")

		var capturedBody string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/repos/test-org/test-repo/issues/") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"number": 103, "title": "Test Issue", "state": "open"}`))
				return
			}
			if r.URL.Path == "/repos/test-org/test-repo/pulls" && r.Method == http.MethodPost {
				var body map[string]string
				_ = json.NewDecoder(r.Body).Decode(&body)
				capturedBody = body["body"]

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"number": 14, "html_url": "https://github.com/test-org/test-repo/pull/14"}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		configContent := []byte(`
platform: github
tracker:
  provider: github
  uri: ` + server.URL + `
  owner: test-org
  repo: test-repo
review:
  merge:
    instructions: false
scm:
  provider: github
  uri: ` + server.URL + `
  owner: test-org
  repo: test-repo
`)
		err := os.WriteFile(filepath.Join(h.RepoDir, ".git-brx.yaml"), configContent, 0644)
		require.NoError(t, err)

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"review", "--merge-instructions"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code, "stderr: %s", stderr.String())
		assert.Contains(t, capturedBody, "### Merge instructions")
	})

	t.Run("ReviewEpicBranchDefaultsToMergeCommitInstructions", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")

		originDir := t.TempDir()
		h.Git("init", "--bare", originDir)
		h.Git("remote", "add", "origin", originDir)
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("epic/201")
		h.CommitFile("epic.go", "package epic", "Epic architecture")
		h.Git("push", "-u", "origin", "epic/201")

		var capturedBody string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/repos/test-org/test-repo/issues/") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"number": 201, "title": "Big Epic Initiative", "state": "open", "labels": [{"name": "epic"}]}`))
				return
			}
			if r.URL.Path == "/repos/test-org/test-repo/pulls" && r.Method == http.MethodPost {
				var body map[string]string
				_ = json.NewDecoder(r.Body).Decode(&body)
				capturedBody = body["body"]

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"number": 20, "html_url": "https://github.com/test-org/test-repo/pull/20"}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		configContent := []byte(`
platform: github
tracker:
  provider: github
  uri: ` + server.URL + `
  owner: test-org
  repo: test-repo
scm:
  provider: github
  uri: ` + server.URL + `
  owner: test-org
  repo: test-repo
`)
		err := os.WriteFile(filepath.Join(h.RepoDir, ".git-brx.yaml"), configContent, 0644)
		require.NoError(t, err)

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"review"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code, "stderr: %s", stderr.String())
		assert.Contains(t, capturedBody, "Create a merge commit")
	})

	t.Run("ReviewFlagStrategyRebaseOverridesConvention", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")

		originDir := t.TempDir()
		h.Git("init", "--bare", originDir)
		h.Git("remote", "add", "origin", originDir)
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("epic/202")
		h.CommitFile("epic.go", "package epic", "Epic architecture")
		h.Git("push", "-u", "origin", "epic/202")

		var capturedBody string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/repos/test-org/test-repo/issues/") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"number": 202, "title": "Big Epic Initiative", "state": "open", "labels": [{"name": "epic"}]}`))
				return
			}
			if r.URL.Path == "/repos/test-org/test-repo/pulls" && r.Method == http.MethodPost {
				var body map[string]string
				_ = json.NewDecoder(r.Body).Decode(&body)
				capturedBody = body["body"]

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"number": 21, "html_url": "https://github.com/test-org/test-repo/pull/21"}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		configContent := []byte(`
platform: github
tracker:
  provider: github
  uri: ` + server.URL + `
  owner: test-org
  repo: test-repo
scm:
  provider: github
  uri: ` + server.URL + `
  owner: test-org
  repo: test-repo
`)
		err := os.WriteFile(filepath.Join(h.RepoDir, ".git-brx.yaml"), configContent, 0644)
		require.NoError(t, err)

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"review", "--strategy=rebase"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code, "stderr: %s", stderr.String())
		assert.Contains(t, capturedBody, "Rebase and merge")
	})

	t.Run("ReviewResolvesReviewersFromPathGlobsAndFiltersAuthor", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "mock-token")
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")

		originDir := t.TempDir()
		h.Git("init", "--bare", originDir)
		h.Git("remote", "add", "origin", originDir)
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("issue/301")
		h.CommitFile("internal/ui/button.go", "package ui", "Add UI button")
		h.Git("push", "-u", "origin", "issue/301")

		var requestedReviewers []string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/user" && r.Method == http.MethodGet {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"login": "current-author"}`))
				return
			}
			if strings.HasPrefix(r.URL.Path, "/repos/test-org/test-repo/issues/") {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"number": 301, "title": "Add button", "state": "open"}`))
				return
			}
			if r.URL.Path == "/repos/test-org/test-repo/pulls" && r.Method == http.MethodPost {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"number": 31, "html_url": "https://github.com/test-org/test-repo/pull/31"}`))
				return
			}
			if strings.HasSuffix(r.URL.Path, "/requested_reviewers") && r.Method == http.MethodPost {
				var req struct {
					Reviewers []string `json:"reviewers"`
				}
				_ = json.NewDecoder(r.Body).Decode(&req)
				requestedReviewers = req.Reviewers
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		configContent := []byte(`
platform: github
tracker:
  provider: github
  uri: ` + server.URL + `
  owner: test-org
  repo: test-repo
scm:
  provider: github
  uri: ` + server.URL + `
  owner: test-org
  repo: test-repo
review:
  mapping:
    paths:
      "internal/ui/**": [current-author, carol]
    default: [team-lead]
`)
		err := os.WriteFile(filepath.Join(h.RepoDir, ".git-brx.yaml"), configContent, 0644)
		require.NoError(t, err)

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"review"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code, "stderr: %s", stderr.String())
		// current-author must be excluded, only carol remains
		assert.Equal(t, []string{"carol"}, requestedReviewers)
	})

	t.Run("ReviewFlagOverridesMappingRules", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")

		originDir := t.TempDir()
		h.Git("init", "--bare", originDir)
		h.Git("remote", "add", "origin", originDir)
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("issue/302")
		h.CommitFile("internal/ui/button.go", "package ui", "Add UI button")
		h.Git("push", "-u", "origin", "issue/302")

		var requestedReviewers []string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/repos/test-org/test-repo/issues/") {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"number": 302, "title": "Add button", "state": "open"}`))
				return
			}
			if r.URL.Path == "/repos/test-org/test-repo/pulls" && r.Method == http.MethodPost {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"number": 32, "html_url": "https://github.com/test-org/test-repo/pull/32"}`))
				return
			}
			if strings.HasSuffix(r.URL.Path, "/requested_reviewers") && r.Method == http.MethodPost {
				var req struct {
					Reviewers []string `json:"reviewers"`
				}
				_ = json.NewDecoder(r.Body).Decode(&req)
				requestedReviewers = req.Reviewers
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		configContent := []byte(`
platform: github
tracker:
  provider: github
  uri: ` + server.URL + `
  owner: test-org
  repo: test-repo
scm:
  provider: github
  uri: ` + server.URL + `
  owner: test-org
  repo: test-repo
review:
  mapping:
    paths:
      "internal/ui/**": [carol]
    default: [team-lead]
`)
		err := os.WriteFile(filepath.Join(h.RepoDir, ".git-brx.yaml"), configContent, 0644)
		require.NoError(t, err)

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"review", "--reviewer", "explicit-reviewer"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code, "stderr: %s", stderr.String())
		assert.Equal(t, []string{"explicit-reviewer"}, requestedReviewers)
	})
}
