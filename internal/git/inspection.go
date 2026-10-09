package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bvarnai/git-brx/internal/domain"
)

// Inspector provides repository inspection methods using Git plumbing.
type Inspector struct {
	runner Runner
}

// NewInspector creates a new Inspector.
func NewInspector(runner Runner) *Inspector {
	return &Inspector{runner: runner}
}

// IsWorkTree reports whether the directory is inside a Git work tree.
func (i *Inspector) IsWorkTree(ctx context.Context, dir string) bool {
	out, err := i.runner.Run(ctx, dir, "rev-parse", "--is-inside-work-tree")
	return err == nil && out == "true"
}

// RepoRoot returns the top-level directory path of the Git repository.
func (i *Inspector) RepoRoot(ctx context.Context, dir string) (string, error) {
	if !i.IsWorkTree(ctx, dir) {
		return "", fmt.Errorf("not inside a git work tree")
	}
	root, err := i.runner.Run(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("unable to determine repository root: %w", err)
	}
	return root, nil
}

// CurrentBranch returns the name of the currently active branch or an error if detached.
func (i *Inspector) CurrentBranch(ctx context.Context, dir string) (*domain.BranchRecord, error) {
	out, err := i.runner.Run(ctx, dir, "symbolic-ref", "--short", "-q", "HEAD")
	if err != nil {
		// Detached HEAD
		shortSHA, shaErr := i.runner.Run(ctx, dir, "rev-parse", "--short", "HEAD")
		if shaErr != nil {
			shortSHA = "unknown"
		}
		return &domain.BranchRecord{
			Name:       shortSHA,
			Type:       domain.BranchTypeCustom,
			CommitHash: shortSHA,
			IsDetached: true,
		}, nil
	}

	branchName := strings.TrimSpace(out)
	branchType := ClassifyBranch(branchName)
	hash, _ := i.runner.Run(ctx, dir, "rev-parse", "--short", "HEAD")
	upstream, _ := i.runner.Run(ctx, dir, "for-each-ref", "--format=%(upstream:short)", "refs/heads/"+branchName)

	return &domain.BranchRecord{
		Name:        branchName,
		Type:        branchType,
		UpstreamRef: upstream,
		CommitHash:  hash,
		IsDetached:  false,
	}, nil
}

// ClassifyBranch identifies branch type from prefix convention.
func ClassifyBranch(name string) domain.BranchType {
	parts := strings.SplitN(name, "/", 2)
	switch parts[0] {
	case "issue":
		return domain.BranchTypeIssue
	case "feature":
		return domain.BranchTypeFeature
	case "epic":
		return domain.BranchTypeEpic
	case "release":
		return domain.BranchTypeRelease
	case "master", "main":
		return domain.BranchTypeMaster
	default:
		return domain.BranchTypeCustom
	}
}

// IsShallow returns true if the repository is a shallow clone.
func (i *Inspector) IsShallow(ctx context.Context, dir string) (bool, error) {
	out, err := i.runner.Run(ctx, dir, "rev-parse", "--is-shallow-repository")
	if err != nil {
		return false, err
	}
	return out == "true", nil
}

// HasOrigin returns true if remote 'origin' is configured and reachable.
func (i *Inspector) HasOrigin(ctx context.Context, dir string) (bool, string, error) {
	out, err := i.runner.Run(ctx, dir, "remote", "get-url", "origin")
	if err != nil {
		return false, "", nil
	}
	return true, strings.TrimSpace(out), nil
}

// HasCommits returns true if HEAD points to a valid commit.
func (i *Inspector) HasCommits(ctx context.Context, dir string) bool {
	_, err := i.runner.Run(ctx, dir, "rev-parse", "--verify", "-q", "HEAD")
	return err == nil
}

// ActiveOperation checks if rebase, merge, cherry-pick, revert, or bisect is active.
func (i *Inspector) ActiveOperation(ctx context.Context, rootDir string) (op string, active bool) {
	gitDir, err := i.runner.Run(ctx, rootDir, "rev-parse", "--git-dir")
	if err != nil {
		gitDir = filepath.Join(rootDir, ".git")
	} else if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(rootDir, gitDir)
	}

	if _, err := os.Stat(filepath.Join(gitDir, "rebase-merge")); err == nil {
		return "rebase", true
	}
	if _, err := os.Stat(filepath.Join(gitDir, "rebase-apply")); err == nil {
		return "rebase", true
	}
	if _, err := os.Stat(filepath.Join(gitDir, "MERGE_HEAD")); err == nil {
		return "merge", true
	}
	if _, err := os.Stat(filepath.Join(gitDir, "CHERRY_PICK_HEAD")); err == nil {
		return "cherry-pick", true
	}
	if _, err := os.Stat(filepath.Join(gitDir, "REVERT_HEAD")); err == nil {
		return "revert", true
	}
	if _, err := os.Stat(filepath.Join(gitDir, "BISECT_LOG")); err == nil {
		return "bisect", true
	}

	return "", false
}

// RebaseProgress holds step and commit details for an active rebase operation.
type RebaseProgress struct {
	CurrentStep int
	TotalSteps  int
	CommitDesc  string
}

// RebaseProgress returns progress information for an active rebase, if available.
func (i *Inspector) RebaseProgress(ctx context.Context, rootDir string) (*RebaseProgress, error) {
	gitDir, err := i.runner.Run(ctx, rootDir, "rev-parse", "--git-dir")
	if err != nil {
		gitDir = filepath.Join(rootDir, ".git")
	} else if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(rootDir, gitDir)
	}

	rebaseDir := filepath.Join(gitDir, "rebase-merge")
	if _, err := os.Stat(rebaseDir); err != nil {
		rebaseDir = filepath.Join(gitDir, "rebase-apply")
		if _, err := os.Stat(rebaseDir); err != nil {
			return nil, fmt.Errorf("no active rebase directory found")
		}
	}

	cur := 0
	total := 0
	if msgNumBytes, err := os.ReadFile(filepath.Join(rebaseDir, "msgnum")); err == nil {
		_, _ = fmt.Sscanf(strings.TrimSpace(string(msgNumBytes)), "%d", &cur)
	}
	if endBytes, err := os.ReadFile(filepath.Join(rebaseDir, "end")); err == nil {
		_, _ = fmt.Sscanf(strings.TrimSpace(string(endBytes)), "%d", &total)
	}

	desc := ""
	if headNameBytes, err := os.ReadFile(filepath.Join(rebaseDir, "head-name")); err == nil {
		desc = strings.TrimSpace(string(headNameBytes))
	}

	return &RebaseProgress{
		CurrentStep: cur,
		TotalSteps:  total,
		CommitDesc:  desc,
	}, nil
}

// ComputeDelta calculates ahead and behind commit counts between a local ref and its upstream.
func (i *Inspector) ComputeDelta(ctx context.Context, dir string, localRef, remoteRef string) (*domain.BranchDelta, error) {
	out, err := i.runner.Run(ctx, dir, "rev-list", "--left-right", "--count", fmt.Sprintf("%s...%s", localRef, remoteRef))
	if err != nil {
		return nil, domain.WrapError(domain.ExitPreconditionRemote, err, "failed to compute commit delta for %s against %s", localRef, remoteRef)
	}

	fields := strings.Fields(out)
	if len(fields) < 2 {
		return nil, domain.NewError(domain.ExitGeneralError, "unexpected rev-list output: %s", out)
	}

	ahead, _ := strconv.Atoi(fields[0])
	behind, _ := strconv.Atoi(fields[1])

	return &domain.BranchDelta{
		LocalBranch:  localRef,
		RemoteBranch: remoteRef,
		AheadCount:   ahead,
		BehindCount:  behind,
	}, nil
}

// BranchExists checks if a branch exists locally in refs/heads or remotely in refs/remotes/origin.
func (i *Inspector) BranchExists(ctx context.Context, dir, branch string) (localExists, remoteExists bool, err error) {
	_, localErr := i.runner.Run(ctx, dir, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	localExists = localErr == nil

	_, remoteErr := i.runner.Run(ctx, dir, "show-ref", "--verify", "--quiet", "refs/remotes/origin/"+branch)
	remoteExists = remoteErr == nil

	return localExists, remoteExists, nil
}

// HasOrphanedCommits checks if HEAD is detached and contains commits not reachable from any local or remote branch.
func (i *Inspector) HasOrphanedCommits(ctx context.Context, dir string) (bool, error) {
	// If HEAD is attached to a branch, switching away cannot orphan commits.
	_, err := i.runner.Run(ctx, dir, "symbolic-ref", "-q", "HEAD")
	if err == nil {
		return false, nil
	}

	// Detached HEAD: check for commits reachable from HEAD but not from any branch or remote.
	out, err := i.runner.Run(ctx, dir, "rev-list", "-n", "1", "HEAD", "--not", "--branches", "--remotes")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// IsDirty returns true if the working directory or staging index contains uncommitted changes.
func (i *Inspector) IsDirty(ctx context.Context, dir string) (bool, error) {
	out, err := i.runner.Run(ctx, dir, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// DefaultBranch inspects repository references to determine the primary default branch ('master', 'main', or origin HEAD).
func (i *Inspector) DefaultBranch(ctx context.Context, dir string) string {
	localMaster, remoteMaster, _ := i.BranchExists(ctx, dir, "master")
	if localMaster || remoteMaster {
		return "master"
	}

	localMain, remoteMain, _ := i.BranchExists(ctx, dir, "main")
	if localMain || remoteMain {
		return "main"
	}

	out, err := i.runner.Run(ctx, dir, "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
	if err == nil {
		trimmed := strings.TrimSpace(out)
		parts := strings.SplitN(trimmed, "/", 2)
		if len(parts) == 2 && parts[1] != "" {
			return parts[1]
		}
	}

	return "master"
}

// ListAllBranchNames returns all deduplicated local and remote branch names.
func (i *Inspector) ListAllBranchNames(ctx context.Context, dir string) ([]string, error) {
	lines, err := i.runner.RunLines(ctx, dir, "for-each-ref", "--format=%(refname:short)", "refs/heads", "refs/remotes/origin")
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var names []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasSuffix(trimmed, "/HEAD") {
			continue
		}
		// Strip remote prefix if present: "origin/foo" -> "foo"
		name := trimmed
		if strings.HasPrefix(trimmed, "origin/") {
			name = strings.TrimPrefix(trimmed, "origin/")
		}

		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}

	return names, nil
}

// UnmergedFiles returns the list of paths that currently have unmerged conflict stages.
func (i *Inspector) UnmergedFiles(ctx context.Context, dir string) ([]string, error) {
	lines, err := i.runner.RunLines(ctx, dir, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil, fmt.Errorf("failed to query unmerged files: %w", err)
	}

	var unmerged []string
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed != "" {
			unmerged = append(unmerged, trimmed)
		}
	}
	return unmerged, nil
}
