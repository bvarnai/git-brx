# Engineering Rules & Guidelines for git-brx

## Project Identity & Architecture
- **Tool:** `git-brx` — Git CLI extension written in Go 1.22+ (`git-brx <subcommand>` or `git brx <subcommand>`).
- **CLI Framework:** `github.com/spf13/cobra`.
- **Package Layout:**
  - `cmd/git-brx/main.go`: Thin entrypoint only (signal context `SIGINT`/`SIGTERM`, exit code mapping).
  - `internal/checks/`: Precondition invariant enforcement (`InsideWorkTree`, `NoActiveOperation`, `NoOrphanedCommits`, `NotShallow`, `AttachedBranch`, `HasOrigin`, `HasCommits`).
  - `internal/cli/`: Cobra commands, flag binding, input validation, and user experience orchestration.
  - `internal/config/`: 4-tier cascading YAML config loader (`loader.go`), merger, provenance reporter, and reviewer routing rules.
  - `internal/domain/`: Pure Go value types, domain models (`BranchRecord`, `Issue`, `PullRequestRequest`), and application error definitions. Depends only on Go stdlib.
  - `internal/git/`: Subprocess execution boundary (`Runner`, `ExecRunner`, `BuildSanitizedEnv`), inspection engine (`Inspector`), and operational engine (`Operations`).
  - `internal/github/`: GitHub REST API client (issues, pull requests, reviewers, authentication).
  - `internal/suggest/`: Levenshtein distance string matching for branch typo suggestions.
  - `internal/term/`: Thread-safe `Logger` with mutex protection, ANSI color policy resolution (`GIT_BRX_COLOR`, `NO_COLOR`, `TERM=dumb`), `Confirm` prompt, `Spinner`, and `Timer`.
  - `test/integration/`: End-to-end integration tests using isolated disposable repositories in `t.TempDir()`.
- **Dependency Boundaries:** `internal/config`, `internal/git`, `internal/checks`, `internal/github`, and `internal/suggest` may depend on `domain` and `term`, but NEVER on `internal/cli`.

## Coding Standards & Idioms
In accordance with the [Google Go Style Guide](https://google.github.io/styleguide/go/guide) and `docs/spec/global.md`:

- **External Execution Boundary (`git`):**
  - NEVER call `os/exec.Command` directly in `internal/cli` or `internal/domain`.
  - ALWAYS invoke Git through `internal/git.Runner`.
  - Sanitize subprocess environments with `BuildSanitizedEnv`: inject `LC_ALL=C`, `LANG=C`, `GIT_TERMINAL_PROMPT=0`, `GIT_ADVICE=0`, and `-c gc.auto=0`.
  - Prefer deterministic Git plumbing (`rev-parse`, `symbolic-ref`, `show-ref`, `ls-remote`, `diff-index`, `rev-list`) over porcelain.
- **Context, Cancellation & Cleanup:**
  - Every external execution (Git subprocesses, HTTP calls) must accept a `context.Context` bound to `SIGINT`/`SIGTERM`.
  - Subprocesses configure graceful `SIGTERM` cancellation with `WaitDelay = 3 * time.Second`.
  - Map process context cancellation directly to exit code `130` (`ExitInterrupted`).
- **Error Handling:**
  - Never call `log.Fatal()`, `panic()`, or `os.Exit()` inside `internal/`. Return explicit `error` values up to `main.go`.
  - Construct categorized errors with `domain.NewError(code, ...)` / `domain.WrapError(code, err, ...)` and optional `.WithHint(...)` for deterministic exit code mapping at the CLI boundary.
- **Libraries & Dependencies:**
  - Use the Go standard library (`os`, `strings`, `bufio`, `regexp`, `context`, `net/http`)—never shell out to legacy external script utilities.
  - Allowed third-party dependencies: `github.com/spf13/cobra`, `github.com/stretchr/testify`, `gopkg.in/yaml.v3`. Do NOT import heavyweight Git libraries (`go-git`).

## Testing Rules
- Every subcommand and package must include:
  1. Table-driven unit tests for pure logic, parsers, and CLI flows using mock `git.Runner` interfaces.
  2. Integration tests creating disposable repositories in `t.TempDir()` via `test/integration/harness.go`.
- Tests MUST NOT modify or rely on the host machine's `~/.gitconfig` or global Git state.

## Workflow & Output Rules
- Check `docs/spec/global.md` and `docs/spec/<subcommand>.md` for behavioral requirements before writing code; implement strict spec parity first.
- Enforce stream separation:
  - `stdout`: strictly for primary command results, machine-parseable data (`name`, `config`), formatted graphs (`history`), shell scripts (`completion`), and `--help`/`--version`.
  - `stderr`: all operational logs (`[git-brx] <message>`), warnings (`[git-brx] Warning: <message>`), errors (`[git-brx] Error: <message>`), actionable hints (`[git-brx] Hint: <message>`), and debug diagnostics (`[git-brx] Debug: <message>` in magenta).

# Git Workflow Policy

- Always use `git-brx` (or `git brx`) CLI commands for Git branch operations:
  - Creating topic branches: `git brx create <branch> -y`
  - Upstream synchronization: `git brx sync`
  - Pulling collaborator commits: `git brx update`
  - Pushing branches: `git brx publish`
  - Submitting PRs: `git brx review`
  - Cleaning merged branches: `git brx delete <branch>`

- Do not run manual `git checkout -b`, `git push -f`, or raw API scripts for PR creation.
