package errcode_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/bvarnai/git-brx/internal/errcode"
	"github.com/stretchr/testify/assert"
)

func TestExitCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		wantCode int
	}{
		{
			name:     "nil error maps to 0",
			err:      nil,
			wantCode: errcode.ExitSuccess,
		},
		{
			name:     "ErrGeneral maps to 1",
			err:      errcode.NewError(errcode.ErrGeneral, "io failure"),
			wantCode: errcode.ExitGeneralError,
		},
		{
			name:     "ErrRuntime alias maps to 1",
			err:      errcode.NewError(errcode.ErrRuntime, "runtime failure"),
			wantCode: errcode.ExitRuntime,
		},
		{
			name:     "unknown error defaults to 1",
			err:      errors.New("unclassified error"),
			wantCode: errcode.ExitGeneralError,
		},
		{
			name:     "ErrUsage maps to 2",
			err:      errcode.NewError(errcode.ErrUsage, "unexpected arg %q", "foo"),
			wantCode: errcode.ExitUsageError,
		},
		{
			name:     "ErrPreconditionRepo maps to 3",
			err:      errcode.NewError(errcode.ErrPreconditionRepo, "branch not found"),
			wantCode: errcode.ExitPreconditionRepo,
		},
		{
			name:     "ErrNotRepository maps to 3",
			err:      errcode.NewError(errcode.ErrNotRepository, "Not a git repository"),
			wantCode: errcode.ExitPreconditionRepo,
		},
		{
			name:     "ErrDetachedHead maps to 3",
			err:      errcode.NewError(errcode.ErrDetachedHead, "Cannot execute on a detached HEAD"),
			wantCode: errcode.ExitPreconditionRepo,
		},
		{
			name:     "ErrShallowRepo maps to 3",
			err:      errcode.NewError(errcode.ErrShallowRepo, "You are in a shallow repository"),
			wantCode: errcode.ExitPreconditionRepo,
		},
		{
			name:     "ErrPreconditionRemote maps to 4",
			err:      errcode.NewError(errcode.ErrPreconditionRemote, "Unable to reach remote"),
			wantCode: errcode.ExitPreconditionRemote,
		},
		{
			name:     "ErrTransport maps to 4",
			err:      errcode.NewError(errcode.ErrTransport, "git binary not found"),
			wantCode: errcode.ExitPreconditionRemote,
		},
		{
			name:     "ErrConflict maps to 5",
			err:      errcode.NewError(errcode.ErrConflict, "merge conflict encountered"),
			wantCode: errcode.ExitConflict,
		},
		{
			name:     "ErrGitExec maps to 5",
			err:      errcode.WrapError(errcode.ErrGitExec, errors.New("exit status 1"), "git rebase failed"),
			wantCode: errcode.ExitConflict,
		},
		{
			name:     "ErrAPI maps to 6",
			err:      errcode.NewError(errcode.ErrAPI, "GitHub API returned 401"),
			wantCode: errcode.ExitAPIError,
		},
		{
			name:     "ErrUserAborted maps to 7",
			err:      errcode.NewError(errcode.ErrUserAborted, "Branch creation canceled"),
			wantCode: errcode.ExitUserAborted,
		},
		{
			name:     "ErrConfig maps to 8",
			err:      errcode.NewError(errcode.ErrConfig, "invalid configuration file"),
			wantCode: errcode.ExitConfigError,
		},
		{
			name:     "ErrInterrupted maps to 130",
			err:      errcode.NewError(errcode.ErrInterrupted, "received SIGINT"),
			wantCode: errcode.ExitInterrupted,
		},
		{
			name:     "context.Canceled maps to 130",
			err:      fmt.Errorf("command aborted: %w", context.Canceled),
			wantCode: errcode.ExitInterrupted,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.wantCode, errcode.ExitCode(tc.err))
		})
	}
}

func TestAppErrorFormattingAndHint(t *testing.T) {
	t.Parallel()

	err := errcode.NewError(errcode.ErrUsage, "Invalid branch name '%s'", "bad..branch")
	assert.Equal(t, "Invalid branch name 'bad..branch'", err.Error())
	assert.True(t, errors.Is(err, errcode.ErrUsage))

	wrapped := errcode.WrapError(errcode.ErrConflict, io.EOF, "Merge conflict detected").
		WithHint("Use 'git-brx resolve' to resolve conflicts")

	assert.True(t, errors.Is(wrapped, errcode.ErrConflict))
	assert.True(t, errors.Is(wrapped, io.EOF))
	assert.Equal(t, "Merge conflict detected: EOF", wrapped.Error())
	assert.Equal(t, "Use 'git-brx resolve' to resolve conflicts", errcode.Hint(wrapped))
	assert.Equal(t, "", errcode.Hint(errors.New("plain")))

	emptyMsgWrap := errcode.WrapError(errcode.ErrGeneral, io.ErrUnexpectedEOF, "")
	assert.Equal(t, io.ErrUnexpectedEOF.Error(), emptyMsgWrap.Error())
}
