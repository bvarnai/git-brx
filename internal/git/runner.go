package git

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
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
	DebugFn func(cmd string)
}

// NewExecRunner creates a new ExecRunner.
func NewExecRunner(debugFn func(cmd string)) *ExecRunner {
	return &ExecRunner{
		DebugFn: debugFn,
	}
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
	if r.DebugFn != nil {
		r.DebugFn(fmt.Sprintf("git %s", strings.Join(args, " ")))
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}

	// Guarantee LC_ALL=C and avoid hanging on interactive credential prompts
	env := os.Environ()
	env = append(env, "LC_ALL=C", "GIT_TERMINAL_PROMPT=0")
	if len(extraEnv) > 0 {
		env = append(env, extraEnv...)
	}
	cmd.Env = env

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
	if r.DebugFn != nil {
		r.DebugFn(fmt.Sprintf("git %s", strings.Join(args, " ")))
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}

	env := os.Environ()
	env = append(env, "LC_ALL=C", "GIT_TERMINAL_PROMPT=0")
	if len(extraEnv) > 0 {
		env = append(env, extraEnv...)
	}
	cmd.Env = env

	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = os.Stdin

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
