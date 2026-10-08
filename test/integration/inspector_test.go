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

	// Assert Work Tree
	root, err := inspector.AssertWorkTree(ctx, h.RepoDir)
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

	// In-flight operation check
	rebase, merge := inspector.InFlightOperations(ctx, h.RepoDir)
	assert.False(t, rebase)
	assert.False(t, merge)
}
