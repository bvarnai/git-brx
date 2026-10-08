package domain

import "fmt"

// ExitCode represents application-wide process exit codes.
type ExitCode int

const (
	ExitSuccess            ExitCode = 0 // Command completed cleanly
	ExitGeneralError       ExitCode = 1 // Unhandled runtime error
	ExitUsageError         ExitCode = 2 // Command-line syntax or flag error
	ExitPreconditionRepo   ExitCode = 3 // Git repository precondition failure
	ExitPreconditionRemote ExitCode = 4 // Remote/network precondition failure
	ExitConflict           ExitCode = 5 // Git state or merge conflict encountered
	ExitAPIError           ExitCode = 6 // External service/API failure (JIRA/Bitbucket)
	ExitUserAborted        ExitCode = 7 // User aborted or canceled confirmation
	ExitConfigError        ExitCode = 8 // Configuration missing or schema validation error
)

// AppError is an error associated with a structured ExitCode.
type AppError struct {
	Code    ExitCode
	Message string
	Hint    string
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// WithHint sets an actionable hint on the error.
func (e *AppError) WithHint(format string, args ...any) *AppError {
	if len(args) == 0 {
		e.Hint = format
	} else {
		e.Hint = fmt.Sprintf(format, args...)
	}
	return e
}

// NewError creates a new AppError with an exit code and formatted message.
func NewError(code ExitCode, format string, args ...any) *AppError {
	return &AppError{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
	}
}

// WrapError wraps an existing error with an exit code and context message.
func WrapError(code ExitCode, err error, format string, args ...any) *AppError {
	return &AppError{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
		Err:     err,
	}
}
