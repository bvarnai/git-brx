package cli

import (
	"context"
	"strings"
)

// resolvedBase captures a target base branch name and the Git ref to diff/sync against.
type resolvedBase struct {
	Name        string
	Ref         string
	FromConfig  bool
	FromDefault bool
}

// resolveBaseBranch determines the base branch for the current branch in strict order:
// 1. explicitBase (CLI positional/flag argument)
// 2. recorded branch.<currentBranch>.brx-base (if it still exists locally or on origin)
// 3. repository default branch (main, master, or origin/HEAD)
func (a *App) resolveBaseBranch(ctx context.Context, rootDir, currentBranch, explicitBase string) resolvedBase {
	if b := strings.TrimSpace(explicitBase); b != "" {
		return resolvedBase{
			Name: b,
			Ref:  a.resolveEffectiveRef(ctx, rootDir, b),
		}
	}

	if currentBranch != "" {
		if recorded := a.Inspector.BranchBase(ctx, rootDir, currentBranch); recorded != "" && recorded != currentBranch {
			localExists, remoteExists, _ := a.Inspector.BranchExists(ctx, rootDir, recorded)
			if localExists || remoteExists {
				ref := recorded
				if !localExists && remoteExists {
					ref = "origin/" + recorded
				}
				return resolvedBase{
					Name:       recorded,
					Ref:        ref,
					FromConfig: true,
				}
			}
			a.Logger.Warn("Recorded base branch '%s' no longer exists; falling back to default branch", recorded)
		}
	}

	def := a.Inspector.DefaultBranch(ctx, rootDir)
	if def == "" {
		def = "main"
	}
	return resolvedBase{
		Name:        def,
		Ref:         a.resolveEffectiveRef(ctx, rootDir, def),
		FromDefault: true,
	}
}

func (a *App) resolveEffectiveRef(ctx context.Context, rootDir, branch string) string {
	localExists, remoteExists, _ := a.Inspector.BranchExists(ctx, rootDir, branch)
	if !localExists && remoteExists {
		return "origin/" + branch
	}
	return branch
}
