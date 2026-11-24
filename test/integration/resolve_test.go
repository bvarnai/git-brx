package integration

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bvarnai/git-brx/internal/cli"
	"github.com/bvarnai/git-brx/internal/errcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveIntegration(t *testing.T) {
	t.Run("CleanRepoLogsNoticeAndExits0", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"resolve"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "No merge or rebase operation is currently in progress")
	})

	t.Run("MergeConflictResolvedAndTargetedStaging", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("shared.txt", "line 1\n", "Initial commit")

		// Create conflict between master and branch
		h.CreateBranch("feature/conflict")
		h.CommitFile("shared.txt", "line from branch\n", "Branch commit")

		h.Git("checkout", "master")
		h.CommitFile("shared.txt", "line from master\n", "Master commit")

		// Merge feature into master to trigger conflict
		h.Git("checkout", "feature/conflict")
		_, _ = h.GitAllowError("merge", "master")

		// Verify conflict exists
		mergeHead := filepath.Join(h.RepoDir, ".git", "MERGE_HEAD")
		_, err := os.Stat(mergeHead)
		require.NoError(t, err, "MERGE_HEAD must exist during conflict")

		// Create an unrelated untracked dirty file to ensure resolve does NOT stage it
		unrelatedFile := filepath.Join(h.RepoDir, "unrelated.txt")
		err = os.WriteFile(unrelatedFile, []byte("unrelated edit"), 0644)
		require.NoError(t, err)

		// Configure a mock merge tool in git that resolves the conflict by writing resolved content
		h.Git("config", "merge.tool", "dummytool")
		h.Git("config", "mergetool.dummytool.cmd", "echo resolved > \"$MERGED\"")
		h.Git("config", "mergetool.dummytool.trustExitCode", "true")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"resolve"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "Merge completed")

		// Verify merge is finalized
		_, statErr := os.Stat(mergeHead)
		assert.True(t, os.IsNotExist(statErr), "MERGE_HEAD must be cleared after merge completion")

		// Verify unrelated file remains UNTRACKED (not staged!)
		status := h.Git("status", "--porcelain")
		assert.Contains(t, status, "?? unrelated.txt", "unrelated file must remain untracked; resolve must not run 'git add .'")
	})

	t.Run("RebaseConflictResolvedAndContinued", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("shared.txt", "line 1\n", "Initial commit")

		h.CreateBranch("issue/rebase-conflict")
		h.CommitFile("shared.txt", "line from issue\n", "Issue commit")

		h.Git("checkout", "master")
		h.CommitFile("shared.txt", "line from master\n", "Master commit")

		h.Git("checkout", "issue/rebase-conflict")
		_, _ = h.GitAllowError("rebase", "master")

		// Configure dummy mergetool
		h.Git("config", "merge.tool", "dummytool")
		h.Git("config", "mergetool.dummytool.cmd", "echo rebased > \"$MERGED\"")
		h.Git("config", "mergetool.dummytool.trustExitCode", "true")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"resolve"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "Rebase completed")

		// Verify rebase completed cleanly
		rebaseMerge := filepath.Join(h.RepoDir, ".git", "rebase-merge")
		_, statErr := os.Stat(rebaseMerge)
		assert.True(t, os.IsNotExist(statErr))
	})

	t.Run("ManuallyResolvedMarkersAutoStagedWithoutMergetool", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("shared.txt", "line 1\n", "Initial commit")

		h.CreateBranch("issue/manual-resolve")
		h.CommitFile("shared.txt", "line from issue\n", "Issue commit")

		h.Git("checkout", "master")
		h.CommitFile("shared.txt", "line from master\n", "Master commit")

		h.Git("checkout", "issue/manual-resolve")
		_, _ = h.GitAllowError("rebase", "master")

		// Unset any merge.tool to ensure resolve does not invoke mergetool
		_, _ = h.GitAllowError("config", "--unset", "merge.tool")

		// Create an unrelated untracked file to verify targeted staging
		unrelated := filepath.Join(h.RepoDir, "untracked.txt")
		require.NoError(t, os.WriteFile(unrelated, []byte("keep untracked"), 0644))

		// Manually resolve conflict markers in editor without running git add
		require.NoError(t, os.WriteFile(filepath.Join(h.RepoDir, "shared.txt"), []byte("manually resolved\n"), 0644))

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"resolve"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code, "stderr: %s", stderr.String())
		assert.Contains(t, stderr.String(), "Rebase completed")

		rebaseMerge := filepath.Join(h.RepoDir, ".git", "rebase-merge")
		_, statErr := os.Stat(rebaseMerge)
		assert.True(t, os.IsNotExist(statErr))

		status := h.Git("status", "--porcelain")
		assert.Contains(t, status, "?? untracked.txt")
	})

	t.Run("HeadlessNoToolListsConflictedFilesAndExits5", func(t *testing.T) {
		h := NewHarness(t)
		t.Setenv("HOME", t.TempDir())
		h.CommitFile("shared.txt", "line 1\n", "Initial commit")

		h.CreateBranch("feature/headless-conflict")
		h.CommitFile("shared.txt", "line from branch\n", "Branch commit")

		h.Git("checkout", "master")
		h.CommitFile("shared.txt", "line from master\n", "Master commit")

		h.Git("checkout", "feature/headless-conflict")
		_, _ = h.GitAllowError("merge", "master")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"resolve", "--no-tool"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitConflict), code)
		assert.Contains(t, stderr.String(), "shared.txt")
		assert.Contains(t, stderr.String(), "git-brx reset")

		// Without --no-tool and without merge.tool configured, resolve also exits 5 fast
		stdout.Reset()
		stderr.Reset()
		code = cli.RunInDir(context.Background(), h.RepoDir, []string{"resolve"}, &stdout, &stderr)
		assert.Equal(t, int(errcode.ExitConflict), code)
		assert.Contains(t, stderr.String(), "shared.txt")

		mergeHead := filepath.Join(h.RepoDir, ".git", "MERGE_HEAD")
		_, statErr := os.Stat(mergeHead)
		assert.NoError(t, statErr, "MERGE_HEAD must remain active when markers are unresolved")
	})

	t.Run("MultiCommitRebaseConflictResolvesAllSteps", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("a.txt", "init a\n", "Initial a")
		h.CommitFile("b.txt", "init b\n", "Initial b")

		h.CreateBranch("issue/multi-step")
		h.CommitFile("a.txt", "branch a\n", "Modify a on branch")
		h.CommitFile("b.txt", "branch b\n", "Modify b on branch")

		h.Git("checkout", "master")
		h.CommitFile("a.txt", "master a\n", "Modify a on master")
		h.CommitFile("b.txt", "master b\n", "Modify b on master")

		h.Git("checkout", "issue/multi-step")
		_, _ = h.GitAllowError("rebase", "master")

		h.Git("config", "merge.tool", "dummytool")
		h.Git("config", "mergetool.dummytool.cmd", "echo resolved > \"$MERGED\"")
		h.Git("config", "mergetool.dummytool.trustExitCode", "true")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"resolve"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code, "stderr: %s", stderr.String())
		assert.Contains(t, stderr.String(), "step 1 of 2")
		assert.Contains(t, stderr.String(), "step 2 of 2")
		assert.Contains(t, stderr.String(), "Rebase completed")

		rebaseMerge := filepath.Join(h.RepoDir, ".git", "rebase-merge")
		_, statErr := os.Stat(rebaseMerge)
		assert.True(t, os.IsNotExist(statErr))
	})
}
