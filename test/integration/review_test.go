package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

	t.Run("ReviewChildIssueTargetingEpicDefaultsToSquashInstructions", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")

		originDir := t.TempDir()
		h.Git("init", "--bare", originDir)
		h.Git("remote", "add", "origin", originDir)
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("epic/201")
		h.CommitFile("epic.go", "package epic", "Epic architecture")
		h.Git("push", "-u", "origin", "epic/201")

		h.CreateBranch("issue/205")
		h.CommitFile("slice.go", "package epic", "Implement slice")
		h.Git("push", "-u", "origin", "issue/205")

		var capturedBody, capturedBase, capturedHead string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/repos/test-org/test-repo/issues/") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"number": 205, "title": "Implement epic slice", "state": "open", "labels": [{"name": "enhancement"}]}`))
				return
			}
			if r.URL.Path == "/repos/test-org/test-repo/pulls" && r.Method == http.MethodPost {
				var body map[string]string
				_ = json.NewDecoder(r.Body).Decode(&body)
				capturedBody = body["body"]
				capturedBase = body["base"]
				capturedHead = body["head"]

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"number": 22, "html_url": "https://github.com/test-org/test-repo/pull/22"}`))
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
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"review", "epic/201"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code, "stderr: %s", stderr.String())
		assert.Equal(t, "epic/201", capturedBase)
		assert.Equal(t, "issue/205", capturedHead)
		assert.Contains(t, capturedBody, "Squash and merge")
		assert.NotContains(t, capturedBody, "Create a merge commit")
	})

	t.Run("RejectsUnpushedLocalCommits", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("issue/50")
		h.CommitFile("f1.txt", "v1", "Published commit")
		h.Git("push", "-u", "origin", "issue/50")
		h.CommitFile("f2.txt", "v2", "Unpushed commit")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"review", "--no-color"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitPreconditionRemote), code)
		assert.Contains(t, stderr.String(), "has unpushed local commits (out of sync with origin)")
		assert.Contains(t, stderr.String(), "Run 'git-brx publish'")
	})

	t.Run("RejectsMissingRemoteTargetBranchWithSuggestion", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("epic/10")
		h.CommitFile("epic.txt", "epic", "Epic")
		h.Git("push", "-u", "origin", "epic/10")

		h.CreateBranch("issue/50")
		h.CommitFile("slice.txt", "slice", "Slice")
		h.Git("push", "-u", "origin", "issue/50")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"review", "epic/19", "--no-color"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitPreconditionRemote), code)
		assert.Contains(t, stderr.String(), "Target branch 'epic/19' does not exist on remote origin")
		assert.Contains(t, stderr.String(), "Did you mean 'epic/10'?")
	})

	t.Run("UsesRecordedBrxBaseWhenTargetOmitted", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("epic/10")
		h.CommitFile("epic.txt", "epic", "Epic")
		h.Git("push", "-u", "origin", "epic/10")

		h.CreateBranch("issue/51")
		h.Git("config", "branch.issue/51.brx-base", "epic/10")
		h.CommitFile("slice.txt", "slice", "Slice work")
		h.Git("push", "-u", "origin", "issue/51")

		var capturedBase, capturedBody string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/repos/test-org/test-repo/issues/") {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"number": 51, "title": "Recorded base slice", "state": "open"}`))
				return
			}
			if r.URL.Path == "/repos/test-org/test-repo/pulls" && r.Method == http.MethodPost {
				var body map[string]any
				_ = json.NewDecoder(r.Body).Decode(&body)
				capturedBase, _ = body["base"].(string)
				capturedBody, _ = body["body"].(string)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"number": 51, "html_url": "https://github.com/test-org/test-repo/pull/51"}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		configContent := []byte("platform: github\ntracker:\n  provider: github\n  uri: " + server.URL + "\n  owner: test-org\n  repo: test-repo\nscm:\n  provider: github\n  uri: " + server.URL + "\n  owner: test-org\n  repo: test-repo\n")
		require.NoError(t, os.WriteFile(filepath.Join(h.RepoDir, ".git-brx.yaml"), configContent, 0644))

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"review", "--no-color"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code, "stderr: %s", stderr.String())
		assert.Equal(t, "epic/10", capturedBase)
		assert.Contains(t, stderr.String(), "Selecting recorded base 'epic/10' as target branch")
		assert.Contains(t, capturedBody, "Squash and merge")
	})

	t.Run("MultiReviewerDeduplicatesAndWarnsOn422", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("issue/52")
		h.CommitFile("work.txt", "work", "Work")
		h.Git("push", "-u", "origin", "issue/52")

		var capturedReviewers []string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/repos/test-org/test-repo/issues/") {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"number": 52, "title": "Multi reviewer test", "state": "open"}`))
				return
			}
			if r.URL.Path == "/repos/test-org/test-repo/pulls" && r.Method == http.MethodPost {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"number": 52, "html_url": "https://github.com/test-org/test-repo/pull/52"}`))
				return
			}
			if strings.HasSuffix(r.URL.Path, "/requested_reviewers") && r.Method == http.MethodPost {
				var req struct {
					Reviewers []string `json:"reviewers"`
				}
				_ = json.NewDecoder(r.Body).Decode(&req)
				capturedReviewers = req.Reviewers
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnprocessableEntity)
				_, _ = w.Write([]byte(`{"message": "Review cannot be requested from pull request author."}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		configContent := []byte("platform: github\ntracker:\n  provider: github\n  uri: " + server.URL + "\n  owner: test-org\n  repo: test-repo\nscm:\n  provider: github\n  uri: " + server.URL + "\n  owner: test-org\n  repo: test-repo\n")
		require.NoError(t, os.WriteFile(filepath.Join(h.RepoDir, ".git-brx.yaml"), configContent, 0644))

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"review", "-r", "alice,bob", "-r", "bob", "--no-color"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code)
		assert.Equal(t, []string{"alice", "bob"}, capturedReviewers)
		assert.Contains(t, stderr.String(), "Warning: Pull request created, but failed to assign reviewers")
		assert.Equal(t, "https://github.com/test-org/test-repo/pull/52\n", stdout.String())
	})

	t.Run("DraftSkipsAutoMappingAndHonorsExplicitReviewer", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("issue/53")
		h.CommitFile("work.txt", "work", "Work")
		h.Git("push", "-u", "origin", "issue/53")

		var capturedDraft bool
		var requestedReviewers []string
		reviewerPostCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/repos/test-org/test-repo/issues/") {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"number": 53, "title": "Draft mode issue", "state": "open"}`))
				return
			}
			if r.URL.Path == "/repos/test-org/test-repo/pulls" && r.Method == http.MethodPost {
				var body map[string]any
				_ = json.NewDecoder(r.Body).Decode(&body)
				capturedDraft, _ = body["draft"].(bool)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"number": 53, "html_url": "https://github.com/test-org/test-repo/pull/53"}`))
				return
			}
			if strings.HasSuffix(r.URL.Path, "/requested_reviewers") && r.Method == http.MethodPost {
				reviewerPostCount++
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

		configContent := []byte("platform: github\ntracker:\n  provider: github\n  uri: " + server.URL + "\n  owner: test-org\n  repo: test-repo\nscm:\n  provider: github\n  uri: " + server.URL + "\n  owner: test-org\n  repo: test-repo\nreview:\n  mapping:\n    default: [team-lead]\n")
		require.NoError(t, os.WriteFile(filepath.Join(h.RepoDir, ".git-brx.yaml"), configContent, 0644))

		// Without -r, --draft skips automated review.mapping
		var stdout1, stderr1 bytes.Buffer
		code1 := cli.RunInDir(context.Background(), h.RepoDir, []string{"review", "--draft", "--no-color"}, &stdout1, &stderr1)
		assert.Equal(t, int(errcode.ExitSuccess), code1, "stderr: %s", stderr1.String())
		assert.True(t, capturedDraft)
		assert.Equal(t, 0, reviewerPostCount)
		assert.Equal(t, "https://github.com/test-org/test-repo/pull/53\n", stdout1.String())

		// With explicit -r alice, --draft still sends draft: true and assigns alice
		capturedDraft = false
		var stdout2, stderr2 bytes.Buffer
		code2 := cli.RunInDir(context.Background(), h.RepoDir, []string{"review", "--draft", "-r", "alice", "--no-color"}, &stdout2, &stderr2)
		assert.Equal(t, int(errcode.ExitSuccess), code2, "stderr: %s", stderr2.String())
		assert.True(t, capturedDraft)
		assert.Equal(t, 1, reviewerPostCount)
		assert.Equal(t, []string{"alice"}, requestedReviewers)
	})

	t.Run("DuplicatePRIncludesExistingURL", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("issue/54")
		h.CommitFile("work.txt", "work", "Work")
		h.Git("push", "-u", "origin", "issue/54")

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/repos/test-org/test-repo/issues/") {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"number": 54, "title": "Duplicate PR", "state": "open"}`))
				return
			}
			if r.URL.Path == "/repos/test-org/test-repo/pulls" && r.Method == http.MethodPost {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnprocessableEntity)
				_, _ = w.Write([]byte(`{"message": "Validation Failed", "errors": [{"message": "A pull request already exists for test-org:issue/54."}]}`))
				return
			}
			if r.URL.Path == "/repos/test-org/test-repo/pulls" && r.Method == http.MethodGet {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`[{"number": 77, "html_url": "https://github.com/test-org/test-repo/pull/77", "state": "open", "merged_at": null}]`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		configContent := []byte("platform: github\ntracker:\n  provider: github\n  uri: " + server.URL + "\n  owner: test-org\n  repo: test-repo\nscm:\n  provider: github\n  uri: " + server.URL + "\n  owner: test-org\n  repo: test-repo\n")
		require.NoError(t, os.WriteFile(filepath.Join(h.RepoDir, ".git-brx.yaml"), configContent, 0644))

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"review", "--no-color"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitAPIError), code)
		assert.Contains(t, stderr.String(), "A pull request for branch 'issue/54' already exists (https://github.com/test-org/test-repo/pull/77)")
	})

	t.Run("RemoteOnlyTargetComputesChangedFilesFromOrigin", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		remoteBare := h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		h2 := CloneHarness(t, remoteBare)
		h2.Git("checkout", "-b", "epic/remote-10")
		h2.CommitFile("epic.txt", "epic", "Remote epic base")
		h2.Git("push", "-u", "origin", "epic/remote-10")

		// Fetch origin so origin/epic/remote-10 exists without a local epic/remote-10 branch
		h.Git("fetch", "origin")
		h.CreateBranch("issue/55")
		h.CommitFile("internal/ui/widget.go", "package ui", "Add UI widget")
		h.Git("push", "-u", "origin", "issue/55")

		var requestedReviewers []string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/repos/test-org/test-repo/issues/") {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"number": 55, "title": "Widget work", "state": "open"}`))
				return
			}
			if r.URL.Path == "/repos/test-org/test-repo/pulls" && r.Method == http.MethodPost {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"number": 55, "html_url": "https://github.com/test-org/test-repo/pull/55"}`))
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

		configContent := []byte("platform: github\ntracker:\n  provider: github\n  uri: " + server.URL + "\n  owner: test-org\n  repo: test-repo\nscm:\n  provider: github\n  uri: " + server.URL + "\n  owner: test-org\n  repo: test-repo\nreview:\n  mapping:\n    paths:\n      \"internal/ui/**\": [carol]\n")
		require.NoError(t, os.WriteFile(filepath.Join(h.RepoDir, ".git-brx.yaml"), configContent, 0644))

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"review", "epic/remote-10", "--no-color"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code, "stderr: %s", stderr.String())
		assert.Equal(t, []string{"carol"}, requestedReviewers)
	})

	t.Run("EpicAggregatesMergedChildClosingReferences", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("epic/91")
		h.CommitFile("epic.txt", "epic", "Epic work")
		h.Git("push", "-u", "origin", "epic/91")

		var capturedBody string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/repos/test-org/test-repo/issues/91" {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"number": 91, "title": "Workflow hardening", "state": "open"}`))
				return
			}
			if r.URL.Path == "/repos/test-org/test-repo/pulls" && r.Method == http.MethodGet {
				assert.Equal(t, "epic/91", r.URL.Query().Get("base"))
				assert.Equal(t, "closed", r.URL.Query().Get("state"))
				assert.Equal(t, "100", r.URL.Query().Get("per_page"))
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`[
					{"number": 95, "merged_at": "2026-11-24T10:00:00Z", "body": "Closes #94\nCloses other/repo#555"},
					{"number": 93, "merged_at": "2026-11-24T09:00:00Z", "body": "Fixes: #92\nCloses #91"},
					{"number": 99, "merged_at": null, "body": "Closes #99"}
				]`))
				return
			}
			if r.URL.Path == "/repos/test-org/test-repo/pulls" && r.Method == http.MethodPost {
				var body map[string]any
				_ = json.NewDecoder(r.Body).Decode(&body)
				capturedBody, _ = body["body"].(string)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"number": 102, "html_url": "https://github.com/test-org/test-repo/pull/102"}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		configContent := []byte("platform: github\ntracker:\n  provider: github\n  uri: " + server.URL + "\n  owner: test-org\n  repo: test-repo\nscm:\n  provider: github\n  uri: " + server.URL + "\n  owner: test-org\n  repo: test-repo\n")
		require.NoError(t, os.WriteFile(filepath.Join(h.RepoDir, ".git-brx.yaml"), configContent, 0644))

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"review", "master", "--no-color"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code, "stderr: %s", stderr.String())
		assert.True(t, strings.HasPrefix(capturedBody, "Closes #91\nCloses #92\nCloses #94\n\n### Merge instructions"), "body:\n%s", capturedBody)
		assert.NotContains(t, capturedBody, "Closes #99")
		assert.NotContains(t, capturedBody, "#555")
		assert.Contains(t, stderr.String(), "Including 2 child closing references from merged pull requests targeting 'epic/91'")
		assert.Equal(t, "https://github.com/test-org/test-repo/pull/102\n", stdout.String())
	})

	t.Run("NonEpicBranchSkipsChildClosingLookup", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("feature/56")
		h.CommitFile("feat.txt", "feat", "Feature work")
		h.Git("push", "-u", "origin", "feature/56")

		pullsGetCalled := false
		var capturedBody string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/repos/test-org/test-repo/issues/56" {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"number": 56, "title": "Feature work", "state": "open"}`))
				return
			}
			if r.URL.Path == "/repos/test-org/test-repo/pulls" && r.Method == http.MethodGet {
				pullsGetCalled = true
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`[]`))
				return
			}
			if r.URL.Path == "/repos/test-org/test-repo/pulls" && r.Method == http.MethodPost {
				var body map[string]any
				_ = json.NewDecoder(r.Body).Decode(&body)
				capturedBody, _ = body["body"].(string)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"number": 56, "html_url": "https://github.com/test-org/test-repo/pull/56"}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		configContent := []byte("platform: github\ntracker:\n  provider: github\n  uri: " + server.URL + "\n  owner: test-org\n  repo: test-repo\nscm:\n  provider: github\n  uri: " + server.URL + "\n  owner: test-org\n  repo: test-repo\n")
		require.NoError(t, os.WriteFile(filepath.Join(h.RepoDir, ".git-brx.yaml"), configContent, 0644))

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"review", "master", "--no-color"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code, "stderr: %s", stderr.String())
		assert.False(t, pullsGetCalled, "GET /pulls must not be called for non-epic branch")
		assert.True(t, strings.HasPrefix(capturedBody, "Closes #56\n\n### Merge instructions"), "body:\n%s", capturedBody)
	})

	t.Run("EpicChildLookupFailureWarnsAndCreatesPR", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("epic/92")
		h.CommitFile("epic.txt", "epic", "Epic work")
		h.Git("push", "-u", "origin", "epic/92")

		var capturedBody string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/repos/test-org/test-repo/issues/92" {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"number": 92, "title": "Resilient epic", "state": "open"}`))
				return
			}
			if r.URL.Path == "/repos/test-org/test-repo/pulls" && r.Method == http.MethodGet {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"message": "Upstream outage"}`))
				return
			}
			if r.URL.Path == "/repos/test-org/test-repo/pulls" && r.Method == http.MethodPost {
				var body map[string]any
				_ = json.NewDecoder(r.Body).Decode(&body)
				capturedBody, _ = body["body"].(string)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"number": 108, "html_url": "https://github.com/test-org/test-repo/pull/108"}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		configContent := []byte("platform: github\ntracker:\n  provider: github\n  uri: " + server.URL + "\n  owner: test-org\n  repo: test-repo\nscm:\n  provider: github\n  uri: " + server.URL + "\n  owner: test-org\n  repo: test-repo\n")
		require.NoError(t, os.WriteFile(filepath.Join(h.RepoDir, ".git-brx.yaml"), configContent, 0644))

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"review", "master", "--no-color"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code, "stderr: %s", stderr.String())
		assert.Contains(t, stderr.String(), "Warning: Unable to fetch merged child pull requests for 'epic/92'; omitting child closing references")
		assert.True(t, strings.HasPrefix(capturedBody, "Closes #92\n\n### Merge instructions"), "body:\n%s", capturedBody)
		assert.Equal(t, "https://github.com/test-org/test-repo/pull/108\n", stdout.String())
	})

	t.Run("EpicExceedingTenClosingReferencesWarns", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("epic/93")
		h.CommitFile("epic.txt", "epic", "Epic work")
		h.Git("push", "-u", "origin", "epic/93")

		var prItems []string
		for issueNum := 101; issueNum <= 110; issueNum++ {
			prItems = append(prItems, fmt.Sprintf(`{"number": %d, "merged_at": "2026-11-24T10:00:00Z", "body": "Closes #%d"}`, issueNum+100, issueNum))
		}
		prsJSON := "[" + strings.Join(prItems, ",") + "]"

		var capturedBody string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/repos/test-org/test-repo/issues/93" {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"number": 93, "title": "Large epic", "state": "open"}`))
				return
			}
			if r.URL.Path == "/repos/test-org/test-repo/pulls" && r.Method == http.MethodGet {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(prsJSON))
				return
			}
			if r.URL.Path == "/repos/test-org/test-repo/pulls" && r.Method == http.MethodPost {
				var body map[string]any
				_ = json.NewDecoder(r.Body).Decode(&body)
				capturedBody, _ = body["body"].(string)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"number": 109, "html_url": "https://github.com/test-org/test-repo/pull/109"}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		configContent := []byte("platform: github\ntracker:\n  provider: github\n  uri: " + server.URL + "\n  owner: test-org\n  repo: test-repo\nscm:\n  provider: github\n  uri: " + server.URL + "\n  owner: test-org\n  repo: test-repo\n")
		require.NoError(t, os.WriteFile(filepath.Join(h.RepoDir, ".git-brx.yaml"), configContent, 0644))

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"review", "master", "--no-color"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code, "stderr: %s", stderr.String())
		assert.Contains(t, stderr.String(), "Warning: Pull request includes 11 closing references, which exceeds GitHub's limit of 10 auto-closed issues per pull request")
		assert.Contains(t, capturedBody, "Closes #93\nCloses #101\nCloses #102\nCloses #103\nCloses #104\nCloses #105\nCloses #106\nCloses #107\nCloses #108\nCloses #109\nCloses #110")
	})
}
