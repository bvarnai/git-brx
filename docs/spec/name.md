# Spec: git-brx name

## 1. Command Signature
- **Usage:** `git-brx name [flags]`
- **Arguments:** None. (Positional arguments are rejected).
- **Flags:**
  - Standard global flags: `--help` (`-h`), `--version`, `--verbose` (`-v`), `--quiet` (`-q`), `--no-color`.

## 2. Prerequisites & Environment
- **Required host binaries:** `git` (>= 2.20.0).
- **Environment variables read:** `GIT_DIR`, `GIT_WORK_TREE`, `GIT_PREFIX`, `NO_COLOR`.
- **Working directory requirements (GIT_PREFIX behavior):** Can be invoked from any subdirectory inside a Git repository. Operates invariant of `GIT_PREFIX`.
- **Git state assertions (e.g., dirty working tree check):**
  - Must be inside a Git working tree (`git rev-parse --is-inside-work-tree` == `true`).
  - Safe to execute regardless of dirty working tree, staging status, or detached HEAD state.

## 3. Core Execution Flow
1. **Repository Verification:** Run `git rev-parse --is-inside-work-tree`. If non-zero, fail immediately.
2. **Branch Name Resolution:** Execute `git symbolic-ref --short -q HEAD`.
   - **Attached HEAD:** If exit code is `0`, print the resulting string followed by a newline (`\n`) directly to `stdout`. Exit code `0`.
   - **Detached HEAD:** If exit code is non-zero (indicating detached HEAD):
     - Resolve the current detached commit hash via `git rev-parse --short HEAD`.
     - Print the short commit hash or `HEAD` to `stdout`.
     - In diagnostic mode (`stderr`), log: `[git-brx] Warning: HEAD is detached at <commit>`.
     - Exit code `0` (or `3` if strict attached validation is requested).

## 4. Error Handling & Exit Codes
| Exit Code | Scenario | Emitted Stderr Pattern | Side Effect Cleanup |
| :--- | :--- | :--- | :--- |
| `0` | Branch or detached commit resolved successfully. | None (unless warning on detached HEAD). | None. |
| `2` | Unexpected positional arguments or unknown flags passed. | `[git-brx] Error: Unexpected argument: <arg>` | None. |
| `3` | Invoked outside a valid Git repository or inside `.git`. | `[git-brx] Error: Not a git repository` | None. |

## 5. Edge Cases & Behavioral Notes
- **Clean Stdout for Automation:** Standard output is strictly reserved for the branch name string followed by a newline, ensuring safe shell capture (`NAME=$(git-brx name)`). All validation diagnostics or unexpected arguments are emitted to `stderr` with exit code `2`.
- **Atomic Git Plumbing:** Active branch inspection uses atomic Git plumbing (`git symbolic-ref --short -q HEAD`) for fast and reliable resolution.
- **Clean Detached HEAD Identifier:** When HEAD is detached, `name` emits a clean short commit SHA or ref identifier without terminal prompt decoration or styling artifacts.
