# Spec: git-brx resolve

## 1. Command Signature
- **Usage:** `git-brx resolve [flags]`
- **Arguments:** None.
- **Flags:**
  - Standard global flags: `--help` (`-h`), `--version`, `--verbose` (`-v`), `--quiet` (`-q`), `--no-color`.
  - Tool flag: `--tool` (`-t` `<tool>`): Specify explicit merge tool override (defaults to configured `git config merge.tool`).

## 2. Prerequisites & Environment
- **Required host binaries:** `git` (>= 2.20.0), configured external merge tool (e.g., Beyond Compare, Meld, VSCode).
- **Environment variables read:** `GIT_DIR`, `GIT_WORK_TREE`, `GIT_PREFIX`, `NO_COLOR`.
- **Working directory requirements (GIT_PREFIX behavior):** Can be invoked from any repository subdirectory; operations are executed relative to repository root (`git rev-parse --show-toplevel`).
- **Git state assertions (e.g., dirty working tree check):**
  - Must be inside a Git working tree (`git rev-parse --is-inside-work-tree` == `true`).
  - Active rebase (`.git/rebase-merge` / `.git/rebase-apply`) or merge (`.git/MERGE_HEAD`) MUST be in progress. (If neither is active, command exits cleanly with a notice).

## 3. Core Execution Flow
1. **Repository Verification:** Run `git rev-parse --is-inside-work-tree`.
2. **State Detection:**
   - Detect active merge: Check if `.git/MERGE_HEAD` exists.
   - Detect active rebase: Check if `.git/rebase-merge` or `.git/rebase-apply` exists.
   - If neither is active:
     - Log: `[git-brx] No merge/rebase is ongoing`
     - Exit with Exit Code `0`.
3. **Conflict Identification & Interactive Mergetool:**
   - Query currently conflicted files:
     ```bash
     git diff --name-only --diff-filter=U
     ```
   - Launch configured merge tool:
     ```bash
     git mergetool --no-prompt
     ```
4. **Targeted Staging:**
   - Instead of blind `git add .`, query unmerged paths and verify each file is resolved:
     ```bash
     git diff --name-only --diff-filter=U
     ```
   - If conflicts remain unresolved, warn the user and do not proceed with automatic commit/continue.
   - For all resolved paths, stage them explicitly:
     ```bash
     git add -- <resolved_paths...>
     ```
5. **Continuation Branching:**
   - **Merge Flow:**
     - Log: `[git-brx] Resolving merge conflicts...`
     - Check if `.git/MERGE_MSG` exists.
     - Finalize the merge using standard Git continuation:
       ```bash
       git commit --no-edit
       ```
       *(Preserves the full default merge message without truncation).*
     - If failed, emit hint: `[git-brx] Hint: Use 'git-brx reset' to abort the merge and restore your branch`.
     - Log: `[git-brx] Merge completed`.
   - **Rebase Flow:**
     - Check active rebase progress via `.git/rebase-merge/msgnum` and `.git/rebase-merge/end`.
     - Log current step: `[git-brx] Resolving rebase conflict at step <current> of <total>...`
     - Attempt to continue the rebase:
       ```bash
       git rebase --continue
       ```
       *(Executed with `GIT_EDITOR=true` to prevent unexpected terminal hangs).*
     - If subsequent commits in the rebase series encounter new conflicts:
       - Log: `[git-brx] Conflicts encountered on subsequent commit in rebase series`
       - Repeat the resolution cycle with updated step indicators. Loop is capped at 20 iterations to prevent infinite execution.
     - If rebase continuation fails or is aborted:
       - Emit hint: `[git-brx] Hint: Use 'git-brx reset' to abort the rebase and restore your branch`.
     - Once all commits are rebased, log: `[git-brx] Rebase completed`.
6. **Completion:** Exit with Exit Code `0`.

## 4. Error Handling & Exit Codes
| Exit Code | Scenario | Emitted Stderr Pattern | Side Effect Cleanup |
| :--- | :--- | :--- | :--- |
| `0` | Conflicts resolved and merge/rebase continued successfully (or no conflict active). | `[git-brx] Merge completed` / `[git-brx] Rebase completed` | None. |
| `2` | Unknown arguments or flags passed. | `[git-brx] ! Unexpected argument: <arg>` | None. |
| `3` | Outside repository. | `[git-brx] ! Awh! This is not a git repository` | None. |
| `5` | Mergetool crashed or user failed to resolve conflicts, leaving unmerged files. | `[git-brx] ! Conflicts remain unresolved; cannot continue` | Preserves in-flight conflict state. |

## 5. Discrepancies & Edge Cases Discovered
- **Indiscriminate Staging (`git add .`):** Legacy `resolve.sh` ran `git add .`, which staged any untracked or unrelated files modified elsewhere in the repository. The Go specification isolates and stages *only* the specific files flagged as unmerged (`--diff-filter=U`).
- **Commit Message Truncation:** Legacy script extracted the merge message with `line=$(head -n 1 .git/MERGE_MSG)` and committed with `-m "${line}"`, truncating multi-line merge details. Modern Go invokes `git commit --no-edit` to preserve the complete message payload.
- **Infinite Rebase Loop Risk:** Legacy script executed `while ! git rebase --continue; do git mergetool --no-prompt; git add .; done`. If the user cancelled the mergetool or resolution failed, the loop ran indefinitely. Modern Go checks process exit status and aborts cleanly upon repeated failures.
