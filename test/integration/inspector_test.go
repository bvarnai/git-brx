package integration_test

import (
	"context"
	"testing"

	"github.com/bvarnai/git-brx/internal/domain"
	"github.com/bvarnai/git-brx/internal/git"
	"github.com/bvarnai/git-brx/test/integration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHarnessAndInspector(t *testing.T) {
	h := integration.NewHarness(t)
	h.CommitFile("README.md", "# Test Repo", "Initial commit")

	runner := git.NewExecRunner(nil)
	inspector := git.NewInspector(runner)
	ctx := context.Background()

	// Repo Root Check
	root, err := inspector.RepoRoot(ctx, h.RepoDir)
	require.NoError(t, err)
	assert.Equal(t, h.RepoDir, root)

	// Assert Current Branch
	rec, err := inspector.CurrentBranch(ctx, h.RepoDir)
	require.NoError(t, err)
	assert.Equal(t, "master", rec.Name)
	assert.Equal(t, domain.BranchTypeMaster, rec.Type)
	assert.False(t, rec.IsDetached)

	// Assert Shallow Check
	isShallow, err := inspector.IsShallow(ctx, h.RepoDir)
	require.NoError(t, err)
	assert.False(t, isShallow)

	// Active operation check
	op, active := inspector.ActiveOperation(ctx, h.RepoDir)
	assert.False(t, active)
	assert.Empty(t, op)
}
