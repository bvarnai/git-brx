package integration

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bvarnai/git-brx/internal/cli"
	"github.com/bvarnai/git-brx/internal/domain"
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

		assert.Equal(t, int(domain.ExitSuccess), code)
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

		assert.Equal(t, int(domain.ExitPreconditionRepo), code)
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

		assert.Equal(t, int(domain.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "Deleted local branch 'issue/VSB-102' and pruned origin")
		assert.Equal(t, "master", h.CurrentBranch())
	})

	t.Run("DryRunLeavesBranchIntact", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")

		originDir := t.TempDir()
		h.Git("init", "--bare", originDir)
		h.Git("remote", "add", "origin", originDir)
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("epic/billing")
		h.CommitFile("bill.go", "package bill", "Billing commit")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"delete", "--dry-run", "--no-color"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "Would checkout branch 'master'")
		assert.Contains(t, stderr.String(), "Would delete local branch 'epic/billing'")
		assert.Contains(t, stderr.String(), "Would prune stale tracking references from origin")

		// Branch must still be current and present
		assert.Equal(t, "epic/billing", h.CurrentBranch())
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

		assert.Equal(t, int(domain.ExitPreconditionRepo), code)
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

		assert.Equal(t, int(domain.ExitConflict), code)
		assert.Contains(t, stderr.String(), "Unable to switch to 'master' branch")
		assert.Equal(t, "issue/conflict", h.CurrentBranch())
	})
}
