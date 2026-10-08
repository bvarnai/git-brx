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

// Checkout switches the working tree to the specified branch.
func (o *Operations) Checkout(ctx context.Context, dir, branch string) error {
	_, err := o.runner.Run(ctx, dir, "checkout", branch)
	return err
}
