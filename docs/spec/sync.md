# Spec: git-brx sync

## 1. Command Signature
- **Usage:** `git-brx sync [flags] [<branch>]`
- **Arguments:**
  - `<branch>` *(optional)*: Base branch to synchronize changes from. Defaults to `main` (or repository default) if omitted.
- **Flags:**
  - Standard global flags: `--help` (`-h`), `--version`, `--verbose` (`-v`), `--quiet` (`-q`), `--no-color`.
  - Strategy flags:
    - `-m`, `--merge`: Force merge strategy regardless of branch type default.
    - `-r`, `--rebase`: Force rebase strategy regardless of branch type default.
    - `-a`, `--autostash`: Enable automatic stashing of uncommitted changes (rebase only).
    - `-i`, `--interactive`: Launch interactive rebase (rebase only).
    - `--no-auto-update-base`: Disable automatic fast-forwarding of clean base branch.

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
     - If resolved strategy is `merge` and either `--autostash` or `--interactive` is set, fail with Exit Code `2` (`Flags --autostash and --interactive are only supported with rebase strategy`).
3. **Target Base Branch Resolution & Strategy Logging:**
   - If positional argument `<branch>` is omitted, default to repository base branch (e.g. `main`) and log:
     `[git-brx] Syncing '<current_branch>' with '<base>' base branch by default (strategy: <strategy>)`.
   - If positional argument `<branch>` is explicitly supplied, log:
     `[git-brx] Syncing '<current_branch>' with '<base>' (strategy: <strategy>)`.
4. **Remote Fetch & Upstream Delta Verification:**
   - Execute `git fetch origin`.
   - Calculate ahead/behind count for the base branch against its upstream tracking ref via:
     ```bash
     git rev-list --left-right --count <with_branch>...origin/<with_branch>
     ```
   - **Diverged or Unpushed Commits (`<ahead> > 0`):**
     - Log: `[git-brx] Error: Base branch '<with_branch>' is not up-to-date with 'origin/<with_branch>'`
     - Log: `[git-brx] Hint: Switch to '<with_branch>' and run 'git-brx update' first`
     - Fail with Exit Code `5`.
   - **Clean Base Behind Origin (`<ahead> == 0 && <behind> > 0`):**
     - If `--no-auto-update-base` is specified:
       - Log: `[git-brx] Error: Base branch '<with_branch>' is not up-to-date with 'origin/<with_branch>'`
       - Log: `[git-brx] Hint: Switch to '<with_branch>' and run 'git-brx update' first`
       - Fail with Exit Code `5`.
     - Otherwise:
       - Fast-forward local base ref without checking it out:
         ```bash
         git fetch origin <with_branch>:<with_branch>
         ```
       - Log: `[git-brx] Fast-forwarded local '<with_branch>' to match 'origin/<with_branch>' (<behind> commit(s) behind)`.
       - Proceed with synchronization.
5. **Execution (Rebase vs Merge):**
   - **Rebase Path:**
     ```bash
     git rebase [--autostash] [--interactive] <with_branch>
     ```
   - **Merge Path:**
     ```bash
     git merge -m "Merge branch <with_branch> into <current_branch>" <with_branch>
     ```
6. **Post-Operation Completion & Conflict Interception:**
   - If rebase or merge fails due to conflicts:
     - Log: `[git-brx] Error: Sync failed due to merge conflicts`
     - Log: `[git-brx] Hint: Use 'git-brx resolve' in case of merge conflicts or 'git-brx reset' to abort`
     - Exit with Exit Code `5`.
   - If branch is already up to date:
     - Log: `[git-brx] Branch '<current_branch>' is already up to date with '<base>'`
     - Exit with Exit Code `0`.
   - Otherwise, on successful completion:
     - Rebase: Log `[git-brx] Successfully rebased '<current_branch>' onto '<base>'`
     - Merge: Log `[git-brx] Successfully merged '<base>' into '<current_branch>'`
     - Exit with Exit Code `0`.

## 4. Error Handling & Exit Codes
| Exit Code | Scenario | Emitted Stderr Pattern | Side Effect Cleanup |
| :--- | :--- | :--- | :--- |
| `0` | Branch successfully synchronized. | None (informational progress logged). | None. |
| `2` | Conflicting strategy flags (`--merge` + `--rebase`) or rebase flags on merge. | `[git-brx] Error: Multiple override sync strategies specified` | None. |
| `3` | Shallow repository, detached HEAD, or unpermitted branch type. | `[git-brx] Error: You are in a shallow repository` / `[git-brx] Error: You must be on 'issue', 'feature' or 'epic' branch` | None. |
| `4` | Fetch from origin failed. | `[git-brx] Error: Sync failed (git fetch failed)` | None. |
| `5` | In-flight rebase/merge active, base branch not up to date, or conflicts hit. | `[git-brx] Error: Sync branch '<branch>' is not up-to-date` / `[git-brx] Error: Sync failed due to merge conflicts` | Leave conflicted state for `git-brx resolve`. |

## 5. Edge Cases & Behavioral Notes
- **Flexible Branch Type Strategy Resolution:** Permits syncing on any branch type (including `release/*` or custom prefixes) using configured or explicitly requested strategies (`merge` or `rebase`).
- **In-Memory Upstream Delta Calculation:** Ahead/behind commit counts against upstream and base branches are computed purely in-memory via `git rev-list --left-right --count`, avoiding temporary files and race conditions.
- **Multi-Slash Refspec Parsing:** Branch types are parsed by splitting strictly on the first slash delimiter (e.g., `feature/subsystem/ticket-1` correctly extracts type `feature`), safely handling hierarchically structured branch names.
