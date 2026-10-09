package git

import (
	"context"
)

// Operations provides safe Git mutating methods.
type Operations struct {
	runner Runner
}

// NewOperations creates a new Operations instance.
func NewOperations(runner Runner) *Operations {
	return &Operations{runner: runner}
}

// Fetch synchronizes references from the specified remote.
func (o *Operations) Fetch(ctx context.Context, dir, remote string) error {
	_, err := o.runner.Run(ctx, dir, "fetch", remote)
	return err
}

// FetchRef fetches a specific reference from the remote.
func (o *Operations) FetchRef(ctx context.Context, dir, remote, ref string) error {
	_, err := o.runner.Run(ctx, dir, "fetch", remote, ref)
	return err
}

// Checkout switches the working tree to the specified branch.
func (o *Operations) Checkout(ctx context.Context, dir, branch string) error {
	_, err := o.runner.Run(ctx, dir, "checkout", branch)
	return err
}

// Rebase runs git rebase against the specified upstream ref.
func (o *Operations) Rebase(ctx context.Context, dir, upstream string, autostash, interactive bool) error {
	args := []string{"rebase"}
	if autostash {
		args = append(args, "--autostash")
	}
	if interactive {
		args = append(args, "--interactive")
	}
	args = append(args, upstream)
	_, err := o.runner.Run(ctx, dir, args...)
	return err
}

// Merge runs git merge with a commit message merging upstream into current branch.
func (o *Operations) Merge(ctx context.Context, dir, upstream, message string) error {
	args := []string{"merge"}
	if message != "" {
		args = append(args, "-m", message)
	}
	args = append(args, upstream)
	_, err := o.runner.Run(ctx, dir, args...)
	return err
}

// AbortMerge runs git merge --abort.
func (o *Operations) AbortMerge(ctx context.Context, dir string) error {
	_, err := o.runner.Run(ctx, dir, "merge", "--abort")
	return err
}

// AbortRebase runs git rebase --abort.
func (o *Operations) AbortRebase(ctx context.Context, dir string) error {
	_, err := o.runner.Run(ctx, dir, "rebase", "--abort")
	return err
}

// ResetHard runs git reset --hard against the specified reference.
func (o *Operations) ResetHard(ctx context.Context, dir, targetRef string) error {
	_, err := o.runner.Run(ctx, dir, "reset", "--hard", targetRef)
	return err
}

// Clean removes untracked files and directories via git clean -fd.
func (o *Operations) Clean(ctx context.Context, dir string) error {
	_, err := o.runner.Run(ctx, dir, "clean", "-fd")
	return err
}
