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
		assert.Contains(t, stderr.String(), "Syncing 'issue/VSB-101' with 'master' base branch by default (strategy: rebase)")
		assert.Contains(t, stderr.String(), "Successfully rebased 'issue/VSB-101' onto 'master'")

		// Sync again when already up to date
		stdout.Reset()
		stderr.Reset()
		code = cli.RunInDir(context.Background(), h.RepoDir, []string{"sync"}, &stdout, &stderr)
		assert.Equal(t, int(domain.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "Branch 'issue/VSB-101' is already up to date with 'master'")

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
		assert.Contains(t, stderr.String(), "Syncing 'feature/user-auth' with 'master' base branch by default (strategy: merge)")
		assert.Contains(t, stderr.String(), "Successfully merged 'master' into 'feature/user-auth'")

		// Sync again when already up to date
		stdout.Reset()
		stderr.Reset()
		code = cli.RunInDir(context.Background(), h.RepoDir, []string{"sync"}, &stdout, &stderr)
		assert.Equal(t, int(domain.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "Branch 'feature/user-auth' is already up to date with 'master'")

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
		assert.Contains(t, stderr.String(), "Syncing 'issue/VSB-102' with 'master' base branch by default (strategy: merge)")
		assert.Contains(t, stderr.String(), "Successfully merged 'master' into 'issue/VSB-102'")
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
		assert.Contains(t, stderr.String(), "Syncing 'feature/payment' with 'master' base branch by default (strategy: rebase)")
		assert.Contains(t, stderr.String(), "Successfully rebased 'feature/payment' onto 'master'")

		assert.Equal(t, int(domain.ExitSuccess), code)
		log := h.Git("log", "--oneline")
		assert.Contains(t, log, "Payment commit")
		assert.Contains(t, log, "Core commit")
		assert.NotContains(t, log, "Merge branch")
	})

	t.Run("AutoFastForwardCleanBaseBranchOnIssueBranch", func(t *testing.T) {
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

		assert.Equal(t, int(domain.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "Fast-forwarded local 'master' to match 'origin/master' (1 commit behind)")

		// Verify local master ref matches origin/master
		masterSha := strings.TrimSpace(h.Git("rev-parse", "master"))
		originMasterSha := strings.TrimSpace(h.Git("rev-parse", "origin/master"))
		assert.Equal(t, originMasterSha, masterSha)

		// Verify issue branch contains upstream.txt
		log := h.Git("log", "--oneline")
		assert.Contains(t, log, "Upstream commit")
		assert.Contains(t, log, "Work commit")
	})

	t.Run("AutoFastForwardCleanBaseBranchOnFeatureBranchMerge", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("feature/VSB-103")
		h.CommitFile("feature3.txt", "work", "Feature commit")

		// Push a commit to origin/master without pulling locally into master
		h.Git("checkout", "master")
		h.CommitFile("upstream.txt", "upstream", "Upstream commit")
		h.Git("push", "origin", "master")
		h.Git("reset", "--hard", "HEAD~1") // roll local master back 1 commit

		h.Git("checkout", "feature/VSB-103")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"sync"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "Fast-forwarded local 'master' to match 'origin/master' (1 commit behind)")

		// Verify local master ref matches origin/master
		masterSha := strings.TrimSpace(h.Git("rev-parse", "master"))
		originMasterSha := strings.TrimSpace(h.Git("rev-parse", "origin/master"))
		assert.Equal(t, originMasterSha, masterSha)

		// Verify merge commit exists
		log := h.Git("log", "--oneline")
		assert.Contains(t, log, "Merge branch 'master' into 'feature/VSB-103'")
	})

	t.Run("NoAutoUpdateBaseFlagPreservesManualGuard", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("issue/VSB-104")
		h.CommitFile("issue4.txt", "work", "Work commit")

		h.Git("checkout", "master")
		h.CommitFile("upstream.txt", "upstream", "Upstream commit")
		h.Git("push", "origin", "master")
		h.Git("reset", "--hard", "HEAD~1")

		h.Git("checkout", "issue/VSB-104")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"sync", "--no-auto-update-base"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitConflict), code)
		assert.Contains(t, stderr.String(), "Base branch 'master' is not up-to-date with 'origin/master'")
		assert.Contains(t, stderr.String(), "Switch to 'master' and run 'git-brx update' first")
	})

	t.Run("DivergedBaseBranchFailsWithExit5AndHint", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("issue/VSB-105")
		h.CommitFile("issue5.txt", "work", "Work commit")

		// Push a commit to origin/master
		h.Git("checkout", "master")
		h.CommitFile("upstream.txt", "upstream", "Upstream commit")
		h.Git("push", "origin", "master")
		h.Git("reset", "--hard", "HEAD~1")                                      // roll local master back 1 commit
		h.CommitFile("local_diverged.txt", "diverged", "Local diverged commit") // local commit making master diverged

		// Now local master is diverged from origin/master (ahead 1, behind 1)
		h.Git("checkout", "issue/VSB-105")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"sync"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitConflict), code)
		assert.Contains(t, stderr.String(), "Base branch 'master' is not up-to-date with 'origin/master'")
		assert.Contains(t, stderr.String(), "Switch to 'master' and run 'git-brx update' first")
	})

	t.Run("MergeConflictReturnsExit5AndResolveHint", func(t *testing.T) {
		h := NewHarness(t)
		t.Cleanup(func() {
			h.GitAllowError("merge", "--abort")
		})
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
		assert.Contains(t, stderr.String(), "Use 'git-brx resolve' to resolve conflicts, or 'git-brx reset' to abort")
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
