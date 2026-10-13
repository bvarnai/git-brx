package config

import (
	"testing"

	"github.com/bvarnai/git-brx/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestMatchPathPattern(t *testing.T) {
	tests := []struct {
		pattern  string
		filePath string
		expected bool
	}{
		// Exact matches
		{"internal/cli/review.go", "internal/cli/review.go", true},
		{"internal/cli/review.go", "internal/cli/create.go", false},

		// Recursive wildcards (**)
		{"internal/ui/**", "internal/ui/button.go", true},
		{"internal/ui/**", "internal/ui/components/modal.go", true},
		{"internal/ui/**", "internal/git/runner.go", false},
		{"**/*.md", "README.md", true},
		{"**/*.md", "docs/spec/review.md", true},
		{"**/*.md", "docs/spec/review.go", false},

		// Single wildcard (*)
		{"docs/*", "docs/readme.md", true},
		{"docs/*", "docs/sub/readme.md", false},
		{"internal/*/*.go", "internal/ui/button.go", true},
		{"internal/*/*.go", "internal/ui/comp/button.go", false},

		// Empty/blank inputs
		{"", "internal/cli/review.go", false},
		{"internal/**", "", false},
		{"**", "any/file.go", true},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.filePath, func(t *testing.T) {
			actual := MatchPathPattern(tt.pattern, tt.filePath)
			assert.Equal(t, tt.expected, actual, "pattern: %s, file: %s", tt.pattern, tt.filePath)
		})
	}
}

func TestParseReviewers(t *testing.T) {
	t.Run("single string", func(t *testing.T) {
		res := ParseReviewers("alice")
		assert.Equal(t, []string{"alice"}, res)
	})

	t.Run("comma-separated string", func(t *testing.T) {
		res := ParseReviewers("alice, bob,  charlie")
		assert.Equal(t, []string{"alice", "bob", "charlie"}, res)
	})

	t.Run("string slice", func(t *testing.T) {
		res := ParseReviewers([]string{"alice", "bob"})
		assert.Equal(t, []string{"alice", "bob"}, res)
	})

	t.Run("any slice", func(t *testing.T) {
		res := ParseReviewers([]any{"alice", "bob"})
		assert.Equal(t, []string{"alice", "bob"}, res)
	})

	t.Run("deduplication case-insensitive", func(t *testing.T) {
		res := ParseReviewers([]string{"alice", "Alice", "bob"})
		assert.Equal(t, []string{"alice", "bob"}, res)
	})

	t.Run("nil value", func(t *testing.T) {
		res := ParseReviewers(nil)
		assert.Nil(t, res)
	})
}

func TestFilterAuthor(t *testing.T) {
	reviewers := []string{"Alice", "Bob", "Charlie"}

	t.Run("excludes author case-insensitively", func(t *testing.T) {
		res := FilterAuthor(reviewers, "bob")
		assert.Equal(t, []string{"Alice", "Charlie"}, res)
	})

	t.Run("empty author leaves list intact", func(t *testing.T) {
		res := FilterAuthor(reviewers, "")
		assert.Equal(t, reviewers, res)
	})

	t.Run("author not in list", func(t *testing.T) {
		res := FilterAuthor(reviewers, "dave")
		assert.Equal(t, reviewers, res)
	})

	t.Run("all reviewers filtered out", func(t *testing.T) {
		res := FilterAuthor([]string{"bob"}, "Bob")
		assert.Empty(t, res)
	})
}

func TestResolveReviewers(t *testing.T) {
	mapping := &domain.ReviewMappingConfig{
		Paths: map[string]any{
			"internal/ui/**":  []string{"alice", "carol"},
			"internal/git/**": "bob",
			"docs/**":         "tech-writer",
		},
		Labels: map[string]any{
			"security": []string{"charlie", "dave"},
			"database": "dave",
		},
		Branches: map[string]any{
			"epic/*":    "lead-architect",
			"release/*": "release-manager",
		},
		Default: "team-lead",
	}

	t.Run("tier 1 - matches path rules and aggregates unique reviewers", func(t *testing.T) {
		opts := domain.ReviewerResolutionOptions{
			ChangedFiles: []string{"internal/ui/button.go", "internal/git/runner.go"},
			Labels:       []string{"security"},
			Branch:       "feature/12",
			Author:       "someone-else",
		}
		reviewers := ResolveReviewers(mapping, opts)
		assert.ElementsMatch(t, []string{"alice", "carol", "bob"}, reviewers)
	})

	t.Run("tier 1 - filters author from matched paths", func(t *testing.T) {
		opts := domain.ReviewerResolutionOptions{
			ChangedFiles: []string{"internal/ui/button.go"},
			Author:       "alice",
		}
		reviewers := ResolveReviewers(mapping, opts)
		assert.Equal(t, []string{"carol"}, reviewers)
	})

	t.Run("tier 1 - author is only matched path reviewer cascades to default", func(t *testing.T) {
		opts := domain.ReviewerResolutionOptions{
			ChangedFiles: []string{"docs/readme.md"},
			Author:       "tech-writer",
		}
		reviewers := ResolveReviewers(mapping, opts)
		assert.Equal(t, []string{"team-lead"}, reviewers)
	})

	t.Run("tier 2 - fallback to labels when no paths match", func(t *testing.T) {
		opts := domain.ReviewerResolutionOptions{
			ChangedFiles: []string{"cmd/git-brx/main.go"},
			Labels:       []string{"security"},
			Branch:       "feature/12",
			Author:       "someone-else",
		}
		reviewers := ResolveReviewers(mapping, opts)
		assert.ElementsMatch(t, []string{"charlie", "dave"}, reviewers)
	})

	t.Run("tier 2 - filters author from matched labels", func(t *testing.T) {
		opts := domain.ReviewerResolutionOptions{
			ChangedFiles: []string{"cmd/git-brx/main.go"},
			Labels:       []string{"security"},
			Author:       "charlie",
		}
		reviewers := ResolveReviewers(mapping, opts)
		assert.Equal(t, []string{"dave"}, reviewers)
	})

	t.Run("tier 3 - fallback to branch when no paths or labels match", func(t *testing.T) {
		opts := domain.ReviewerResolutionOptions{
			ChangedFiles: []string{"cmd/git-brx/main.go"},
			Labels:       []string{"unmapped-label"},
			Branch:       "epic/big-project",
			Author:       "someone-else",
		}
		reviewers := ResolveReviewers(mapping, opts)
		assert.Equal(t, []string{"lead-architect"}, reviewers)
	})

	t.Run("tier 4 - fallback to default when nothing matches", func(t *testing.T) {
		opts := domain.ReviewerResolutionOptions{
			ChangedFiles: []string{"cmd/git-brx/main.go"},
			Labels:       []string{"unmapped"},
			Branch:       "feature/unmapped",
			Author:       "someone-else",
		}
		reviewers := ResolveReviewers(mapping, opts)
		assert.Equal(t, []string{"team-lead"}, reviewers)
	})

	t.Run("tier 4 - author is default reviewer yields empty list", func(t *testing.T) {
		opts := domain.ReviewerResolutionOptions{
			ChangedFiles: []string{"cmd/git-brx/main.go"},
			Labels:       []string{"unmapped"},
			Branch:       "feature/unmapped",
			Author:       "team-lead",
		}
		reviewers := ResolveReviewers(mapping, opts)
		assert.Empty(t, reviewers)
	})

	t.Run("nil mapping returns nil", func(t *testing.T) {
		reviewers := ResolveReviewers(nil, domain.ReviewerResolutionOptions{})
		assert.Nil(t, reviewers)
	})
}
