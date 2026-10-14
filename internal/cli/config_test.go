package cli_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/bvarnai/git-brx/internal/cli"
	"github.com/bvarnai/git-brx/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestConfigCommand_UnitTable(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".git-brx.yaml")
	cfgContent := `
platform: github
branch:
  template: ^(issue|feature)/[0-9]+$
`
	require.NoError(t, os.WriteFile(configPath, []byte(cfgContent), 0644))

	tests := []struct {
		name          string
		args          []string
		workDir       string
		mockResponses map[string]mockGitResponse
		expectedCode  int
		errSubstr     string
		validateOut   func(t *testing.T, stdout string)
	}{
		{
			name:         "unexpected arguments returns usage error",
			args:         []string{"config", "extra"},
			expectedCode: int(domain.ExitUsageError),
			errSubstr:    "Unexpected argument: extra",
		},
		{
			name:    "zero-config inside repo with remote origin auto-discovers and outputs annotated YAML",
			args:    []string{"config"},
			workDir: t.TempDir(),
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {out: "true"},
				"rev-parse --show-toplevel":       {out: tempDir},
				"remote get-url origin":           {out: "git@github.com:my-org/my-service.git"},
			},
			expectedCode: int(domain.ExitSuccess),
			validateOut: func(t *testing.T, stdout string) {
				assert.Contains(t, stdout, "# Source hierarchy:")
				assert.Contains(t, stdout, "platform: github")
				// YAML round trip
				var cfg domain.ProjectConfig
				err := yaml.Unmarshal([]byte(stdout), &cfg)
				require.NoError(t, err)
				assert.Equal(t, "github", cfg.Platform)
			},
		},
		{
			name:    "sources flag prints only hierarchy comments",
			args:    []string{"config", "--sources"},
			workDir: tempDir,
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {out: "true"},
				"rev-parse --show-toplevel":       {out: tempDir},
				"remote get-url origin":           {out: "git@github.com:my-org/my-service.git"},
			},
			expectedCode: int(domain.ExitSuccess),
			validateOut: func(t *testing.T, stdout string) {
				assert.Contains(t, stdout, "# Source hierarchy:")
				assert.Contains(t, stdout, configPath)
				assert.NotContains(t, stdout, "platform: github")
			},
		},
		{
			name:    "outside repo with no configs returns ExitConfigError",
			args:    []string{"config"},
			workDir: t.TempDir(),
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {out: "false"},
			},
			expectedCode: int(domain.ExitConfigError),
			errSubstr:    "No configuration file found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runner := &mockGitRunner{responses: tc.mockResponses}
			var stdout, stderr bytes.Buffer
			app := cli.NewApp(&stdout, &stderr, runner)
			if tc.workDir != "" {
				app.WorkingDir = tc.workDir
			}
			app.RootCmd.SetArgs(tc.args)

			err := app.Execute(context.Background())

			if tc.expectedCode == 0 {
				require.NoError(t, err, "stderr: %s", stderr.String())
				if tc.validateOut != nil {
					tc.validateOut(t, stdout.String())
				}
			} else {
				require.Error(t, err)
				var appErr *domain.AppError
				require.True(t, errors.As(err, &appErr), "expected domain.AppError, got: %v", err)
				assert.Equal(t, tc.expectedCode, int(appErr.Code))
				if tc.errSubstr != "" {
					assert.Contains(t, appErr.Message, tc.errSubstr)
				}
			}
		})
	}
}
