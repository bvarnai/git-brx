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

// AssertWorkTree ensures the working directory is inside a valid Git work tree.
func (i *Inspector) AssertWorkTree(ctx context.Context, dir string) (string, error) {
	out, err := i.runner.Run(ctx, dir, "rev-parse", "--is-inside-work-tree")
	if err != nil || out != "true" {
		return "", domain.NewError(domain.ExitPreconditionRepo, "Awh! This is not a git repository")
	}

	root, err := i.runner.Run(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", domain.WrapError(domain.ExitPreconditionRepo, err, "unable to determine repository root")
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

// InFlightOperations checks if rebase or merge is currently in progress.
func (i *Inspector) InFlightOperations(ctx context.Context, rootDir string) (rebaseActive bool, mergeActive bool) {
	gitDir, err := i.runner.Run(ctx, rootDir, "rev-parse", "--git-dir")
	if err != nil {
		gitDir = filepath.Join(rootDir, ".git")
	} else if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(rootDir, gitDir)
	}

	_, rebaseMergeErr := os.Stat(filepath.Join(gitDir, "rebase-merge"))
	_, rebaseApplyErr := os.Stat(filepath.Join(gitDir, "rebase-apply"))
	rebaseActive = rebaseMergeErr == nil || rebaseApplyErr == nil

	_, mergeErr := os.Stat(filepath.Join(gitDir, "MERGE_HEAD"))
	mergeActive = mergeErr == nil

	return rebaseActive, mergeActive
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
