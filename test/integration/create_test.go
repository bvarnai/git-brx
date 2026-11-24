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
	"github.com/bvarnai/git-brx/internal/errcode"
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

		assert.Equal(t, int(errcode.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "[git-brx] Creating branch 'issue/42'")
		assert.Contains(t, stderr.String(), "[git-brx] Hint: Use 'git-brx publish' to publish your new branch")

		currentBranch := strings.TrimSpace(h.Git("rev-parse", "--abbrev-ref", "HEAD"))
		assert.Equal(t, "issue/42", currentBranch)
	})

	t.Run("LocalBranchAlreadyExistsAutoSwitches", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.CreateBranch("issue/existing")
		h.Git("checkout", "master")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"create", "--offline", "issue/existing"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "Branch 'issue/existing' already exists; switched to 'issue/existing'")

		currentBranch := strings.TrimSpace(h.Git("rev-parse", "--abbrev-ref", "HEAD"))
		assert.Equal(t, "issue/existing", currentBranch)
	})

	t.Run("RemoteBranchAlreadyExistsAutoSwitches", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("issue/on-remote")
		h.CommitFile("remote.txt", "remote", "Remote commit")
		h.Git("push", "-u", "origin", "issue/on-remote")
		h.Git("checkout", "master")
		h.Git("branch", "-D", "issue/on-remote")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"create", "-o=false", "issue/on-remote"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "Branch 'issue/on-remote' already exists; switched to 'issue/on-remote'")
		currentBranch := strings.TrimSpace(h.Git("rev-parse", "--abbrev-ref", "HEAD"))
		assert.Equal(t, "issue/on-remote", currentBranch)
	})

	t.Run("BranchNamingPatternMismatchReturnsCode8", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.Git("remote", "add", "origin", "git@github.com:myorg/myrepo.git")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"create", "--offline", "my-feature-branch"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitConfigError), code)
		assert.Contains(t, stderr.String(), "doesn't match pattern")
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

		assert.Equal(t, int(errcode.ExitSuccess), code)
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

		assert.Equal(t, int(errcode.ExitAPIError), code)
		assert.Contains(t, stderr.String(), "GitHub resource not found")
	})

	t.Run("DirectGitHubLabelMappingInBranchMapping", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/repos/testorg/testrepo/issues/101" && r.Method == http.MethodGet {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{
					"number": 101,
					"title": "Add analytics pipeline",
					"state": "open",
					"assignee": {"login": "octocat"},
					"labels": [{"name": "kind/feature"}]
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
			"  uri: " + server.URL + "\n" +
			"branch:\n" +
			"  mapping:\n" +
			"    kind/feature: feature\n" +
			"    kind/bug: issue\n"
		err := os.WriteFile(filepath.Join(h.RepoDir, ".git-brx.yaml"), []byte(configContent), 0644)
		require.NoError(t, err)

		// 1. Success on feature/101 matching 'kind/feature'
		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"create", "--yes", "feature/101"}, &stdout, &stderr)
		assert.Equal(t, int(errcode.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "Creating branch 'feature/101'")

		// 2. Failure on issue/101 (mismatch guard triggers on direct label)
		h.Git("checkout", "master")
		h.Git("branch", "-D", "feature/101")
		stdout.Reset()
		stderr.Reset()
		code = cli.RunInDir(context.Background(), h.RepoDir, []string{"create", "--yes", "issue/101"}, &stdout, &stderr)
		assert.Equal(t, int(errcode.ExitConfigError), code)
		assert.Contains(t, stderr.String(), "Issue type 'kind/feature' is not allowed on 'issue' branch")
		assert.Contains(t, stderr.String(), "Use 'feature' branch or check mapping rules")
	})

	t.Run("DirectGitHubLabelMappingCaseInsensitiveAndPrecedence", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/repos/testorg/testrepo/issues/102" && r.Method == http.MethodGet {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{
					"number": 102,
					"title": "Refactor parser",
					"state": "open",
					"assignee": {"login": "octocat"},
					"labels": [{"name": "Enhancement"}]
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

		// Direct label "enhancement: issue" overrides default "Story: feature" inference
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
			"  uri: " + server.URL + "\n" +
			"branch:\n" +
			"  mapping:\n" +
			"    enhancement: issue\n" +
			"    Story: feature\n"
		err := os.WriteFile(filepath.Join(h.RepoDir, ".git-brx.yaml"), []byte(configContent), 0644)
		require.NoError(t, err)

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"create", "--yes", "issue/102"}, &stdout, &stderr)
		assert.Equal(t, int(errcode.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "Creating branch 'issue/102'")
	})

	t.Run("PartialConfigWithoutPlatformAutoInfersPlatformFromOrigin", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/repos/testorg/testrepo/issues/103" && r.Method == http.MethodGet {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{
					"number": 103,
					"title": "Add auto-fallback support",
					"state": "open",
					"assignee": {"login": "octocat"},
					"labels": [{"name": "enhancement"}]
				}`))
				return
			}
			http.NotFound(w, r)
		}))
		defer server.Close()

		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		remoteDir := h.SetupRemoteBare()
		h.Git("remote", "remove", "origin")
		h.Git("config", "url."+remoteDir+".insteadOf", "git@github.com:testorg/testrepo.git")
		h.Git("remote", "add", "origin", "git@github.com:testorg/testrepo.git")
		h.Git("push", "-u", "origin", "master")

		// .git-brx.yaml omits platform: entirely, only setting custom branch.mapping and mock URI overrides
		configContent := "tracker:\n" +
			"  uri: " + server.URL + "\n" +
			"scm:\n" +
			"  uri: " + server.URL + "\n" +
			"branch:\n" +
			"  mapping:\n" +
			"    enhancement: issue\n"
		err := os.WriteFile(filepath.Join(h.RepoDir, ".git-brx.yaml"), []byte(configContent), 0644)
		require.NoError(t, err)

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"create", "--yes", "issue/103"}, &stdout, &stderr)
		assert.Equal(t, int(errcode.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "Issue #103: Add auto-fallback support")
		assert.Contains(t, stderr.String(), "Creating branch 'issue/103'")
	})

	t.Run("ExplicitPlatformNoneSkipsIssueTrackerValidation", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		remoteDir := h.SetupRemoteBare()
		h.Git("remote", "remove", "origin")
		h.Git("config", "url."+remoteDir+".insteadOf", "git@github.com:testorg/testrepo.git")
		h.Git("remote", "add", "origin", "git@github.com:testorg/testrepo.git")
		h.Git("push", "-u", "origin", "master")

		configContent := "platform: none\n" +
			"branch:\n" +
			"  mapping:\n" +
			"    enhancement: issue\n"
		err := os.WriteFile(filepath.Join(h.RepoDir, ".git-brx.yaml"), []byte(configContent), 0644)
		require.NoError(t, err)

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"create", "--yes", "issue/999"}, &stdout, &stderr)
		assert.Equal(t, int(errcode.ExitSuccess), code)
		// Tracker validation skipped; no Target Issue printed
		assert.NotContains(t, stderr.String(), "Target Issue:")
		assert.Contains(t, stderr.String(), "Creating branch 'issue/999'")
	})

	t.Run("DualMappingDefaultGitHubPreset", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/repos/testorg/testrepo/issues/201":
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{
					"number": 201,
					"title": "Add flexible enhancement routing",
					"state": "open",
					"assignee": {"login": "octocat"},
					"labels": [{"name": "enhancement"}]
				}`))
			case "/repos/testorg/testrepo/issues/202":
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{
					"number": 202,
					"title": "Fix crash on startup",
					"state": "open",
					"assignee": {"login": "octocat"},
					"labels": [{"name": "bug"}]
				}`))
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		remoteDir := h.SetupRemoteBare()
		h.Git("remote", "remove", "origin")
		h.Git("config", "url."+remoteDir+".insteadOf", "git@github.com:testorg/testrepo.git")
		h.Git("remote", "add", "origin", "git@github.com:testorg/testrepo.git")
		h.Git("push", "-u", "origin", "master")

		// Point tracker/scm to mock server, using default GitHub preset for template & mapping
		configContent := "platform: github\n" +
			"tracker:\n" +
			"  uri: " + server.URL + "\n" +
			"scm:\n" +
			"  uri: " + server.URL + "\n"
		err := os.WriteFile(filepath.Join(h.RepoDir, ".git-brx.yaml"), []byte(configContent), 0644)
		require.NoError(t, err)

		// 1. Enhancement allows 'issue/' branch
		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"create", "--yes", "issue/201"}, &stdout, &stderr)
		assert.Equal(t, int(errcode.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "Creating branch 'issue/201'")

		// Clean up branch for next test
		h.Git("checkout", "master")
		h.Git("branch", "-D", "issue/201")
		stdout.Reset()
		stderr.Reset()

		// 2. Enhancement also allows 'feature/' branch
		code = cli.RunInDir(context.Background(), h.RepoDir, []string{"create", "--yes", "feature/201"}, &stdout, &stderr)
		assert.Equal(t, int(errcode.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "Creating branch 'feature/201'")

		// 3. Enhancement rejects 'epic/' branch with plural hint
		h.Git("checkout", "master")
		h.Git("branch", "-D", "feature/201")
		stdout.Reset()
		stderr.Reset()
		code = cli.RunInDir(context.Background(), h.RepoDir, []string{"create", "--yes", "epic/201"}, &stdout, &stderr)
		assert.Equal(t, int(errcode.ExitConfigError), code)
		assert.Contains(t, stderr.String(), "Issue type 'enhancement' is not allowed on 'epic' branch")
		assert.Contains(t, stderr.String(), "Use 'issue' or 'feature' branch or check mapping rules")

		// 4. Bug allows 'issue/' branch
		stdout.Reset()
		stderr.Reset()
		code = cli.RunInDir(context.Background(), h.RepoDir, []string{"create", "--yes", "issue/202"}, &stdout, &stderr)
		assert.Equal(t, int(errcode.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "Creating branch 'issue/202'")

		// 5. Bug rejects 'feature/' branch with single target hint
		h.Git("checkout", "master")
		h.Git("branch", "-D", "issue/202")
		stdout.Reset()
		stderr.Reset()
		code = cli.RunInDir(context.Background(), h.RepoDir, []string{"create", "--yes", "feature/202"}, &stdout, &stderr)
		assert.Equal(t, int(errcode.ExitConfigError), code)
		assert.Contains(t, stderr.String(), "Issue type 'bug' is not allowed on 'feature' branch")
		assert.Contains(t, stderr.String(), "Use 'issue' branch or check mapping rules")
	})

	t.Run("MultiBranchCustomMappingArrayAndCommaString", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/repos/testorg/testrepo/issues/301":
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{
					"number": 301,
					"title": "Custom array routing",
					"state": "open",
					"assignee": {"login": "octocat"},
					"labels": [{"name": "rfq"}]
				}`))
			case "/repos/testorg/testrepo/issues/302":
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{
					"number": 302,
					"title": "Comma string routing",
					"state": "open",
					"assignee": {"login": "octocat"},
					"labels": [{"name": "task"}]
				}`))
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		remoteDir := h.SetupRemoteBare()
		h.Git("remote", "remove", "origin")
		h.Git("config", "url."+remoteDir+".insteadOf", "git@github.com:testorg/testrepo.git")
		h.Git("remote", "add", "origin", "git@github.com:testorg/testrepo.git")
		h.Git("push", "-u", "origin", "master")

		configContent := "platform: github\n" +
			"tracker:\n" +
			"  uri: " + server.URL + "\n" +
			"scm:\n" +
			"  uri: " + server.URL + "\n" +
			"branch:\n" +
			"  template: '^(issue|feature|epic)/[1-9][0-9]*$'\n" +
			"  mapping:\n" +
			"    rfq:\n" +
			"      - issue\n" +
			"      - feature\n" +
			"    task: 'issue, feature, epic'\n"
		err := os.WriteFile(filepath.Join(h.RepoDir, ".git-brx.yaml"), []byte(configContent), 0644)
		require.NoError(t, err)

		// rfq on feature succeeds
		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"create", "--yes", "feature/301"}, &stdout, &stderr)
		assert.Equal(t, int(errcode.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "Creating branch 'feature/301'")

		// task on epic succeeds
		stdout.Reset()
		stderr.Reset()
		code = cli.RunInDir(context.Background(), h.RepoDir, []string{"create", "--yes", "epic/302"}, &stdout, &stderr)
		assert.Equal(t, int(errcode.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "Creating branch 'epic/302'")
	})

	t.Run("CreateWithBaseFlagAndParentTracking", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")
		require.NoError(t, os.WriteFile(filepath.Join(h.RepoDir, ".git-brx.yaml"), []byte("platform: none\n"), 0644))

		h.CreateBranch("epic/10")
		h.CommitFile("epic.txt", "epic", "Epic commit")
		h.Git("push", "-u", "origin", "epic/10")
		epicSha := strings.TrimSpace(h.Git("rev-parse", "HEAD"))

		h.Git("checkout", "master")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"create", "--offline", "--base", "epic/10", "issue/44"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code, "stderr: %s", stderr.String())
		assert.Equal(t, "issue/44", strings.TrimSpace(h.Git("rev-parse", "--abbrev-ref", "HEAD")))
		assert.Equal(t, epicSha, strings.TrimSpace(h.Git("rev-parse", "HEAD")))
		assert.Equal(t, "epic/10", strings.TrimSpace(h.Git("config", "--local", "--get", "branch.issue/44.brx-base")))

		// Missing --base fails with exit 3
		stdout.Reset()
		stderr.Reset()
		code = cli.RunInDir(context.Background(), h.RepoDir, []string{"create", "--offline", "--base", "epic/missing", "issue/46"}, &stdout, &stderr)
		assert.Equal(t, int(errcode.ExitPreconditionRepo), code)
		assert.Contains(t, stderr.String(), "Base branch 'epic/missing' not found")
	})

	t.Run("CreateWarningsForTopicForkAndDirtyTree", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.Git("remote", "add", "origin", "git@github.com:myorg/myrepo.git")
		h.CreateBranch("issue/10")
		h.CommitFile("topic.txt", "topic", "Topic commit")

		require.NoError(t, os.WriteFile(filepath.Join(h.RepoDir, "dirty.txt"), []byte("wip"), 0644))

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"create", "--offline", "issue/45"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "Creating 'issue/45' from topic branch 'issue/10' instead of default branch")
		assert.Contains(t, stderr.String(), "Uncommitted local changes will be carried over to 'issue/45'")
		assert.Equal(t, "issue/10", strings.TrimSpace(h.Git("config", "--local", "--get", "branch.issue/45.brx-base")))
	})
}
