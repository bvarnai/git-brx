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

func TestNameIntegration(t *testing.T) {
	t.Run("MasterBranchStdoutPurity", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"name"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitSuccess), code)
		assert.Equal(t, "master\n", stdout.String(), "stdout must contain only branch name and newline")
		assert.Empty(t, stderr.String(), "stderr must be empty on attached clean branch")
	})

	t.Run("TopicBranchStdoutPurity", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.CreateBranch("issue/VSB-500")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"name"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitSuccess), code)
		assert.Equal(t, "issue/VSB-500\n", stdout.String())
		assert.Empty(t, stderr.String())
	})

	t.Run("DeepSubdirectoryInvariant", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.CreateBranch("feature/nested-dir")

		subDir := filepath.Join(h.RepoDir, "sub", "deep", "path")
		err := os.MkdirAll(subDir, 0755)
		require.NoError(t, err)

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), subDir, []string{"name"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitSuccess), code)
		assert.Equal(t, "feature/nested-dir\n", stdout.String())
	})

	t.Run("DetachedHEADPrintsHashAndWarnsOnStderr", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		shortSha := strings.TrimSpace(h.Git("rev-parse", "--short", "HEAD"))

		// Detach HEAD
		h.Git("checkout", "--detach", "HEAD")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"name"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitSuccess), code)
		assert.Equal(t, shortSha+"\n", stdout.String())
		assert.Contains(t, stderr.String(), "Warning: HEAD is detached at "+shortSha)
	})

	t.Run("ExcessArgumentsRejectedWithExit2", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"name", "extra-arg"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitUsageError), code)
		assert.Empty(t, stdout.String(), "stdout must remain empty on error")
		assert.Contains(t, stderr.String(), "Unexpected argument: extra-arg")
	})
}
