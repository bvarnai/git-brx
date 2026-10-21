package git

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// Runner represents the direct Git execution boundary interface.
type Runner interface {
	// Run executes a git command and returns combined stdout, or an error.
	Run(ctx context.Context, dir string, args ...string) (string, error)
	// RunLines executes a git command and returns stdout split into trimmed lines.
	RunLines(ctx context.Context, dir string, args ...string) ([]string, error)
	// RunWithEnv executes a git command with additional environment variables.
	RunWithEnv(ctx context.Context, dir string, extraEnv []string, args ...string) (string, error)
	// RunStream executes a git command streaming stdout and stderr to the provided writers.
	RunStream(ctx context.Context, dir string, stdout, stderr io.Writer, extraEnv []string, args ...string) error
}

// ExecRunner implements Runner using os/exec with LC_ALL=C and non-interactive safeguards.
type ExecRunner struct {
	BaseEnv []string
}

// NewExecRunner creates a new ExecRunner.
func NewExecRunner() *ExecRunner {
	return &ExecRunner{}
}

// Run executes a git command and returns its trimmed stdout.
func (r *ExecRunner) Run(ctx context.Context, dir string, args ...string) (string, error) {
	return r.RunWithEnv(ctx, dir, nil, args...)
}

// RunLines executes a git command and returns stdout split by newline.
func (r *ExecRunner) RunLines(ctx context.Context, dir string, args ...string) ([]string, error) {
	out, err := r.Run(ctx, dir, args...)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return []string{}, nil
	}
	return strings.Split(out, "\n"), nil
}

// RunWithEnv executes a git command with additional environment overrides.
func (r *ExecRunner) RunWithEnv(ctx context.Context, dir string, extraEnv []string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = BuildSanitizedEnv(r.BaseEnv, extraEnv)
	cmd.WaitDelay = 3 * time.Second
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			return cmd.Process.Kill()
		}
		return nil
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	outStr := strings.TrimSpace(stdout.String())
	errStr := strings.TrimSpace(stderr.String())

	if err != nil {
		if errStr != "" {
			return outStr, fmt.Errorf("git %s failed: %w: %s", args[0], err, errStr)
		}
		return outStr, fmt.Errorf("git %s failed: %w", args[0], err)
	}

	return outStr, nil
}

// RunStream executes a git command streaming stdout and stderr to the provided writers.
func (r *ExecRunner) RunStream(ctx context.Context, dir string, stdout, stderr io.Writer, extraEnv []string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = BuildSanitizedEnv(r.BaseEnv, extraEnv)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = os.Stdin
	cmd.WaitDelay = 3 * time.Second
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			return cmd.Process.Kill()
		}
		return nil
	}

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Exit code 141 indicates SIGPIPE, typically when a pager exits early (e.g. 'q' in less).
			if exitErr.ExitCode() == 141 {
				return nil
			}
		}
		return fmt.Errorf("git %s failed: %w", args[0], err)
	}

	return nil
}

// BuildSanitizedEnv constructs a child process environment from baseEnv (or os.Environ()
// if baseEnv is nil), appends extraEnv, and enforces mandatory overrides:
// LC_ALL=C, LANG=C, GIT_TERMINAL_PROMPT=0, GIT_ADVICE=0, and gc.auto=0.
func BuildSanitizedEnv(baseEnv []string, extraEnv ...[]string) []string {
	if baseEnv == nil {
		baseEnv = os.Environ()
	}
	combined := make([]string, 0, len(baseEnv)+8)
	combined = append(combined, baseEnv...)
	for _, extra := range extraEnv {
		combined = append(combined, extra...)
	}

	filtered := make([]string, 0, len(combined)+5)
	for _, entry := range combined {
		k, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		switch k {
		case "LC_ALL", "LANG", "GIT_TERMINAL_PROMPT", "GIT_ADVICE", "GIT_CONFIG_COUNT", "GIT_CONFIG_KEY_0", "GIT_CONFIG_VALUE_0":
			continue
		default:
			filtered = append(filtered, entry)
		}
	}
	return append(filtered,
		"LC_ALL=C",
		"LANG=C",
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ADVICE=0",
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=gc.auto",
		"GIT_CONFIG_VALUE_0=0",
	)
}
