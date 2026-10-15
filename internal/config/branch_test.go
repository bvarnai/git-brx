package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseBranchTargets(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		assert.Nil(t, ParseBranchTargets(nil))
	})

	t.Run("single string", func(t *testing.T) {
		targets := ParseBranchTargets("issue")
		assert.Equal(t, []string{"issue"}, targets)
	})

	t.Run("comma separated string", func(t *testing.T) {
		targets := ParseBranchTargets("issue, feature, epic")
		assert.Equal(t, []string{"issue", "feature", "epic"}, targets)
	})

	t.Run("string slice", func(t *testing.T) {
		targets := ParseBranchTargets([]string{"issue", "feature"})
		assert.Equal(t, []string{"issue", "feature"}, targets)
	})

	t.Run("any slice", func(t *testing.T) {
		targets := ParseBranchTargets([]any{"issue", "feature"})
		assert.Equal(t, []string{"issue", "feature"}, targets)
	})

	t.Run("case insensitive and deduplicated", func(t *testing.T) {
		targets := ParseBranchTargets([]any{"Issue", "issue", " FEATURE ", "feature"})
		assert.Equal(t, []string{"issue", "feature"}, targets)
	})
}
