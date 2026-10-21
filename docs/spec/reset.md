# Spec: git-brx reset

## 1. Command Signature
- **Usage:** `git-brx reset [flags]`
- **Arguments:** None.
- **Flags:**
  - Standard global flags: `--help` (`-h`), `--version`, `--verbose` (`-v`), `--quiet` (`-q`), `--dry-run` (`-n`), `--no-color`.
  - Feature flag: `--clean` (`-c`): Also remove untracked files and directories via `git clean -fd`.

## 2. Prerequisites & Environment
- **Required host binaries:** `git` (>= 2.20.0).
- **Environment variables read:** `GIT_DIR`, `GIT_WORK_TREE`, `GIT_PREFIX`, `NO_COLOR`.
- **Working directory requirements (GIT_PREFIX behavior):** Can be invoked from any repository subdirectory; operations are executed relative to repository root (`git rev-parse --show-toplevel`).
- **Git state assertions (e.g., dirty working tree check):**
  - Must be inside a Git working tree (`git rev-parse --is-inside-work-tree` == `true`).
  - Remote `origin` must be configured (`git remote get-url origin`).
  - Must NOT be in a detached HEAD state (`git symbolic-ref -q HEAD`).

## 3. Core Execution Flow
1. **Repository & State Verification:**
   - Verify worktree: `git rev-parse --is-inside-work-tree`.
   - Assert origin exists: `git remote get-url origin`.
2. **Abort In-Flight Rebase or Merge:**
   - Abort any in-flight operations first, ensuring that any temporary detached HEAD states induced by mid-flight rebases are restored:
     - Check if merge is in progress (`.git/MERGE_HEAD` exists):
       - Log: `[git-brx] Aborting in-progress merge`
       - Execute: `git merge --abort`. If this fails, exit with Exit Code `5`.
     - Check if rebase is in progress (`.git/rebase-merge` or `.git/rebase-apply` exists):
       - Log: `[git-brx] Aborting in-progress rebase`
       - Execute: `git rebase --abort`. If this fails, exit with Exit Code `5`.
3. **Attached Branch & Remote Reference Validation:**
   - Assert attached branch: `git symbolic-ref --short -q HEAD`. Store current branch name.
   - Verify that the remote tracking reference `origin/<branch>` exists:
     ```bash
     git rev-parse --verify -q origin/<branch>
     ```

   - If not found:
     - Log: `[git-brx] Error: Reset failed: remote reference 'origin/<branch>' does not exist`
     - Log: `[git-brx] Hint: Your branch may not have been published yet. Use 'git-brx publish' to publish it`
     - Fail with Exit Code `4`.
4. **Dry-Run Check:**
   - If `--dry-run` is active: Log intended `git reset --hard origin/<branch>` (and optional `git clean -fd`), then exit with Exit Code `0`.
5. **Hard Reset Execution:**
   - Execute:
     ```bash
     git reset --hard origin/<branch>
     ```
   - If `--clean` flag was passed:
     ```bash
     git clean -fd
     ```
6. **Completion:**
   - Log: `[git-brx] Working tree successfully reset to 'origin/<branch>'`.
   - Exit with Exit Code `0`.

## 4. Error Handling & Exit Codes
| Exit Code | Scenario | Emitted Stderr Pattern | Side Effect Cleanup |
| :--- | :--- | :--- | :--- |
| `0` | In-flight operations aborted and working copy reset to remote tip. | `[git-brx] Working tree successfully reset to 'origin/<branch>'` | None. |
| `2` | Positional arguments or unknown flags supplied. | `[git-brx] Error: Unexpected argument: <arg>` | None. |
| `3` | Outside repository or detached HEAD. | `[git-brx] Error: You are in detached HEAD state, this means you are not on any branch` | None. |
| `4` | Origin missing or `origin/<branch>` does not exist. | `[git-brx] Error: Reset failed: remote reference 'origin/<branch>' does not exist` | None. |
| `5` | `git merge --abort` or `git rebase --abort` failed. | `[git-brx] Error: Reset failed (git merge --abort failed)` | None. |

## 5. Edge Cases & Behavioral Notes
- **Untracked File Management (`--clean`):** Standard `reset` restores tracked files via `git reset --hard`. To discard untracked working copy artifacts as well, the `--clean` flag may be explicitly passed (`git clean -fd`).
- **Unpublished Branch Pre-Check:** Before attempting a hard reset against `origin/<branch>`, `reset` verifies the existence of the remote tracking branch and suggests running `publish` if it does not yet exist.
- **Direct In-Process Execution:** Executes directly within the compiled binary without nested subshell invocations or wrapper script overhead.
