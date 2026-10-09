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

// NotShallow asserts that the repository is not a shallow clone.
func NotShallow(ctx context.Context, inspector *git.Inspector, rootDir string) error {
	shallow, err := inspector.IsShallow(ctx, rootDir)
	if err != nil {
		return domain.WrapError(domain.ExitGeneralError, err, "failed to check shallow repository state")
	}
	if shallow {
		return domain.NewError(domain.ExitPreconditionRepo, "You are in a shallow repository")
	}
	return nil
}

// AttachedBranch ensures the repository is on a named branch and not in a detached HEAD state.
func AttachedBranch(ctx context.Context, inspector *git.Inspector, rootDir string) (*domain.BranchRecord, error) {
	curr, err := inspector.CurrentBranch(ctx, rootDir)
	if err != nil {
		return nil, domain.WrapError(domain.ExitGeneralError, err, "failed to get current branch")
	}
	if curr.IsDetached {
		return nil, domain.NewError(domain.ExitPreconditionRepo, "Cannot execute on a detached HEAD").
			WithHint("Checkout or create a branch first: 'git-brx select <branch>'")
	}
	return curr, nil
}

// HasOrigin ensures that the remote 'origin' is configured.
func HasOrigin(ctx context.Context, inspector *git.Inspector, rootDir string) error {
	hasOrigin, _, err := inspector.HasOrigin(ctx, rootDir)
	if err != nil {
		return domain.WrapError(domain.ExitGeneralError, err, "failed to check remote origin")
	}
	if !hasOrigin {
		return domain.NewError(domain.ExitPreconditionRemote, "Your repository has no remote origin")
	}
	return nil
}

// HasCommits ensures that the repository contains at least one valid commit ref.
func HasCommits(ctx context.Context, inspector *git.Inspector, rootDir string) error {
	if !inspector.HasCommits(ctx, rootDir) {
		return domain.NewError(domain.ExitPreconditionRepo, "Repository has no commits yet")
	}
	return nil
}
