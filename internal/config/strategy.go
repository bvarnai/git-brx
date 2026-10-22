package config

import (
	"path"
	"regexp"
	"strings"

	"github.com/bvarnai/git-brx/internal/domain"
	"github.com/bvarnai/git-brx/internal/errcode"
)

var (
	epicOrReleaseRegex = regexp.MustCompile(`^(epic|release)/`)
)

// MatchBranchPattern checks if a branch name matches a pattern (glob, exact, or prefix).
func MatchBranchPattern(pattern, branch string) bool {
	pattern = strings.TrimSpace(pattern)
	branch = strings.TrimSpace(branch)
	if pattern == "" || branch == "" {
		return false
	}
	if pattern == branch || pattern == "*" {
		return true
	}
	// Prefix match: e.g. "epic/*" matches "epic/123", "epic/sub/branch"
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "*")
		if strings.HasPrefix(branch, prefix) {
			return true
		}
	}
	// Glob match
	matched, err := path.Match(pattern, branch)
	return err == nil && matched
}

// ResolveMergeStrategy determines the appropriate merge strategy for a PR.
// Precedence:
// 1. Explicit CLI override (--strategy)
// 2. Custom Target strategies in config (review.merge.strategies.targets)
// 3. Custom Source strategies in config (review.merge.strategies.sources)
// 4. Built-in convention:
//   - Source is epic/* or release/* -> "merge"
//   - Target is epic/* or release/* -> "merge"
//
// 5. Config default (review.merge.strategy, defaults to "squash")
func ResolveMergeStrategy(mergeCfg *domain.MergeConfig, sourceBranch, targetBranch, cliStrategy string) (string, error) {
	if cliStrategy != "" {
		normalized := strings.ToLower(strings.TrimSpace(cliStrategy))
		switch normalized {
		case "squash", "merge", "rebase":
			return normalized, nil
		default:
			return "", errcode.NewError(errcode.ErrUsage, "Unsupported merge strategy: %s (must be squash, merge, or rebase)", cliStrategy)
		}
	}

	if mergeCfg != nil {
		// 2. Custom Target strategies
		for pattern, strat := range mergeCfg.Strategies.Targets {
			if MatchBranchPattern(pattern, targetBranch) {
				return validateStrategy(strat)
			}
		}

		// 3. Custom Source strategies
		for pattern, strat := range mergeCfg.Strategies.Sources {
			if MatchBranchPattern(pattern, sourceBranch) {
				return validateStrategy(strat)
			}
		}
	}

	// 4. Built-in Smart Conventions:
	// If source is epic/* or release/* -> merge
	if epicOrReleaseRegex.MatchString(sourceBranch) {
		return "merge", nil
	}
	// If target is epic/* or release/* -> merge
	if epicOrReleaseRegex.MatchString(targetBranch) {
		return "merge", nil
	}

	// 5. Default strategy from config
	if mergeCfg != nil && mergeCfg.Strategy != "" {
		return validateStrategy(mergeCfg.Strategy)
	}

	return "squash", nil
}

func validateStrategy(strat string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(strat))
	switch normalized {
	case "squash", "merge", "rebase":
		return normalized, nil
	default:
		return "", errcode.NewError(errcode.ErrConfig, "Unsupported merge strategy in configuration: %s (must be squash, merge, or rebase)", strat)
	}
}
