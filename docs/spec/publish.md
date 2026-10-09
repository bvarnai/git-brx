# Spec: git-brx publish

## 1. Command Signature
- **Usage:** `git-brx publish [flags]`
- **Arguments:** None.
- **Flags:**
  - Standard global flags: `--help` (`-h`), `--version`, `--verbose` (`-v`), `--quiet` (`-q`), `--dry-run` (`-n`), `--no-color`.
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
  - Protected branch check: Publishing with force-with-lease directly to `master` or `release/*` should be guarded or rejected unless explicit flags are given.

## 3. Core Execution Flow
1. **Repository & State Assertions:**
   - Verify worktree: `git rev-parse --is-inside-work-tree`.
   - Verify non-shallow repository: `git rev-parse --is-shallow-repository`. If true, fail with Exit Code `3`.
   - Verify attached branch: `git symbolic-ref --short -q HEAD`. Store current branch name `<branch>`.
   - Protected branch assertion: If `<branch>` is `master`, log warning or require confirmation before pushing with lease.
2. **Upstream Tracking Evaluation:**
   - Inspect existing upstream configuration:
     ```bash
     git for-each-ref --format='%(upstream:short)' refs/heads/<branch>
     ```
   - Determine if upstream tracking matches `origin/<branch>`.
3. **Dry-Run Check:**
   - If `--dry-run` is active: Run `git push --dry-run [args]`, log planned ref updates, and exit with Exit Code `0`.
4. **Push Execution:**
   - **Case A: Upstream already configured as `origin/<branch>`:**
     ```bash
     git push origin <branch> --force-with-lease
     ```
   - **Case B: First publish (upstream unset or different):**
     ```bash
     git push --set-upstream origin <branch> --force-with-lease
     ```
5. **Lease Failure Interception:**
   - If push is rejected due to stale lease (non-fast-forward / lease mismatch):
     - Log: `[git-brx] ! Publish rejected: remote has newer commits`
     - Log: `[git-brx] Hint: Run 'git-brx update' to incorporate remote changes before publishing`
     - Fail with Exit Code `4`.
   - If push fails due to network or authentication failure:
     - Log: `[git-brx] ! Publish failed: unable to reach remote or authentication rejected`
     - Fail with Exit Code `4`.
6. **Completion:**
   - Log: `[git-brx] Branch '<branch>' successfully published to origin`.
   - Exit with Exit Code `0`.

## 4. Error Handling & Exit Codes
| Exit Code | Scenario | Emitted Stderr Pattern | Side Effect Cleanup |
| :--- | :--- | :--- | :--- |
| `0` | Branch pushed and tracking configured cleanly. | `[git-brx] Branch '<branch>' successfully published to origin` | None. |
| `2` | Unknown arguments or flags passed. | `[git-brx] ! Unexpected argument: <arg>` | None. |
| `3` | Shallow repository or detached HEAD. | `[git-brx] ! You are in a shallow repository` / `[git-brx] ! You are in detached HEAD state` | None. |
| `4` | Push rejected by server (lease mismatch) or remote unreachable. | `[git-brx] ! Publish rejected: remote has newer commits` / `[git-brx] ! Publish failed` | None. |

## 5. Discrepancies & Edge Cases Discovered
- **Stale Lease Diagnostics:** Legacy `publish.sh` issued a generic `Publish failed (gitish: 'git push ...' failed)` on lease rejections. The Go specification detects `--force-with-lease` rejections specifically and suggests running `update`.
- **Accidental Force Push on Master:** Legacy script had no protection against running `publish` on `master`, immediately executing `git push origin master --force-with-lease`. While server branch permissions may reject it, client-side safety checks prevent accidental destructive pushes.
- **Unquoted Parameter Word Splitting:** Legacy Git alias passed `$@` without double quotes. Cleanly handled by native Go argument parser.
