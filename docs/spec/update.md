# Spec: git-brx update

## 1. Command Signature
- **Usage:** `git-brx update [flags]`
- **Arguments:** None.
- **Flags:**
  - Standard global flags: `--help` (`-h`), `--version`, `--verbose` (`-v`), `--quiet` (`-q`), `--dry-run` (`-n`), `--no-color`.
  - Feature flag: `--autostash` (`-a`): Automatically stash unstaged/staged changes before rebasing and pop stash afterwards.

## 2. Prerequisites & Environment
- **Required host binaries:** `git` (>= 2.20.0).
- **Environment variables read:** `GIT_DIR`, `GIT_WORK_TREE`, `GIT_PREFIX`, `NO_COLOR`.
- **Working directory requirements (GIT_PREFIX behavior):** Can be invoked from any repository subdirectory; operations are executed relative to repository root (`git rev-parse --show-toplevel`).
- **Git state assertions (e.g., dirty working tree check):**
  - Must be inside a Git working tree (`git rev-parse --is-inside-work-tree` == `true`).
  - Must NOT be in a detached HEAD state (`git symbolic-ref -q HEAD`).
  - Remote `origin` must be configured (`git remote get-url origin`).
  - Must NOT be in the middle of an in-flight rebase (`.git/rebase-merge` or `.git/rebase-apply`) or merge (`.git/MERGE_HEAD`).
  - If `--autostash` is not provided, working tree must be clean or not conflict with rebased commits.

## 3. Core Execution Flow
1. **Repository & State Assertions:**
   - Verify repository context: `git rev-parse --is-inside-work-tree`.
   - Assert attached branch: `git symbolic-ref --short -q HEAD`. Store current branch name.
   - Assert remote origin exists: `git remote get-url origin`.
   - Assert no active merge/rebase: Check `.git/MERGE_HEAD`, `.git/rebase-merge`, `.git/rebase-apply`. If present, exit with Exit Code `5`.
2. **Remote Reference Verification:**
   - Verify that the current branch exists on `origin` via `git ls-remote --heads origin <branch>`.
   - If not found on remote, emit warning: `[git-brx] Error: Branch '<branch>' has not been published to origin yet` with hint `[git-brx] Hint: Use 'git-brx publish' to publish your branch first` and exit with Exit Code `4`.
3. **Dry-Run Check:**
   - If `--dry-run` is active: Perform `git fetch --dry-run origin <branch>`, compute commit delta, log planned rebase actions, and exit with Exit Code `0`.
4. **Fetch & Rebase Execution:**
   - Fetch latest remote changes: `git fetch origin <branch>`.
   - Execute rebase:
     ```bash
     git rebase origin/<branch>
     ```
     *(If `--autostash` is enabled, append `--autostash`).*
5. **Conflict Interception:**
   - If `git rebase` exits with a non-zero code due to merge conflicts:
     - Log diagnostic message: `[git-brx] Error: Update failed due to conflicts`
     - Log actionable hint: `[git-brx] Hint: Use 'git-brx resolve' to resolve conflicts or 'git-brx reset' to abort`
     - Terminate with Exit Code `5`.
6. **Completion:**
   - Log informational success: `[git-brx] Current branch '<branch>' is up to date`.
   - Terminate with Exit Code `0`.

## 4. Error Handling & Exit Codes
| Exit Code | Scenario | Emitted Stderr Pattern | Side Effect Cleanup |
| :--- | :--- | :--- | :--- |
| `0` | Branch successfully updated and rebased onto remote tip. | `[git-brx] Current branch '<branch>' is up to date` | None. |
| `2` | Positional arguments or unknown flags supplied. | `[git-brx] Error: Unexpected argument: <arg>` | None. |
| `3` | Outside repository or detached HEAD. | `[git-brx] Error: You are in detached HEAD state, this means you are not on any branch` | None. |
| `4` | Origin missing, unreachable, or branch not published on remote. | `[git-brx] Error: Your repository has no remote origin` / `[git-brx] Error: Branch not found on remote` | None. |
| `5` | Active rebase/merge already in progress or conflicts hit during rebase. | `[git-brx] Error: You are in a middle of a rebase/merge` / `[git-brx] Error: Update failed due to conflicts` | Leave in conflicted rebase state for `resolve`. |

## 5. Discrepancies & Edge Cases Discovered
- **Unpublished Branch Trap:** Legacy `update.sh` executed `git pull --rebase origin`. When invoked on a freshly created local branch that was never pushed, Git output confusing refspec errors. Modern spec explicitly verifies remote branch existence beforehand with an actionable hint to use `publish`.
- **Implicit Upstream Rebase Assumption:** Legacy executed `git pull --rebase origin`, relying on Git's default upstream mapping or pulling HEAD refspec, which occasionally pulled from the remote's default branch rather than the tracking topic branch. Modern Go implementation explicitly targets `origin/<current_branch>`.
- **Dirty Working Tree Conflicts:** Legacy offered no autostash option, causing `git pull --rebase` to fail destructively if local files were modified. The Go spec supports `--autostash` to safely preserve uncommitted edits.
