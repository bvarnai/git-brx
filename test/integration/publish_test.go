package integration

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/bvarnai/git-brx/internal/cli"
	"github.com/bvarnai/git-brx/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublishIntegration(t *testing.T) {
	t.Run("FirstPublishSetsUpstreamAndLease", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		// Create local topic branch
		h.CreateBranch("issue/publish-test")
		h.CommitFile("feature.txt", "feature content", "Add feature")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"publish"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "[git-brx] Branch 'issue/publish-test' successfully published to origin")

		// Verify upstream tracking was configured
		upstream := strings.TrimSpace(h.Git("rev-parse", "--abbrev-ref", "issue/publish-test@{upstream}"))
		assert.Equal(t, "origin/issue/publish-test", upstream)

		// Verify remote ref exists on bare remote
		out := h.Git("ls-remote", "--heads", "origin", "issue/publish-test")
		assert.Contains(t, out, "refs/heads/issue/publish-test")
	})

	t.Run("SubsequentPublishPushesUpdatesWithLease", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("issue/subsequent-publish")
		h.CommitFile("feature1.txt", "v1", "Commit 1")

		// First publish
		var stdout1, stderr1 bytes.Buffer
		code1 := cli.RunInDir(context.Background(), h.RepoDir, []string{"publish"}, &stdout1, &stderr1)
		require.Equal(t, int(domain.ExitSuccess), code1)

		// Make additional commit
		h.CommitFile("feature2.txt", "v2", "Commit 2")

		// Second publish
		var stdout2, stderr2 bytes.Buffer
		code2 := cli.RunInDir(context.Background(), h.RepoDir, []string{"publish"}, &stdout2, &stderr2)
		assert.Equal(t, int(domain.ExitSuccess), code2)
		assert.Contains(t, stderr2.String(), "[git-brx] Branch 'issue/subsequent-publish' successfully published to origin")
	})

	t.Run("DryRunModeDoesNotPushRefToOrigin", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("issue/dry-run-publish")
		h.CommitFile("dry.txt", "dry content", "Dry commit")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"publish", "--dry-run"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "[git-brx] Would publish branch 'issue/dry-run-publish' to origin")

		// Remote should NOT have this branch
		out := h.Git("ls-remote", "--heads", "origin", "issue/dry-run-publish")
		assert.Empty(t, strings.TrimSpace(out))
	})

	t.Run("RefuseForcePushLeaseOnMaster", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"publish"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitUsageError), code)
		assert.Contains(t, stderr.String(), "Refusing to force-push with lease directly to protected 'master' branch")
	})

	t.Run("PublishNoForceAllowsCleanMasterPush", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		h.CommitFile("master2.txt", "content 2", "Second master commit")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"publish", "--no-force"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "[git-brx] Branch 'master' successfully published to origin")
	})
}
