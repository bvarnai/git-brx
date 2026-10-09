package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/bvarnai/git-brx/internal/cli"
	"github.com/bvarnai/git-brx/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReviewIntegration(t *testing.T) {
	t.Run("DryRunFormatsReviewPayload", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")

		originDir := t.TempDir()
		h.Git("init", "--bare", originDir)
		h.Git("remote", "add", "origin", originDir)
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("issue/42")
		h.CommitFile("feature.go", "package main", "Add feature")
		h.Git("push", "-u", "origin", "issue/42")

		configContent := []byte(`
platform: github
scm:
  provider: github
  owner: test-org
  repo: test-repo
`)
		err := os.WriteFile(filepath.Join(h.RepoDir, ".git-brx.yaml"), configContent, 0644)
		require.NoError(t, err)

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"review", "--dry-run", "--no-color"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "Would create pull request")
		assert.Contains(t, stderr.String(), "Source:    issue/42")
		assert.Contains(t, stderr.String(), "Target:    master")
	})

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

		assert.Equal(t, int(domain.ExitPreconditionRemote), code)
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

		assert.Equal(t, int(domain.ExitSuccess), code)
		assert.True(t, prCreated, "PR must have been submitted to mock server")
		assert.Contains(t, stderr.String(), "Created pull-request https://github.com/test-org/test-repo/pull/12")
		assert.Contains(t, stdout.String(), "https://github.com/test-org/test-repo/pull/12")
	})
}
