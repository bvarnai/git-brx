package checks

import (
	"context"

	"github.com/bvarnai/git-brx/internal/domain"
	"github.com/bvarnai/git-brx/internal/git"
)

// InsideWorkTree ensures the directory is inside a valid Git work tree and returns its root path.
func InsideWorkTree(ctx context.Context, inspector *git.Inspector, dir string) (string, error) {
	root, err := inspector.RepoRoot(ctx, dir)
	if err != nil {
		return "", domain.NewError(domain.ExitPreconditionRepo, "Awh! This is not a git repository")
	}
	return root, nil
}

// NoActiveOperation verifies that no rebase, merge, cherry-pick, revert, or bisect is active.
func NoActiveOperation(ctx context.Context, inspector *git.Inspector, rootDir string) error {
	op, active := inspector.ActiveOperation(ctx, rootDir)
	if active {
		hint := "Complete or abort the in-progress operation first"
		if op == "cherry-pick" {
			hint = "Abort with 'git cherry-pick --abort' before switching branches"
		} else if op == "revert" {
			hint = "Abort with 'git revert --abort' before switching branches"
		} else if op == "merge" {
			hint = "Abort with 'git merge --abort' before switching branches"
		} else if op == "rebase" {
			hint = "Abort with 'git rebase --abort' before switching branches"
		}
		return domain.NewError(domain.ExitConflict, "Cannot switch branches during an active %s", op).
			WithHint("%s", hint)
	}
	return nil
}

// NoOrphanedCommits blocks switching away from a detached HEAD if it contains unreferenced commits.
func NoOrphanedCommits(ctx context.Context, inspector *git.Inspector, rootDir string) error {
	orphaned, err := inspector.HasOrphanedCommits(ctx, rootDir)
	if err != nil {
		return domain.WrapError(domain.ExitGeneralError, err, "failed to inspect detached HEAD state")
	}
	if orphaned {
		return domain.NewError(domain.ExitConflict, "You are on a detached HEAD with unreferenced commits").
			WithHint("Save your work into a branch first: 'git branch <branch-name>'")
	}
	return nil
}
