package config

import (
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bvarnai/git-brx/internal/domain"
)

// MatchPathPattern tests whether a relative file path matches a glob pattern.
// Supports:
// - Exact match ("internal/cli/review.go")
// - Recursive wildcards: "**" matches zero or more directories ("internal/**", "**/*.go")
// - Standard wildcards: "*" matches non-separator characters, "?" matches single non-separator.
func MatchPathPattern(pattern, filePath string) bool {
	pattern = filepath.ToSlash(strings.TrimSpace(pattern))
	filePath = filepath.ToSlash(strings.TrimSpace(filePath))
	if pattern == "" || filePath == "" {
		return false
	}
	if pattern == filePath || pattern == "**" {
		return true
	}

	// Trailing /**: e.g. "internal/ui/**"
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "**")
		if strings.HasPrefix(filePath, prefix) || filePath == strings.TrimSuffix(prefix, "/") {
			return true
		}
	}

	// Standard path.Match if no ** is present
	if !strings.Contains(pattern, "**") {
		if matched, err := path.Match(pattern, filePath); err == nil && matched {
			return true
		}
	}

	// Convert glob to regular expression
	regexStr := globToRegex(pattern)
	re, err := regexp.Compile(regexStr)
	if err != nil {
		return false
	}
	return re.MatchString(filePath)
}

func globToRegex(pattern string) string {
	var sb strings.Builder
	sb.WriteString("^")
	i := 0
	n := len(pattern)
	for i < n {
		if i+1 < n && pattern[i:i+2] == "**" {
			if i+2 < n && pattern[i+2] == '/' {
				sb.WriteString("(?:.*/)?")
				i += 3
			} else {
				sb.WriteString(".*")
				i += 2
			}
		} else if pattern[i] == '*' {
			sb.WriteString("[^/]*")
			i++
		} else if pattern[i] == '?' {
			sb.WriteString("[^/]")
			i++
		} else {
			ch := pattern[i]
			if strings.ContainsRune(`.+()|[]{}^$\`, rune(ch)) {
				sb.WriteByte('\\')
			}
			sb.WriteByte(ch)
			i++
		}
	}
	sb.WriteString("$")
	return sb.String()
}

// ParseReviewers parses single strings, comma-separated strings, or lists into reviewer handles.
func ParseReviewers(val any) []string {
	if val == nil {
		return nil
	}
	seen := make(map[string]bool)
	var reviewers []string

	add := func(handle string) {
		trimmed := strings.TrimSpace(handle)
		if trimmed != "" {
			normalized := strings.ToLower(trimmed)
			if !seen[normalized] {
				seen[normalized] = true
				reviewers = append(reviewers, trimmed)
			}
		}
	}

	switch v := val.(type) {
	case string:
		for _, item := range strings.Split(v, ",") {
			add(item)
		}
	case []string:
		for _, item := range v {
			add(item)
		}
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				add(s)
			}
		}
	}
	return reviewers
}

// FilterAuthor removes the PR author from the reviewer list (case-insensitive).
func FilterAuthor(reviewers []string, author string) []string {
	if strings.TrimSpace(author) == "" {
		return reviewers
	}
	authorNorm := strings.ToLower(strings.TrimSpace(author))
	var filtered []string
	for _, r := range reviewers {
		if strings.ToLower(strings.TrimSpace(r)) != authorNorm {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// ResolveReviewers resolves designated reviewers using the 4-tier hierarchy:
// 1. Paths: Matches changed files against mapping.Paths
// 2. Labels: Matches issue labels/components against mapping.Labels
// 3. Branches: Matches branch name against mapping.Branches
// 4. Default: Falls back to mapping.Default
//
// Across all tiers, the author is excluded. If an author-only match occurs,
// resolution cascades to the default tier.
func ResolveReviewers(mapping *domain.ReviewMappingConfig, opts domain.ReviewerResolutionOptions) []string {
	if mapping == nil {
		return nil
	}

	// Tier 1: Changed File Paths
	if len(mapping.Paths) > 0 && len(opts.ChangedFiles) > 0 {
		var pathReviewers []string
		seen := make(map[string]bool)
		for _, file := range opts.ChangedFiles {
			for pattern, val := range mapping.Paths {
				if MatchPathPattern(pattern, file) {
					for _, r := range ParseReviewers(val) {
						norm := strings.ToLower(r)
						if !seen[norm] {
							seen[norm] = true
							pathReviewers = append(pathReviewers, r)
						}
					}
				}
			}
		}

		if len(pathReviewers) > 0 {
			filtered := FilterAuthor(pathReviewers, opts.Author)
			if len(filtered) > 0 {
				return filtered
			}
			// Author was the only matched reviewer; cascade to Default
			return FilterAuthor(ParseReviewers(mapping.Default), opts.Author)
		}
	}

	// Tier 2: Issue Labels / Components
	if len(mapping.Labels) > 0 && len(opts.Labels) > 0 {
		var labelReviewers []string
		seen := make(map[string]bool)
		for _, label := range opts.Labels {
			for pattern, val := range mapping.Labels {
				if strings.EqualFold(strings.TrimSpace(pattern), strings.TrimSpace(label)) {
					for _, r := range ParseReviewers(val) {
						norm := strings.ToLower(r)
						if !seen[norm] {
							seen[norm] = true
							labelReviewers = append(labelReviewers, r)
						}
					}
				}
			}
		}

		if len(labelReviewers) > 0 {
			filtered := FilterAuthor(labelReviewers, opts.Author)
			if len(filtered) > 0 {
				return filtered
			}
			// Author was the only matched reviewer; cascade to Default
			return FilterAuthor(ParseReviewers(mapping.Default), opts.Author)
		}
	}

	// Tier 3: Branch Patterns
	if len(mapping.Branches) > 0 && strings.TrimSpace(opts.Branch) != "" {
		var branchReviewers []string
		seen := make(map[string]bool)
		for pattern, val := range mapping.Branches {
			if MatchBranchPattern(pattern, opts.Branch) {
				for _, r := range ParseReviewers(val) {
					norm := strings.ToLower(r)
					if !seen[norm] {
						seen[norm] = true
						branchReviewers = append(branchReviewers, r)
					}
				}
			}
		}

		if len(branchReviewers) > 0 {
			filtered := FilterAuthor(branchReviewers, opts.Author)
			if len(filtered) > 0 {
				return filtered
			}
			// Author was the only matched reviewer; cascade to Default
			return FilterAuthor(ParseReviewers(mapping.Default), opts.Author)
		}
	}

	// Tier 4: Default Fallback
	if mapping.Default != nil {
		return FilterAuthor(ParseReviewers(mapping.Default), opts.Author)
	}

	return nil
}
