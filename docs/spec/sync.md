# Spec: git-brx sync

## 1. Command Signature
- **Usage:** `git-brx sync [flags] [<branch>]`
- **Arguments:**
  - `<branch>` *(optional)*: Base branch to synchronize changes from. Defaults to `master` if omitted.
- **Flags:**
  - Standard global flags: `--help` (`-h`), `--version`, `--verbose` (`-v`), `--quiet` (`-q`), `--dry-run` (`-n`), `--no-color`.
  - Strategy flags:
    - `-m`, `--merge`: Force merge strategy regardless of branch type default.
    - `-r`, `--rebase`: Force rebase strategy regardless of branch type default.
    - `-a`, `--autostash`: Enable automatic stashing of uncommitted changes (rebase only).
    - `-i`, `--interactive`: Launch interactive rebase (rebase only).

## 2. Prerequisites & Environment
- **Required host binaries:** `git` (>= 2.20.0).
- **Environment variables read:** `GIT_DIR`, `GIT_WORK_TREE`, `GIT_PREFIX`, `NO_COLOR`.
- **Working directory requirements (GIT_PREFIX behavior):** Can be invoked from any repository subdirectory; operations are executed relative to repository root (`git rev-parse --show-toplevel`).
- **Git state assertions (e.g., dirty working tree check):**
  - Must be inside a Git working tree (`git rev-parse --is-inside-work-tree` == `true`).
  - Must NOT be in a shallow repository (`git rev-parse --is-shallow-repository` must be `false`).
  - Must NOT be in a detached HEAD state (`git symbolic-ref -q HEAD`).
  - Must NOT have an active in-flight rebase or merge (`.git/rebase-*` or `.git/MERGE_HEAD`).
  - Active branch prefix must be recognized (`issue/`, `feature/`, `epic/`, or configured mapping).

## 3. Core Execution Flow
1. **Repository & State Assertions:**
   - Verify worktree: `git rev-parse --is-inside-work-tree`.
   - Verify non-shallow repository: `git rev-parse --is-shallow-repository`. If true, fail with Exit Code `3`.
   - Verify attached branch: `git symbolic-ref --short -q HEAD`. Store current branch name.
   - Verify clean rebase/merge state: Assert absence of `.git/rebase-merge`, `.git/rebase-apply`, `.git/MERGE_HEAD`. If present, fail with Exit Code `5`.
2. **Strategy Determination & Validation:**
   - Extract branch prefix (e.g. `issue` from `issue/VSB-101`).
   - Default strategy:
     - `issue`: `rebase`.
     - `feature` or `epic`: `merge`.
     - Any other branch type without explicit `--merge`: Fail with Exit Code `3`.
   - Apply CLI overrides:
     - If both `--merge` and `--rebase` are supplied, fail with Exit Code `2` (`Multiple override sync strategies specified`).
     - If resolved strategy is `merge` and either `--autostash` or `--interactive` is set, fail with Exit Code `2` (`Options 'autostash/interactive' are rebase only`).
3. **Target Base Branch Resolution:**
   - If positional argument `<branch>` is omitted, default to `master` and log:
     `[git-brx] Syncing 'master' branch by default`.
4. **Remote Fetch & Upstream Delta Verification:**
   - Execute `git fetch origin`.
   - Calculate ahead/behind count for the base branch against its upstream tracking ref via:
     ```bash
     git rev-list --left-right --count <with_branch>...origin/<with_branch>
     ```
   - If either `<ahead> != 0` or `<behind> != 0`:
     - Log: `[git-brx] Error: Sync branch '<with_branch>' is not up-to-date`
     - Log: `[git-brx] Hint: Switch to '<with_branch>' branch and use 'git-brx update' to update all changes`
     - Fail with Exit Code `5`.
5. **Dry-Run Check:**
   - If `--dry-run` is active: Emit planned rebase or merge command string and exit with Exit Code `0`.
6. **Execution (Rebase vs Merge):**
   - **Rebase Path:**
     ```bash
     git rebase [--autostash] [--interactive] <with_branch>
     ```
   - **Merge Path:**
     ```bash
     git merge -m "Merge branch <with_branch> into <current_branch>" <with_branch>
     ```
7. **Conflict Interception:**
   - If rebase or merge fails due to conflicts:
     - Log: `[git-brx] Error: Sync failed due to merge conflicts`
     - Log: `[git-brx] Hint: Use 'git-brx resolve' in case of merge conflicts or 'git-brx reset' to abort`
     - Exit with Exit Code `5`.
   - Otherwise, log success and exit with Exit Code `0`.

## 4. Error Handling & Exit Codes
| Exit Code | Scenario | Emitted Stderr Pattern | Side Effect Cleanup |
| :--- | :--- | :--- | :--- |
| `0` | Branch successfully synchronized. | None (informational progress logged). | None. |
| `2` | Conflicting strategy flags (`--merge` + `--rebase`) or rebase flags on merge. | `[git-brx] Error: Multiple override sync strategies specified` | None. |
| `3` | Shallow repository, detached HEAD, or unpermitted branch type. | `[git-brx] Error: You are in a shallow repository` / `[git-brx] Error: You must be on 'issue', 'feature' or 'epic' branch` | None. |
| `4` | Fetch from origin failed. | `[git-brx] Error: Sync failed (git fetch failed)` | None. |
| `5` | In-flight rebase/merge active, base branch not up to date, or conflicts hit. | `[git-brx] Error: Sync branch '<branch>' is not up-to-date` / `[git-brx] Error: Sync failed due to merge conflicts` | Leave conflicted state for `git-brx resolve`. |

## 5. Discrepancies & Edge Cases Discovered
- **Documentation vs Script Discrepancy on Branch Types:** The legacy User's Guide explicitly states in Section 6.8 that `release maintenance` and `master` branches are permitted to sync using the `merge` strategy. However, the legacy Bash script `sync.sh` hardcoded a strict guard:
  ```bash
  if [[ "${branch_type}" != "feature" ]] && [[ "${branch_type}" != "issue" ]] && [[ "${branch_type}" != "epic" ]]; then
      branch::common::err "You must be on 'issue', 'feature' or 'epic' branch"
      exit 1
  fi
  ```
  This caused `sync.sh` to prematurely abort on `release/*` branches. The Go specification relaxes this to allow `merge` on any branch when explicitly configured or requested.
- **Race Condition in Delta Calculation:** Legacy script calculated base branch delta by piping to `/tmp/git_upstream_status_delta` and `/tmp/branch_delta`. The modern Go implementation calculates ahead/behind counts purely in-memory via `git rev-list --left-right --count`.
- **Refspec Parsing Flaw:** Legacy script used `cut -d'/' -f1` to extract branch type, failing if branch names contained nested slashes (e.g. `feature/subsystem/VSB-1`). The Go implementation splits on the first slash delimiter only.
