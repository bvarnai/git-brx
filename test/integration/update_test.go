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
	"github.com/stretchr/testify/require"
)

func TestUpdateIntegration(t *testing.T) {
	t.Run("CleanUpdateFromOrigin", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		remoteBare := h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		// Create topic branch and publish it to origin
		h.CreateBranch("issue/VSB-200")
		h.CommitFile("issue.txt", "my work", "Local commit")
		h.Git("push", "-u", "origin", "issue/VSB-200")

		// Simulate a teammate pushing a new commit to origin/issue/VSB-200
		h2 := CloneHarness(t, remoteBare)
		h2.Git("checkout", "issue/VSB-200")
		h2.CommitFile("colleague.txt", "colleague work", "Colleague commit")
		h2.Git("push", "origin", "issue/VSB-200")

		// In repo h: add local commit on top
		h.CommitFile("local2.txt", "more work", "More local work")

		// Run git-brx update
		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"update"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "Current branch 'issue/VSB-200' is up to date")

		// Verify rebase: colleague commit is now in history
		log := h.Git("log", "--oneline")
		assert.Contains(t, log, "Colleague commit")
		assert.Contains(t, log, "More local work")
	})

	t.Run("UnpublishedBranchFailsWithExit4AndPublishHint", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		// Create branch locally without pushing to remote
		h.CreateBranch("feature/local-only")
		h.CommitFile("local.txt", "local", "Local only commit")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"update"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitPreconditionRemote), code)
		assert.Contains(t, stderr.String(), "Branch 'feature/local-only' has not been published to origin yet")
		assert.Contains(t, stderr.String(), "Use 'git-brx publish' to publish your branch first")
	})

	t.Run("MissingOriginFailsWithExit4", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.CreateBranch("feature/no-remote")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"update"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitPreconditionRemote), code)
		assert.Contains(t, stderr.String(), "Your repository has no remote origin")
	})

	t.Run("RebaseConflictReturnsExit5AndResolveHint", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("file.txt", "initial line\n", "Initial commit")
		remoteBare := h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("issue/VSB-201")
		h.CommitFile("file.txt", "line from local\n", "Local change")
		h.Git("push", "-u", "origin", "issue/VSB-201")
		h.Git("reset", "--hard", "HEAD~1") // roll local back to initial

		// Colleague pushes conflicting change to origin
		h2 := CloneHarness(t, remoteBare)
		h2.Git("checkout", "issue/VSB-201")
		h2.CommitFile("file.txt", "conflicting line from colleague\n", "Colleague change")
		h2.Git("push", "origin", "issue/VSB-201")

		// Local commits conflicting change
		h.CommitFile("file.txt", "different line from local\n", "Local conflicting change")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"update"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitConflict), code)
		assert.Contains(t, stderr.String(), "Update failed due to conflicts")
		assert.Contains(t, stderr.String(), "Use 'git-brx resolve' to resolve conflicts or 'git-brx reset' to abort")
	})

	t.Run("AutostashRebasesOverDirtyTree", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		remoteBare := h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("issue/VSB-202")
		h.CommitFile("tracked.txt", "tracked", "Tracked commit")
		h.Git("push", "-u", "origin", "issue/VSB-202")

		// Colleague pushes new file
		h2 := CloneHarness(t, remoteBare)
		h2.Git("checkout", "issue/VSB-202")
		h2.CommitFile("other.txt", "other content", "Other file commit")
		h2.Git("push", "origin", "issue/VSB-202")

		// Local creates unstaged changes on tracked.txt
		trackedPath := filepath.Join(h.RepoDir, "tracked.txt")
		err := os.WriteFile(trackedPath, []byte("unstaged edit"), 0644)
		require.NoError(t, err)

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"update", "-a"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "Current branch 'issue/VSB-202' is up to date")

		// Verify unstaged edit was preserved after autostash pop
		content, err := os.ReadFile(trackedPath)
		require.NoError(t, err)
		assert.Equal(t, "unstaged edit", string(content))
	})

	t.Run("DryRunModeDoesNotModifyRefs", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		remoteBare := h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("issue/VSB-203")
		h.CommitFile("tracked.txt", "tracked", "Tracked commit")
		h.Git("push", "-u", "origin", "issue/VSB-203")

		origSha := strings.TrimSpace(h.Git("rev-parse", "HEAD"))

		// Colleague pushes
		h2 := CloneHarness(t, remoteBare)
		h2.Git("checkout", "issue/VSB-203")
		h2.CommitFile("other.txt", "other content", "Other file commit")
		h2.Git("push", "origin", "issue/VSB-203")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"update", "--dry-run"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "Would fetch 'origin/issue/VSB-203' and execute: git rebase origin/issue/VSB-203")

		currSha := strings.TrimSpace(h.Git("rev-parse", "HEAD"))
		assert.Equal(t, origSha, currSha, "dry run must not change HEAD commit")
	})
}
