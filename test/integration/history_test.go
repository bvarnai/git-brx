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

func TestHistoryIntegration(t *testing.T) {
	t.Run("FormattedHistoryStreamsDAG", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("file1.txt", "content1", "First commit")
		h.CommitFile("file2.txt", "content2", "Second commit")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"history", "--no-color"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitSuccess), code)
		assert.Empty(t, stderr.String(), "stderr should be empty on successful history execution")

		out := stdout.String()
		assert.Contains(t, out, "First commit")
		assert.Contains(t, out, "Second commit")
		assert.Contains(t, out, "Test User")
		assert.True(t, strings.HasPrefix(out, "*"), "stdout must contain graph asterisk prefix")
	})

	t.Run("LimitShortFlag", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("f1.txt", "1", "Commit 1")
		h.CommitFile("f2.txt", "2", "Commit 2")
		h.CommitFile("f3.txt", "3", "Commit 3")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"history", "-l", "1", "--no-color"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitSuccess), code)
		out := stdout.String()
		assert.Contains(t, out, "Commit 3")
		assert.NotContains(t, out, "Commit 2")
		assert.NotContains(t, out, "Commit 1")
	})

	t.Run("LimitFlagAlias", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("f1.txt", "1", "Commit 1")
		h.CommitFile("f2.txt", "2", "Commit 2")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"history", "--limit", "1", "--no-color"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitSuccess), code)
		out := stdout.String()
		assert.Contains(t, out, "Commit 2")
		assert.NotContains(t, out, "Commit 1")
	})

	t.Run("PathScopingSingleFileWithFollow", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("target.txt", "v1", "Commit target v1")
		h.CommitFile("unrelated.txt", "other", "Commit unrelated")
		h.CommitFile("target.txt", "v2", "Commit target v2")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"history", "target.txt", "--no-color"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitSuccess), code)
		out := stdout.String()
		assert.Contains(t, out, "Commit target v1")
		assert.Contains(t, out, "Commit target v2")
		assert.NotContains(t, out, "Commit unrelated")
	})

	t.Run("PathScopingDirectory", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("pkg/sub/mod.go", "package sub", "Commit in pkg")
		h.CommitFile("docs/readme.md", "# Docs", "Commit in docs")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"history", "pkg", "--no-color"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitSuccess), code)
		out := stdout.String()
		assert.Contains(t, out, "Commit in pkg")
		assert.NotContains(t, out, "Commit in docs")
	})

	t.Run("TopicBranchModeFiltersToTopicBranchCommits", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("base.txt", "base", "Base master commit")
		h.CreateBranch("issue/VSB-200")
		h.CommitFile("topic.txt", "topic", "Topic branch commit only")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"history", "--topic", "--no-color"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitSuccess), code)
		out := stdout.String()
		assert.Contains(t, out, "Topic branch commit only")
		assert.NotContains(t, out, "Base master commit")
	})

	t.Run("TopicBranchModeOnMasterBranchLogsNoticeAndExitsCleanly", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("base.txt", "base", "Base master commit")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"history", "--topic", "--no-color"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitSuccess), code)
		assert.Empty(t, stdout.String())
		assert.Contains(t, stderr.String(), "You are on base branch 'master'; there is no topic branch delta to display")
	})

	t.Run("BranchFlagScopesToNamedBranch", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("base.txt", "base", "Base master commit")
		h.CreateBranch("feature/other")
		h.CommitFile("other.txt", "other", "Other branch commit")
		h.Git("checkout", "master")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"history", "-b", "feature/other", "--no-color"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitSuccess), code)
		out := stdout.String()
		assert.Contains(t, out, "Other branch commit")
	})

	t.Run("SearchKeywordFilter", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("f1.txt", "1", "feat(auth): add login")
		h.CommitFile("f2.txt", "2", "fix(db): resolve deadlock")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"history", "-s", "deadlock", "--no-color"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitSuccess), code)
		out := stdout.String()
		assert.Contains(t, out, "resolve deadlock")
		assert.NotContains(t, out, "add login")
	})

	t.Run("StatDiffPeek", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("stat_test.txt", "line1\nline2\n", "Add stat test file")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"history", "--stat", "--no-color"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitSuccess), code)
		out := stdout.String()
		assert.Contains(t, out, "stat_test.txt")
		assert.Contains(t, out, "1 file changed")
	})

	t.Run("UnbornRepositoryNoCommits", func(t *testing.T) {
		h := NewHarness(t)
		// No commits created in new repository

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"history"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitPreconditionRepo), code)
		assert.Contains(t, stderr.String(), "Repository has no commits yet")
		assert.Empty(t, stdout.String())
	})

	t.Run("OutsideGitRepositoryReturnsCode3", func(t *testing.T) {
		notRepo := t.TempDir()

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), notRepo, []string{"history"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitPreconditionRepo), code)
		assert.Contains(t, stderr.String(), "Awh! This is not a git repository")
	})

	t.Run("DeepSubdirectoryInvariant", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("root.txt", "root", "Root commit")

		subDir := filepath.Join(h.RepoDir, "sub", "deep", "dir")
		err := os.MkdirAll(subDir, 0755)
		require.NoError(t, err)

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), subDir, []string{"history", "--no-color"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitSuccess), code)
		assert.Contains(t, stdout.String(), "Root commit")
	})

	t.Run("ExcessArgumentRejection", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("root.txt", "root", "Root commit")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"history", "path1", "path2"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitUsageError), code)
		assert.Contains(t, stderr.String(), "Unexpected argument: path2")
	})

	t.Run("BranchNameAsPathRejectedWithUsageError", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("root.txt", "root", "Root commit")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"history", "master"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitUsageError), code)
		assert.Contains(t, stderr.String(), "'master' is a branch name, not a file path. Use '-b master' to scope to this branch")
	})

	t.Run("NonexistentPathRejectedWithUsageError", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("root.txt", "root", "Root commit")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"history", "does_not_exist.txt"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitUsageError), code)
		assert.Contains(t, stderr.String(), "Path 'does_not_exist.txt' does not exist in repository")
	})

	t.Run("ZeroCommitsFeedback", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("root.txt", "root", "Root commit")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"history", "-s", "nonexistent-query-string", "--no-color"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "No commits found matching the specified criteria")
		assert.Empty(t, stdout.String())
	})
}
