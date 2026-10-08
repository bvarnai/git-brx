# Engineering Rules & Guidelines for git-brx

## Project Identity & Architecture
- Tool: `git-brx` (Git CLI extension written in Go).
- CLI Framework: `github.com/spf13/cobra`.
- Architecture:
  - `cmd/git-brx/main.go`: Thin entrypoint only.
  - `internal/cli/`: Cobra commands, flag binding, and input validation.
  - `internal/git/`: Centralized Git command execution engine.
  - `internal/domain/`: Pure Go domain models and business logic.
  - `test/integration/`: End-to-end integration tests using isolated git repositories in `t.TempDir()`.

## Coding Standards & Idioms
- Direct Execution Boundary:
  - NEVER call `os/exec.Command` directly in CLI or domain code.
  - ALWAYS invoke Git through the `internal/git.Runner` interface.
  - All Git child processes MUST include `LC_ALL=C` in their environment.
  - Support cancellation: pass `cmd.Context()` into all Git operations.
- Error Handling:
  - Never call `log.Fatal()`, `panic()`, or `os.Exit()` inside `internal/`. Return explicit `error` values up to `main.go`.
  - Use custom domain errors mapped to specific exit codes defined in `docs/spec/global.md`.
- Libraries & Dependencies:
  - Prefer the Go standard library (`os`, `strings`, `bufio`, `regexp`, `context`).
  - Allowed third-party dependencies: `github.com/spf13/cobra`, `github.com/stretchr/testify`.
  - Do NOT import heavyweight Git libraries like `go-git` unless explicitly requested.

### Go Architectural & Style Guidelines

In accordance with the [Google Go Style Guide](https://google.github.io/styleguide/go/guide):

- **Package Layout:**
  - `cmd/git-brx/`: Application entry point and command-line wiring.
  - `internal/git/`: Git plumbing wrapper, repository inspection, and execution boundary.
  - `internal/jira/`: JIRA REST client implementation.
  - `internal/bitbucket/`: Bitbucket Server REST client implementation.
  - `internal/config/`: Configuration loading, validation, and schema definitions.
  - `internal/ui/`: Formatting, terminal detection, color styling, and logging primitives.
- **Error Handling:**
  - Errors must be returned explicitly as the last return value; panics must never be exposed across package boundaries.
  - Wrap errors with context using `fmt.Errorf("action description: %w", err)` to preserve error chains.
  - Map errors to structured application exit codes at the CLI boundary.
- **Context & Concurrency:**
  - Every external execution (Git subprocesses, HTTP calls) must accept a `context.Context` to guarantee prompt cancellation on timeout or user interruption (`SIGINT`).

## Testing Rules
- Every subcommand must include:
  1. Unit tests for pure logic/parsers.
  2. Integration tests creating disposable repos via `test/integration/harness.go`.
- Table-driven tests are mandatory.
- Tests MUST NOT modify or rely on the host machine's `~/.gitconfig` or global Git state.

## Workflow Rules
- Check `docs/spec/` for behavioral requirements before writing code.
- Always implement behavioral parity with the spec before proposing enhancements.
- Keep terminal output clean: use stderr for diagnostics/errors and stdout strictly for command results or plumbing output.
