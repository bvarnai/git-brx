package config_test

import (
	"testing"

	"github.com/bvarnai/git-brx/internal/config"
	"github.com/bvarnai/git-brx/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveMergeStrategy_BuiltInConventions(t *testing.T) {
	tests := []struct {
		name         string
		source       string
		target       string
		wantStrategy string
	}{
		{
			name:         "feature into main defaults to squash",
			source:       "feature/101",
			target:       "main",
			wantStrategy: "squash",
		},
		{
			name:         "issue into master defaults to squash",
			source:       "issue/101",
			target:       "master",
			wantStrategy: "squash",
		},
		{
			name:         "epic into main defaults to merge",
			source:       "epic/1",
			target:       "main",
			wantStrategy: "merge",
		},
		{
			name:         "release into main defaults to merge",
			source:       "release/1.0",
			target:       "main",
			wantStrategy: "merge",
		},
		{
			name:         "feature into epic defaults to merge",
			source:       "feature/101",
			target:       "epic/1",
			wantStrategy: "merge",
		},
		{
			name:         "issue into release defaults to merge",
			source:       "issue/101",
			target:       "release/1.0",
			wantStrategy: "merge",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := config.ResolveMergeStrategy(nil, tt.source, tt.target, "")
			require.NoError(t, err)
			assert.Equal(t, tt.wantStrategy, got)
		})
	}
}

func TestResolveMergeStrategy_ConfigurationOverrides(t *testing.T) {
	t.Run("custom target strategy takes precedence for epic target", func(t *testing.T) {
		cfg := &domain.MergeConfig{
			Strategies: domain.MergeStrategies{
				Targets: map[string]string{
					"epic/*": "rebase",
				},
			},
		}
		got, err := config.ResolveMergeStrategy(cfg, "feature/1", "epic/1", "")
		require.NoError(t, err)
		assert.Equal(t, "rebase", got)
	})

	t.Run("custom source strategy takes precedence", func(t *testing.T) {
		cfg := &domain.MergeConfig{
			Strategies: domain.MergeStrategies{
				Sources: map[string]string{
					"feature/*": "merge",
				},
			},
		}
		got, err := config.ResolveMergeStrategy(cfg, "feature/1", "main", "")
		require.NoError(t, err)
		assert.Equal(t, "merge", got)
	})

	t.Run("custom default strategy overrides global squash default", func(t *testing.T) {
		cfg := &domain.MergeConfig{
			Strategy: "rebase",
		}
		got, err := config.ResolveMergeStrategy(cfg, "feature/1", "main", "")
		require.NoError(t, err)
		assert.Equal(t, "rebase", got)
	})

	t.Run("invalid config strategy returns config error", func(t *testing.T) {
		cfg := &domain.MergeConfig{
			Strategy: "fast-forward-only",
		}
		_, err := config.ResolveMergeStrategy(cfg, "feature/1", "main", "")
		require.Error(t, err)
		var appErr *domain.AppError
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, domain.ExitConfigError, appErr.Code)
	})
}

func TestResolveMergeStrategy_CLIOverrides(t *testing.T) {
	cfg := &domain.MergeConfig{
		Strategy: "squash",
		Strategies: domain.MergeStrategies{
			Sources: map[string]string{"epic/*": "merge"},
		},
	}

	t.Run("CLI override to rebase overrides epic convention", func(t *testing.T) {
		got, err := config.ResolveMergeStrategy(cfg, "epic/1", "main", "rebase")
		require.NoError(t, err)
		assert.Equal(t, "rebase", got)
	})

	t.Run("CLI override to squash overrides epic convention", func(t *testing.T) {
		got, err := config.ResolveMergeStrategy(cfg, "epic/1", "main", "squash")
		require.NoError(t, err)
		assert.Equal(t, "squash", got)
	})

	t.Run("invalid CLI strategy returns usage error", func(t *testing.T) {
		_, err := config.ResolveMergeStrategy(cfg, "feature/1", "main", "invalid-strat")
		require.Error(t, err)
		var appErr *domain.AppError
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, domain.ExitUsageError, appErr.Code)
	})
}
