# Spec: git-brx publish

## 1. Command Signature
- **Usage:** `git-brx publish [flags]`
- **Arguments:** None.
- **Flags:**
  - Standard global flags: `--help` (`-h`), `--version`, `--verbose` (`-v`), `--quiet` (`-q`), `--no-color`.
  - Safety override: `--no-force`: Push with standard fast-forward constraints without `--force-with-lease`.

## 2. Prerequisites & Environment
- **Required host binaries:** `git` (>= 2.20.0).
- **Environment variables read:** `GIT_DIR`, `GIT_WORK_TREE`, `GIT_PREFIX`, `NO_COLOR`.
- **Working directory requirements (GIT_PREFIX behavior):** Can be invoked from any repository subdirectory; operations are executed relative to repository root (`git rev-parse --show-toplevel`).
- **Git state assertions (e.g., dirty working tree check):**
  - Must be inside a Git working tree (`git rev-parse --is-inside-work-tree` == `true`).
  - Must NOT be in a shallow repository (`git rev-parse --is-shallow-repository` must be `false`).
  - Must NOT be in a detached HEAD state (`git symbolic-ref -q HEAD`).
  - Remote `origin` must exist.
  - Protected branch check: Publishing with force-with-lease directly to `main`, `master`, or `release/*` should be guarded or rejected unless explicit flags are given.

## 3. Core Execution Flow
1. **Repository & State Assertions:**
   - Verify worktree: `git rev-parse --is-inside-work-tree`.
   - Verify non-shallow repository: `git rev-parse --is-shallow-repository`. If true, fail with Exit Code `3`.
   - Verify attached branch: `git symbolic-ref --short -q HEAD`. Store current branch name `<branch>`.
   - Protected branch assertion: If `<branch>` is `main` or `master`, log warning or require confirmation before pushing with lease.
2. **Upstream Tracking Evaluation:**
   - Inspect existing upstream configuration:
     ```bash
     git for-each-ref --format='%(upstream:short)' refs/heads/<branch>
     ```
   - Determine if upstream tracking matches `origin/<branch>`.
3. **Push Execution:**
   - **Case A: Upstream already configured as `origin/<branch>`:**
     ```bash
     git push origin <branch> --force-with-lease
     ```
   - **Case B: First publish (upstream unset or different):**
     ```bash
     git push --set-upstream origin <branch> --force-with-lease
     ```
4. **Lease Failure Interception:**
   - If push is rejected due to stale lease (non-fast-forward / lease mismatch):
     - Log: `[git-brx] Error: Publish rejected: remote has newer commits`
     - Log: `[git-brx] Hint: Run 'git-brx update' to incorporate remote changes before publishing`
     - Fail with Exit Code `4`.
   - If push fails due to network or authentication failure:
     - Log: `[git-brx] Error: Publish failed: unable to reach remote or authentication rejected`
     - Fail with Exit Code `4`.
5. **Completion:**
   - Log: `[git-brx] Branch '<branch>' successfully published to origin`.
   - Exit with Exit Code `0`.

## 4. Error Handling & Exit Codes
| Exit Code | Scenario | Emitted Stderr Pattern | Side Effect Cleanup |
| :--- | :--- | :--- | :--- |
| `0` | Branch pushed and tracking configured cleanly. | `[git-brx] Branch '<branch>' successfully published to origin` | None. |
| `2` | Unknown arguments or flags passed. | `[git-brx] Error: Unexpected argument: <arg>` | None. |
| `3` | Shallow repository or detached HEAD. | `[git-brx] Error: You are in a shallow repository` / `[git-brx] Error: You are in detached HEAD state` | None. |
| `4` | Push rejected by server (lease mismatch) or remote unreachable. | `[git-brx] Error: Publish rejected: remote has newer commits` / `[git-brx] Error: Publish failed` | None. |

## 5. Edge Cases & Behavioral Notes
- **Actionable Lease Rejection Diagnostics:** When a push is rejected due to a lease mismatch (`--force-with-lease`), `publish` detects the condition and provides a specific diagnostic message advising the user to run `update`.
- **Protected Default Branch Safety:** Explicit client-side safety checks block accidental force-pushing to default branches (`main`, `master`), requiring explicit branch operations.
- **Robust Argument Parsing:** Positional arguments and flags are parsed cleanly through native CLI structures, preventing unexpected word splitting.
