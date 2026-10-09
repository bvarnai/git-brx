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

func TestResolveIntegration(t *testing.T) {
	t.Run("CleanRepoLogsNoticeAndExits0", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"resolve"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "No merge/rebase is ongoing")
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

		assert.Equal(t, int(domain.ExitSuccess), code)
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

		assert.Equal(t, int(domain.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "Rebase completed")

		// Verify rebase completed cleanly
		rebaseMerge := filepath.Join(h.RepoDir, ".git", "rebase-merge")
		_, statErr := os.Stat(rebaseMerge)
		assert.True(t, os.IsNotExist(statErr))
	})
}
