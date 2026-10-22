// Package errcode defines sentinel error categories, structured application
// errors, and deterministic exit code mapping for git-brx CLI commands.
package errcode

import (
	"context"
	"errors"
	"fmt"
)

// Standard exit codes per docs/spec/global.md Section 3.
const (
	ExitSuccess            = 0   // Command completed cleanly
	ExitGeneralError       = 1   // Unhandled runtime error
	ExitRuntime            = 1   // Alias for ExitGeneralError
	ExitUsageError         = 2   // Command-line syntax or flag error
	ExitUsage              = 2   // Alias for ExitUsageError
	ExitPreconditionRepo   = 3   // Git repository precondition failure
	ExitPreconditionRemote = 4   // Remote/network precondition failure
	ExitConflict           = 5   // Git state or merge conflict encountered
	ExitAPIError           = 6   // External service/API failure
	ExitUserAborted        = 7   // User aborted or canceled confirmation
	ExitConfigError        = 8   // Configuration missing or schema validation error
	ExitConfig             = 8   // Alias for ExitConfigError
	ExitInterrupted        = 130 // Process interrupted (SIGINT / SIGTERM)
)

// Sentinel error categories mapped to standard exit codes.
var (
	// ErrGeneral (and alias ErrRuntime) indicates an unhandled runtime or I/O error (exit code 1).
	ErrGeneral = errors.New("general runtime error")
	ErrRuntime = ErrGeneral

	// ErrUsage indicates a command-line syntax, flag, or argument validation error (exit code 2).
	ErrUsage = errors.New("usage error")

	// ErrPreconditionRepo indicates a Git repository precondition failure (exit code 3).
	ErrPreconditionRepo = errors.New("repository precondition failure")

	// ErrNotRepository indicates invocation outside a Git working tree (exit code 3).
	ErrNotRepository = errors.New("not a git repository")

	// ErrDetachedHead indicates a detached HEAD state when an attached branch is required (exit code 3).
	ErrDetachedHead = errors.New("detached HEAD state")

	// ErrShallowRepo indicates a shallow repository where full history is required (exit code 3).
	ErrShallowRepo = errors.New("shallow repository")

	// ErrPreconditionRemote indicates a remote or network precondition failure (exit code 4).
	ErrPreconditionRemote = errors.New("remote precondition failure")

	// ErrTransport indicates a transport/network error or missing host binary (exit code 4).
	ErrTransport = errors.New("transport error")

	// ErrConflict indicates an active Git operation or merge/rebase conflict (exit code 5).
	ErrConflict = errors.New("git conflict or active operation")

	// ErrGitExec indicates a Git subprocess execution failure (exit code 5).
	ErrGitExec = errors.New("git execution error")

	// ErrAPI indicates an external issue tracker or SCM API failure (exit code 6).
	ErrAPI = errors.New("external API error")

	// ErrUserAborted indicates the user declined an interactive confirmation prompt (exit code 7).
	ErrUserAborted = errors.New("user aborted operation")

	// ErrConfig indicates a configuration file, schema, or branch template validation error (exit code 8).
	ErrConfig = errors.New("configuration error")

	// ErrInterrupted indicates process interruption via SIGINT or SIGTERM (exit code 130).
	ErrInterrupted = errors.New("operation interrupted")
)

// AppError is a structured application error associated with a sentinel category,
// a clean user-facing message, an optional underlying cause, and an optional hint.
type AppError struct {
	Category error
	Message  string
	Hint     string
	Err      error
}

// Error returns the user-facing message (and underlying cause if present)
// without polluting the output with the internal sentinel category name.
func (e *AppError) Error() string {
	if e.Err != nil {
		if e.Message == "" {
			return e.Err.Error()
		}
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Category != nil {
		return e.Category.Error()
	}
	return ""
}

// Unwrap returns both the sentinel Category and the underlying Err so that
// errors.Is and errors.As traverse both chains.
func (e *AppError) Unwrap() []error {
	var errs []error
	if e.Category != nil {
		errs = append(errs, e.Category)
	}
	if e.Err != nil {
		errs = append(errs, e.Err)
	}
	return errs
}

// WithHint sets an actionable remediation hint on the AppError and returns it for chaining.
func (e *AppError) WithHint(format string, args ...any) *AppError {
	if len(args) == 0 {
		e.Hint = format
	} else {
		e.Hint = fmt.Sprintf(format, args...)
	}
	return e
}

// NewError creates a new AppError with a sentinel category and formatted user message.
func NewError(category error, format string, args ...any) *AppError {
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	return &AppError{
		Category: category,
		Message:  msg,
	}
}

// WrapError wraps an existing error with a sentinel category and formatted context message.
func WrapError(category error, err error, format string, args ...any) *AppError {
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	return &AppError{
		Category: category,
		Message:  msg,
		Err:      err,
	}
}

// Hint extracts the remediation hint from err if it wraps an *AppError, or returns "".
func Hint(err error) string {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Hint
	}
	return ""
}

// ExitCode maps any error to its canonical CLI exit code per docs/spec/global.md Section 3.
func ExitCode(err error) int {
	if err == nil {
		return ExitSuccess
	}
	switch {
	case errors.Is(err, ErrInterrupted), errors.Is(err, context.Canceled):
		return ExitInterrupted
	case errors.Is(err, ErrUsage):
		return ExitUsageError
	case errors.Is(err, ErrPreconditionRepo),
		errors.Is(err, ErrNotRepository),
		errors.Is(err, ErrDetachedHead),
		errors.Is(err, ErrShallowRepo):
		return ExitPreconditionRepo
	case errors.Is(err, ErrPreconditionRemote),
		errors.Is(err, ErrTransport):
		return ExitPreconditionRemote
	case errors.Is(err, ErrConflict),
		errors.Is(err, ErrGitExec):
		return ExitConflict
	case errors.Is(err, ErrAPI):
		return ExitAPIError
	case errors.Is(err, ErrUserAborted):
		return ExitUserAborted
	case errors.Is(err, ErrConfig):
		return ExitConfigError
	case errors.Is(err, ErrGeneral):
		return ExitGeneralError
	default:
		return ExitGeneralError
	}
}
