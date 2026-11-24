# Spec: git-brx reset

## 1. Command Signature
- **Usage:** `git-brx reset [flags]`
- **Arguments:** None.
- **Flags:**
  - Standard global flags: `--help` (`-h`), `--version`, `--verbose` (`-v`), `--quiet` (`-q`), `--no-color`.
  - Feature flag: `--clean` (`-c`): Also remove untracked files and directories via `git clean -fd` (only valid when no merge or rebase conflict is active).

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
   - Check active operation (`.git/MERGE_HEAD`, `.git/rebase-merge`, `.git/rebase-apply`).
   - If an active operation is present and `--clean` is passed, fail with Exit Code `2` (`Run 'git-brx reset' to abort the active <op> first before using '--clean'`) before modifying state.
   - If no active operation is present, assert `origin` remote exists (`git remote get-url origin`).
2. **Abort In-Flight Rebase or Merge:**
   - Abort any in-flight operations first, ensuring that any temporary detached HEAD states induced by mid-flight rebases are restored:
     - Check if merge is in progress (`.git/MERGE_HEAD` exists):
       - Log: `[git-brx] Aborting in-progress merge`
       - Execute: `git merge --abort`. If this fails, exit with Exit Code `5`.
     - Check if rebase is in progress (`.git/rebase-merge` or `.git/rebase-apply` exists):
       - Log: `[git-brx] Aborting in-progress rebase`
       - Execute: `git rebase --abort`. If this fails, exit with Exit Code `5`.
   - If an in-flight operation was aborted:
     - Log: `[git-brx] Aborted in-progress <op>; local commits preserved`
     - If `origin/<branch>` exists locally, log hint: `[git-brx] Hint: Run 'git-brx reset' again to discard local changes and match 'origin/<branch>'`.
     - Exit with Exit Code `0` (do NOT require `origin/<branch>` and do NOT run `git reset --hard`).
3. **Attached Branch & Remote Reference Validation (Idle Mode):**
   - Assert attached branch: `git symbolic-ref --short -q HEAD`. Store current branch name.
   - Verify that the remote tracking reference `origin/<branch>` exists:
     ```bash
     git rev-parse --verify -q origin/<branch>
     ```

   - If not found:
     - Log: `[git-brx] Error: Reset failed: remote reference 'origin/<branch>' does not exist`
     - Log: `[git-brx] Hint: Your branch may not have been published yet. Use 'git-brx publish' to publish it`
     - Fail with Exit Code `4`.
4. **Hard Reset Execution (Idle Mode):**
   - Execute:
     ```bash
     git reset --hard origin/<branch>
     ```
   - If `--clean` flag was passed:
     ```bash
     git clean -fd
     ```
5. **Completion:**
   - Log: `[git-brx] Working tree successfully reset to 'origin/<branch>'`.
   - Exit with Exit Code `0`.

## 4. Error Handling & Exit Codes
| Exit Code | Scenario | Emitted Stderr Pattern | Side Effect Cleanup |
| :--- | :--- | :--- | :--- |
| `0` | Active merge/rebase aborted safely preserving local commits, or idle working copy reset to remote tip. | `[git-brx] Aborted in-progress <op>; local commits preserved` / `[git-brx] Working tree successfully reset to 'origin/<branch>'` | None. |
| `2` | Positional arguments, unknown flags, or `--clean` passed during active conflict. | `[git-brx] Error: Unexpected argument: <arg>` / `[git-brx] Error: Cannot use '--clean' while a <op> is in progress` | None. |
| `3` | Outside repository or detached HEAD. | `[git-brx] Error: You are in detached HEAD state, this means you are not on any branch` | None. |
| `4` | Origin missing or `origin/<branch>` does not exist (idle mode). | `[git-brx] Error: Reset failed: remote reference 'origin/<branch>' does not exist` | None. |
| `5` | `git merge --abort` or `git rebase --abort` failed. | `[git-brx] Error: Reset failed (git merge --abort failed)` | None. |

## 5. Edge Cases & Behavioral Notes
- **Safe conflict abort by default:** When a merge or rebase conflict is active, `reset` performs an abort-only restore (`git merge --abort` / `git rebase --abort`) without touching unpushed commits or requiring `origin/<branch>`. Running `git-brx reset` a second time (when idle) discards local commits and resets to `origin/<branch>`.
- **Untracked File Management (`--clean`):** Standard idle `reset` restores tracked files via `git reset --hard`. To discard untracked working copy artifacts as well, `--clean` may be passed in idle mode.
- **Unpublished Branch Pre-Check:** Before attempting a reset against `origin/<branch>` in idle mode, `reset` verifies the existence of the remote tracking branch and suggests running `publish` if it does not yet exist.
- **Direct In-Process Execution:** Executes directly within the compiled binary without nested subshell invocations or wrapper script overhead.

## 6. Black-box verification scenarios
| Scenario ID | Given | When | Then (exit code, output, state) |
| :--- | :--- | :--- | :--- |
| `TC-RST-01` | Unpublished branch with unpushed local commit and active rebase conflict | `git-brx reset` | Exits `0`, logs `Aborted in-progress rebase; local commits preserved`, unpushed commit remains intact. |
| `TC-RST-02` | Published branch with unpushed commit and active merge conflict | `git-brx reset` twice | First run aborts merge, preserves commit, emits re-run hint, exits `0`; second run resets `HEAD` to `origin/<branch>`, exits `0`. |
| `TC-RST-03` | Clean/dirty published branch with no active conflict | `git-brx reset` | Exits `0`, resets working tree to `origin/<branch>`. |
| `TC-RST-04` | Active rebase conflict | `git-brx reset --clean` | Exits `2` with hint to abort first before using `--clean`; does not abort or delete untracked files. |
