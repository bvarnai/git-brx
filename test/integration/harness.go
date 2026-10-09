package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// Harness manages an isolated temporary Git repository for integration tests.
type Harness struct {
	T       *testing.T
	RepoDir string
}

// NewHarness provisions a disposable Git repository with configured user identity.
func NewHarness(t *testing.T) *Harness {
	t.Helper()
	repoDir := t.TempDir()

	h := &Harness{
		T:       t,
		RepoDir: repoDir,
	}

	h.Git("init", "-b", "master")
	h.Git("config", "user.name", "Test User")
	h.Git("config", "user.email", "test@example.com")
	h.Git("config", "commit.gpgsign", "false")

	return h
}

// Git executes a raw Git command within the harness repo.
func (h *Harness) Git(args ...string) string {
	h.T.Helper()
	cmd := exec.CommandContext(context.Background(), "git", args...)
	cmd.Dir = h.RepoDir
	cmd.Env = append(os.Environ(), "LC_ALL=C", "GIT_TERMINAL_PROMPT=0")

	out, err := cmd.CombinedOutput()
	require.NoError(h.T, err, "git %v failed: %s", args, string(out))
	return string(out)
}

// GitAllowError executes a raw Git command allowing failure and returns output and error.
func (h *Harness) GitAllowError(args ...string) (string, error) {
	h.T.Helper()
	cmd := exec.CommandContext(context.Background(), "git", args...)
	cmd.Dir = h.RepoDir
	cmd.Env = append(os.Environ(), "LC_ALL=C", "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// CommitFile creates or updates a file in the repo and commits it.
func (h *Harness) CommitFile(relPath, content, commitMsg string) {
	h.T.Helper()
	fullPath := filepath.Join(h.RepoDir, relPath)
	err := os.MkdirAll(filepath.Dir(fullPath), 0755)
	require.NoError(h.T, err)

	err = os.WriteFile(fullPath, []byte(content), 0644)
	require.NoError(h.T, err)

	h.Git("add", relPath)
	h.Git("commit", "-m", commitMsg)
}

// SetupRemoteBare provisions a bare repository and configures it as 'origin'.
func (h *Harness) SetupRemoteBare() string {
	h.T.Helper()
	remoteDir := h.T.TempDir()

	cmd := exec.Command("git", "init", "--bare", remoteDir)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.CombinedOutput()
	require.NoError(h.T, err, "git init --bare failed: %s", string(out))

	h.Git("remote", "add", "origin", remoteDir)
	return remoteDir
}

// CreateBranch creates and checks out a new branch.
func (h *Harness) CreateBranch(name string) {
	h.T.Helper()
	h.Git("checkout", "-b", name)
}

// CloneHarness provisions a second clone from a remote URL.
func CloneHarness(t *testing.T, remoteURL string) *Harness {
	t.Helper()
	cloneDir := filepath.Join(t.TempDir(), "clone")

	cmd := exec.Command("git", "clone", remoteURL, cloneDir)
	cmd.Env = append(os.Environ(), "LC_ALL=C", "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git clone failed: %s", string(out))

	h := &Harness{
		T:       t,
		RepoDir: cloneDir,
	}
	h.Git("config", "user.name", "Test User 2")
	h.Git("config", "user.email", "test2@example.com")
	h.Git("config", "commit.gpgsign", "false")
	return h
}
