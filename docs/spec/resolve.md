# Spec: git-brx resolve

## 1. Command Signature
- **Usage:** `git-brx resolve [flags]`
- **Arguments:** None.
- **Flags:**
  - Standard global flags: `--help` (`-h`), `--version`, `--verbose` (`-v`), `--quiet` (`-q`), `--no-color`.
  - Feature flags:
    - `--tool` (`-t` `<tool>`): Specify explicit merge tool override (defaults to configured `git config merge.tool`).
    - `--no-tool`: Never launch an external merge tool; stage marker-free unmerged files and continue, or fail listing files with remaining conflict markers.

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
3. **Conflict Identification, Marker Check & Resolution:**
   - Query currently conflicted files:
     ```bash
     git diff --name-only --diff-filter=U
     ```
   - Scan each unmerged path in-process for conflict markers (`<<<<<<<`, `=======`, `>>>>>>>`).
   - **Auto-stage path:** If all unmerged files contain zero conflict markers (e.g., already resolved manually in an editor or by an AI agent), skip `git mergetool` and proceed directly to Targeted Staging.
   - **Interactive mergetool path:** If any unmerged file still contains conflict markers:
     - If `--no-tool` is set, or no `merge.tool` is configured in Git (and `--tool` was not passed):
       - Log the list of still-conflicted file paths to `stderr`.
       - Log hint: `[git-brx] Hint: Resolve conflict markers in the listed files and re-run 'git-brx resolve', or run 'git-brx reset' to abort`.
       - Fail with Exit Code `5`.
     - Otherwise, launch configured merge tool:
       ```bash
       git mergetool --no-prompt [-t <tool>]
       ```
4. **Targeted Staging:**
   - Query remaining unmerged paths and verify no conflict markers remain:
     ```bash
     git diff --name-only --diff-filter=U
     ```
   - If conflicts or markers remain unresolved, log unresolved paths and fail with Exit Code `5` without proceeding to commit/continue.
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
     - If failed, emit hint: `[git-brx] Hint: Use 'git-brx reset' to abort the merge without losing commits`.
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
       - Emit hint: `[git-brx] Hint: Use 'git-brx reset' to abort the rebase without losing commits`.
     - Once all commits are rebased, log: `[git-brx] Rebase completed`.
6. **Completion:** Exit with Exit Code `0`.

## 4. Error Handling & Exit Codes
| Exit Code | Scenario | Emitted Stderr Pattern | Side Effect Cleanup |
| :--- | :--- | :--- | :--- |
| `0` | Conflicts resolved and merge/rebase continued successfully (or no conflict active). | `[git-brx] Merge completed` / `[git-brx] Rebase completed` | None. |
| `2` | Unknown arguments or flags passed. | `[git-brx] Error: Unexpected argument: <arg>` | None. |
| `3` | Outside repository. | `[git-brx] Error: Not a git repository` | None. |
| `5` | Mergetool crashed, unavailable in headless/`--no-tool` mode, or conflict markers remain unresolved. | `[git-brx] Error: Conflicts remain unresolved; cannot continue` | Preserves in-flight conflict state. |

## 5. Edge Cases & Behavioral Notes
- **Targeted Staging Isolation:** Identifies unmerged paths specifically using `git diff --name-only --diff-filter=U`. Only resolved paths that were genuinely in conflict are staged (`git add -- <paths...>`), preventing unrelated local edits from being swept in.
- **Headless & Manual Resolution Support:** When conflict markers have already been removed in the working tree, `resolve` stages the resolved unmerged files directly without invoking `git mergetool`. In non-TTY/headless sessions or with `--no-tool`, remaining markers fail fast with the conflicted file list instead of hanging.
- **Full Merge Message Preservation:** Completes active merges using `git commit --no-edit`, preserving the full original merge commit message payload.
- **Deterministic Loop Termination:** Bounded loop execution during conflict continuation ensures that tool cancellation or non-zero mergetool exit terminates cleanly rather than entering infinite retry loops.

## 6. Black-box verification scenarios
| Scenario ID | Given | When | Then (exit code, output, state) |
| :--- | :--- | :--- | :--- |
| `TC-RSV-01` | Active rebase conflict where user cleared all conflict markers in unmerged files without running `git add` | `git-brx resolve` | Skips `git mergetool`, stages only unmerged files via `git add --`, completes rebase, exits `0`. |
| `TC-RSV-02` | Active merge conflict with `<<<<<<<` markers remaining and `--no-tool` (or non-TTY / no `merge.tool`) | `git-brx resolve --no-tool` | Lists conflicted file paths on `stderr`, emits `git-brx reset` hint, exits `5` without invoking `mergetool`. |
| `TC-RSV-03` | Multi-commit rebase where step 1 is resolved and step 2 hits a new conflict | `git-brx resolve` | Advances to step 2, resolves and completes rebase within bounded iterations, exits `0`. |
