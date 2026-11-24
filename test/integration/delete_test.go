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

func TestDeleteIntegration(t *testing.T) {
	t.Run("CleanDeletionOfUnpublishedBranch", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")

		// Create a local bare origin remote
		originDir := t.TempDir()
		h.Git("init", "--bare", originDir)
		h.Git("remote", "add", "origin", originDir)
		h.Git("push", "-u", "origin", "master")

		// Create local topic branch without publishing
		h.CreateBranch("issue/VSB-101")
		h.CommitFile("topic.txt", "work", "Topic commit")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"delete", "--no-color"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "Deleted local branch 'issue/VSB-101' and pruned origin")

		// Verify local branch is gone and current branch is master
		currBranch := h.CurrentBranch()
		assert.Equal(t, "master", currBranch)
		_, err := h.GitAllowError("show-ref", "--verify", "--quiet", "refs/heads/issue/VSB-101")
		assert.Error(t, err, "deleted branch should no longer exist locally")
	})

	t.Run("SafeGateRejectsBranchStillOnRemote", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")

		originDir := t.TempDir()
		h.Git("init", "--bare", originDir)
		h.Git("remote", "add", "origin", originDir)
		h.Git("push", "-u", "origin", "master")

		// Create topic branch and push to origin
		h.CreateBranch("feature/auth")
		h.CommitFile("auth.go", "package auth", "Auth commit")
		h.Git("push", "-u", "origin", "feature/auth")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"delete", "--no-color"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitPreconditionRepo), code)
		assert.Contains(t, stderr.String(), "Branch 'feature/auth' found on remote origin")
		assert.Contains(t, stderr.String(), "Branch must be deleted on remote first")

		// Branch must still exist
		assert.Equal(t, "feature/auth", h.CurrentBranch())
	})

	t.Run("ForceOverrideBypassesRemoteCheck", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")

		originDir := t.TempDir()
		h.Git("init", "--bare", originDir)
		h.Git("remote", "add", "origin", originDir)
		h.Git("push", "-u", "origin", "master")

		// Create and push topic branch
		h.CreateBranch("issue/VSB-102")
		h.CommitFile("w.txt", "w", "commit")
		h.Git("push", "-u", "origin", "issue/VSB-102")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"delete", "--force", "--no-color"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "Deleted local branch 'issue/VSB-102' and pruned origin")
		assert.Equal(t, "master", h.CurrentBranch())
	})

	t.Run("RejectsDeletionOfMasterBranch", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")

		originDir := t.TempDir()
		h.Git("init", "--bare", originDir)
		h.Git("remote", "add", "origin", originDir)
		h.Git("push", "-u", "origin", "master")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"delete", "--no-color"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitPreconditionRepo), code)
		assert.Contains(t, stderr.String(), "You must be on an 'issue', 'feature', or 'epic' branch to delete it")
	})

	t.Run("WorkingTreeConflictBlocksCheckout", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("shared.txt", "master content\n", "Master commit")

		originDir := t.TempDir()
		h.Git("init", "--bare", originDir)
		h.Git("remote", "add", "origin", originDir)
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("issue/conflict")
		h.CommitFile("shared.txt", "branch content\n", "Branch commit")

		// Create an unstaged edit in shared.txt that will collide with checking out master
		err := os.WriteFile(filepath.Join(h.RepoDir, "shared.txt"), []byte("dirty conflict"), 0644)
		require.NoError(t, err)

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"delete", "--force", "--no-color"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitConflict), code)
		assert.Contains(t, stderr.String(), "Unable to switch to 'master' branch")
		assert.Equal(t, "issue/conflict", h.CurrentBranch())
	})

	t.Run("CleanDeletionOfNamedBranchWhileOnMaster", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")

		originDir := t.TempDir()
		h.Git("init", "--bare", originDir)
		h.Git("remote", "add", "origin", originDir)
		h.Git("push", "-u", "origin", "master")

		// Create local topic branch without publishing
		h.CreateBranch("issue/VSB-200")
		h.CommitFile("topic.txt", "work", "Topic commit")

		// Switch back to master
		h.Git("checkout", "master")
		assert.Equal(t, "master", h.CurrentBranch())

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"delete", "issue/VSB-200", "--no-color"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "Deleted local branch 'issue/VSB-200' and pruned origin")

		// Verify current branch is still master and issue/VSB-200 is gone
		assert.Equal(t, "master", h.CurrentBranch())
		_, err := h.GitAllowError("show-ref", "--verify", "--quiet", "refs/heads/issue/VSB-200")
		assert.Error(t, err, "deleted branch should no longer exist locally")
	})

	t.Run("RejectsDeletionOfNonExistentNamedBranch", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")

		originDir := t.TempDir()
		h.Git("init", "--bare", originDir)
		h.Git("remote", "add", "origin", originDir)
		h.Git("push", "-u", "origin", "master")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"delete", "issue/does-not-exist", "--no-color"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitPreconditionRepo), code)
		assert.Contains(t, stderr.String(), "Branch 'issue/does-not-exist' does not exist locally")
	})

	t.Run("RejectsDeletionOfProtectedBranchByArgument", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")

		originDir := t.TempDir()
		h.Git("init", "--bare", originDir)
		h.Git("remote", "add", "origin", originDir)
		h.Git("push", "-u", "origin", "master")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"delete", "master", "--no-color"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitPreconditionRepo), code)
		assert.Contains(t, stderr.String(), "You can only delete an 'issue', 'feature', or 'epic' branch")
	})

	t.Run("ReturnsToRecordedBrxBaseAndAutoFastForwards", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		remoteBare := h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("epic/10")
		h.CommitFile("epic1.txt", "e1", "Epic 1")
		h.Git("push", "-u", "origin", "epic/10")

		h.CreateBranch("issue/44")
		h.Git("config", "--local", "branch.issue/44.brx-base", "epic/10")
		h.CommitFile("slice.txt", "slice", "Slice commit")

		// Teammate advances origin/epic/10 while local epic/10 stays 1 commit behind
		h2 := CloneHarness(t, remoteBare)
		h2.Git("checkout", "epic/10")
		h2.CommitFile("epic_new.txt", "e2", "Merged sibling slice")
		h2.Git("push", "origin", "epic/10")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"delete", "--no-color"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code, "stderr: %s", stderr.String())
		assert.Equal(t, "epic/10", h.CurrentBranch())
		assert.Contains(t, stderr.String(), "Fast-forwarded 'epic/10' to match 'origin/epic/10' (1 new commit)")
		assert.FileExists(t, filepath.Join(h.RepoDir, "epic_new.txt"))
	})

	t.Run("LinkedWorktreeDetachesHeadWhenReturnBranchBusy", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		// Primary worktree remains on master; create issue/wt-del in a linked worktree
		wtDir := filepath.Join(t.TempDir(), "wt-issue-44")
		h.Git("worktree", "add", wtDir, "-b", "issue/44")
		require.NoError(t, os.WriteFile(filepath.Join(wtDir, "wt.txt"), []byte("wt"), 0644))

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), wtDir, []string{"delete", "--no-color"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code, "stderr: %s", stderr.String())
		assert.Contains(t, stderr.String(), "detached HEAD before deleting 'issue/44'")
		assert.Contains(t, stderr.String(), "Deleted local branch 'issue/44' and pruned origin")
		assert.Equal(t, "master", h.CurrentBranch())
		_, refErr := h.GitAllowError("show-ref", "--verify", "--quiet", "refs/heads/issue/44")
		assert.Error(t, refErr)
	})

	t.Run("DeleteRemoteDeletesWhenMergedAndBlocksWhenOpen", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/repos/test-org/test-repo/pulls" && r.Method == http.MethodGet {
				head := r.URL.Query().Get("head")
				w.Header().Set("Content-Type", "application/json")
				if strings.HasSuffix(head, ":issue/del-merged") {
					_, _ = w.Write([]byte(`[{"number": 10, "html_url": "https://github.com/test-org/test-repo/pull/10", "state": "closed", "merged_at": "2026-11-24T12:00:00Z"}]`))
					return
				}
				if strings.HasSuffix(head, ":issue/del-open") {
					_, _ = w.Write([]byte(`[{"number": 11, "html_url": "https://github.com/test-org/test-repo/pull/11", "state": "open", "merged_at": null}]`))
					return
				}
				_, _ = w.Write([]byte(`[]`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		configContent := []byte("platform: github\nscm:\n  provider: github\n  uri: " + server.URL + "\n  owner: test-org\n  repo: test-repo\n")
		require.NoError(t, os.WriteFile(filepath.Join(h.RepoDir, ".git-brx.yaml"), configContent, 0644))

		// Case 1: MERGED PR with --delete-remote deletes remote and local cleanly
		h.CreateBranch("issue/del-merged")
		h.CommitFile("merged.txt", "done", "Merged work")
		h.Git("push", "-u", "origin", "issue/del-merged")

		var out1, err1 bytes.Buffer
		code1 := cli.RunInDir(context.Background(), h.RepoDir, []string{"delete", "--delete-remote", "--no-color"}, &out1, &err1)
		assert.Equal(t, int(errcode.ExitSuccess), code1, "stderr: %s", err1.String())
		assert.Contains(t, err1.String(), "Deleted remote branch 'origin/issue/del-merged'")
		assert.Contains(t, err1.String(), "Deleted local branch 'issue/del-merged' and pruned origin")
		assert.Empty(t, strings.TrimSpace(h.Git("ls-remote", "--heads", "origin", "issue/del-merged")))

		// Case 2: OPEN PR with --delete-remote refuses and keeps both remote and local intact
		h.CreateBranch("issue/del-open")
		h.CommitFile("open.txt", "wip", "Open work")
		h.Git("push", "-u", "origin", "issue/del-open")

		var out2, err2 bytes.Buffer
		code2 := cli.RunInDir(context.Background(), h.RepoDir, []string{"delete", "--delete-remote", "--no-color"}, &out2, &err2)
		assert.Equal(t, int(errcode.ExitPreconditionRepo), code2)
		assert.Contains(t, err2.String(), "Pull request #11 is still OPEN")
		assert.NotEmpty(t, strings.TrimSpace(h.Git("ls-remote", "--heads", "origin", "issue/del-open")))
		assert.Equal(t, "issue/del-open", h.CurrentBranch())
	})

	t.Run("MultiPRPrioritizesOpenOverClosed", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("issue/reused-44")
		h.CommitFile("work.txt", "v2", "Reused branch commit")
		h.Git("push", "-u", "origin", "issue/reused-44")

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/repos/test-org/test-repo/pulls" && r.Method == http.MethodGet {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`[
					{"number": 40, "html_url": "https://github.com/test-org/test-repo/pull/40", "state": "closed", "merged_at": null},
					{"number": 45, "html_url": "https://github.com/test-org/test-repo/pull/45", "state": "open", "merged_at": null}
				]`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		configContent := []byte("platform: github\nscm:\n  provider: github\n  uri: " + server.URL + "\n  owner: test-org\n  repo: test-repo\n")
		require.NoError(t, os.WriteFile(filepath.Join(h.RepoDir, ".git-brx.yaml"), configContent, 0644))

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"delete", "--no-color"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitPreconditionRepo), code)
		assert.Contains(t, stderr.String(), "Pull request #45 is still OPEN")
		assert.Equal(t, "issue/reused-44", h.CurrentBranch())
	})
}
