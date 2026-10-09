package integration

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bvarnai/git-brx/internal/cli"
	"github.com/bvarnai/git-brx/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestSyncIntegration(t *testing.T) {
	t.Run("DefaultSyncRebaseOnIssueBranch", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		// Create issue branch
		h.CreateBranch("issue/VSB-101")
		h.CommitFile("issue.txt", "issue work", "Work on issue")

		// Add commit on master and push to origin
		h.Git("checkout", "master")
		h.CommitFile("master.txt", "master work", "Master commit")
		h.Git("push", "origin", "master")

		// Switch back to issue branch and sync
		h.Git("checkout", "issue/VSB-101")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"sync"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "Syncing 'master' branch by default")

		// Verify rebase happened: issue branch should contain both master.txt and issue.txt
		log := h.Git("log", "--oneline")
		assert.Contains(t, log, "Work on issue")
		assert.Contains(t, log, "Master commit")
	})

	t.Run("DefaultSyncMergeOnFeatureBranch", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		// Create feature branch
		h.CreateBranch("feature/user-auth")
		h.CommitFile("auth.txt", "auth work", "Feature auth commit")

		// Add commit on master and push
		h.Git("checkout", "master")
		h.CommitFile("master.txt", "master work", "Master commit")
		h.Git("push", "origin", "master")

		// Switch back to feature branch and sync
		h.Git("checkout", "feature/user-auth")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"sync"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "Syncing 'master' branch by default")

		// Verify merge commit exists
		log := h.Git("log", "--oneline")
		assert.Contains(t, log, "Merge branch 'master' into 'feature/user-auth'")
	})

	t.Run("StrategyOverrideMergeOnIssueBranch", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("issue/VSB-102")
		h.CommitFile("issue2.txt", "issue2 work", "Issue 2 commit")

		h.Git("checkout", "master")
		h.CommitFile("master2.txt", "master2 work", "Master commit 2")
		h.Git("push", "origin", "master")

		h.Git("checkout", "issue/VSB-102")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"sync", "-m"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitSuccess), code)
		log := h.Git("log", "--oneline")
		assert.Contains(t, log, "Merge branch 'master' into 'issue/VSB-102'")
	})

	t.Run("StrategyOverrideRebaseOnFeatureBranch", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("feature/payment")
		h.CommitFile("payment.txt", "pay", "Payment commit")

		h.Git("checkout", "master")
		h.CommitFile("core.txt", "core", "Core commit")
		h.Git("push", "origin", "master")

		h.Git("checkout", "feature/payment")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"sync", "-r"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitSuccess), code)
		log := h.Git("log", "--oneline")
		assert.Contains(t, log, "Payment commit")
		assert.Contains(t, log, "Core commit")
		assert.NotContains(t, log, "Merge branch")
	})

	t.Run("DivergedBaseBranchFailsWithExit5AndHint", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("issue/VSB-103")
		h.CommitFile("issue3.txt", "work", "Work commit")

		// Push a commit to origin/master without pulling locally into master
		h.Git("checkout", "master")
		h.CommitFile("upstream.txt", "upstream", "Upstream commit")
		h.Git("push", "origin", "master")
		h.Git("reset", "--hard", "HEAD~1") // roll local master back 1 commit

		// Now local master is behind origin/master by 1 commit
		h.Git("checkout", "issue/VSB-103")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"sync"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitConflict), code)
		assert.Contains(t, stderr.String(), "Sync branch 'master' is not up-to-date")
		assert.Contains(t, stderr.String(), "Switch to 'master' branch and use 'git-brx update'")
	})

	t.Run("MergeConflictReturnsExit5AndResolveHint", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("shared.txt", "line 1\nline 2\n", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("feature/conflicting")
		h.CommitFile("shared.txt", "conflict from branch\n", "Feature conflict")

		h.Git("checkout", "master")
		h.CommitFile("shared.txt", "conflict from master\n", "Master conflict")
		h.Git("push", "origin", "master")

		h.Git("checkout", "feature/conflicting")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"sync"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitConflict), code)
		assert.Contains(t, stderr.String(), "Sync failed due to merge conflicts")
		assert.Contains(t, stderr.String(), "Use 'git-brx resolve' in case of merge conflicts or 'git-brx reset' to abort")
	})

	t.Run("DryRunModeDoesNotModifyRefs", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("issue/VSB-104")
		h.CommitFile("issue4.txt", "issue4", "Commit 4")
		origSha := strings.TrimSpace(h.Git("rev-parse", "HEAD"))

		h.Git("checkout", "master")
		h.CommitFile("master4.txt", "master4", "Master commit 4")
		h.Git("push", "origin", "master")

		h.Git("checkout", "issue/VSB-104")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"sync", "--dry-run"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "Would execute: git rebase master")

		currSha := strings.TrimSpace(h.Git("rev-parse", "HEAD"))
		assert.Equal(t, origSha, currSha, "dry run must not change HEAD commit")
	})

	t.Run("ActiveRebaseBlocksSync", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.CreateBranch("issue/VSB-105")

		err := os.MkdirAll(filepath.Join(h.RepoDir, ".git", "rebase-merge"), 0755)
		assert.NoError(t, err)

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"sync"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitConflict), code)
		assert.Contains(t, stderr.String(), "Cannot switch branches during an active rebase")
	})
}
