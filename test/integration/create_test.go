package integration

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bvarnai/git-brx/internal/cli"
	"github.com/bvarnai/git-brx/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateIntegration(t *testing.T) {
	t.Run("CreateBranchOffline", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.Git("remote", "add", "origin", "git@github.com:myorg/myrepo.git")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"create", "--offline", "issue/42"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "[git-brx] Creating branch 'issue/42'")
		assert.Contains(t, stderr.String(), "[git-brx] Hint: Use 'git-brx publish' to publish a new branch")

		currentBranch := strings.TrimSpace(h.Git("rev-parse", "--abbrev-ref", "HEAD"))
		assert.Equal(t, "issue/42", currentBranch)
	})

	t.Run("LocalBranchAlreadyExistsGuidance", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.CreateBranch("issue/existing")
		h.Git("checkout", "master")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"create", "--offline", "issue/existing"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "[git-brx] Branch 'issue/existing' found (local)")
		assert.Contains(t, stderr.String(), "[git-brx] Hint: To select that branch, use 'git-brx select issue/existing' instead")

		// Did not change branch
		currentBranch := strings.TrimSpace(h.Git("rev-parse", "--abbrev-ref", "HEAD"))
		assert.Equal(t, "master", currentBranch)
	})

	t.Run("RemoteBranchAlreadyExistsGuidance", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("issue/on-remote")
		h.Git("push", "-u", "origin", "issue/on-remote")
		h.Git("checkout", "master")
		h.Git("branch", "-D", "issue/on-remote")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"create", "-o=false", "issue/on-remote"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "[git-brx] Branch 'issue/on-remote' found (remote)")
		assert.Contains(t, stderr.String(), "[git-brx] Hint: To select this branch, use 'git-brx select issue/on-remote' instead")
	})

	t.Run("BranchNamingPatternMismatchReturnsCode8", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.Git("remote", "add", "origin", "git@github.com:myorg/myrepo.git")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"create", "--offline", "my-feature-branch"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitConfigError), code)
		assert.Contains(t, stderr.String(), "doesn't match pattern")
	})

	t.Run("DryRunModeDoesNotModifyRefs", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.Git("remote", "add", "origin", "git@github.com:myorg/myrepo.git")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"create", "--offline", "--dry-run", "issue/99"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "[git-brx] Would create branch 'issue/99'")

		currentBranch := strings.TrimSpace(h.Git("rev-parse", "--abbrev-ref", "HEAD"))
		assert.Equal(t, "master", currentBranch)

		_, err := h.GitAllowError("show-ref", "--verify", "refs/heads/issue/99")
		require.Error(t, err)
	})

	t.Run("OnlineModeWithMockGitHubServer", func(t *testing.T) {
		// Mock GitHub API server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/repos/testorg/testrepo/issues/42" && r.Method == http.MethodGet {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{
					"number": 42,
					"title": "Fix login crash",
					"state": "open",
					"assignee": {"login": "octocat"},
					"labels": [{"name": "bug"}]
				}`))
				return
			}
			http.NotFound(w, r)
		}))
		defer server.Close()

		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		// Write config pointing to mock server
		configContent := "platform: github\n" +
			"tracker:\n" +
			"  provider: github\n" +
			"  owner: testorg\n" +
			"  repo: testrepo\n" +
			"  uri: " + server.URL + "\n" +
			"scm:\n" +
			"  provider: github\n" +
			"  owner: testorg\n" +
			"  repo: testrepo\n" +
			"  uri: " + server.URL + "\n"
		err := os.WriteFile(filepath.Join(h.RepoDir, ".git-brx.yaml"), []byte(configContent), 0644)
		require.NoError(t, err)

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"create", "--yes", "issue/42"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "Issue #42: Fix login crash")
		assert.Contains(t, stderr.String(), "Assignee: octocat")
		assert.Contains(t, stderr.String(), "Creating branch 'issue/42'")

		currentBranch := strings.TrimSpace(h.Git("rev-parse", "--abbrev-ref", "HEAD"))
		assert.Equal(t, "issue/42", currentBranch)
	})

	t.Run("OnlineModeIssueTracker404ReturnsCode6", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message": "Not Found"}`))
		}))
		defer server.Close()

		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		configContent := "platform: github\n" +
			"tracker:\n" +
			"  provider: github\n" +
			"  owner: testorg\n" +
			"  repo: testrepo\n" +
			"  uri: " + server.URL + "\n"
		err := os.WriteFile(filepath.Join(h.RepoDir, ".git-brx.yaml"), []byte(configContent), 0644)
		require.NoError(t, err)

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"create", "--yes", "issue/999"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitAPIError), code)
		assert.Contains(t, stderr.String(), "GitHub resource not found")
	})
}
