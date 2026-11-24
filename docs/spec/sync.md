# Spec: git-brx sync

## 1. Command Signature
- **Usage:** `git-brx sync [flags] [<branch>]`
- **Arguments:**
  - `<branch>` *(optional)*: Base branch to synchronize changes from. Defaults to recorded `branch.<current_branch>.brx-base` if set, otherwise `main` (or repository default).
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
   - Resolve target `<base>` in strict order:
     1. Explicit positional argument `<branch>`, if provided.
     2. Recorded parent base (`git config --get branch.<current_branch>.brx-base`), logging `[git-brx] Syncing '<current_branch>' with recorded base '<base>' (strategy: <strategy>)`.
     3. Repository default base branch (`main`, `master`, or `origin/HEAD`), logging `[git-brx] Syncing '<current_branch>' with '<base>' base branch by default (strategy: <strategy>)`.
   - If a recorded parent base no longer exists locally or on `origin` (after fetch in online mode), emit a warning and fall back to the repository default base branch.
4. **Remote Fetch, Worktree-Safe & Upstream Delta Verification:**
   - If `origin` exists, execute `git fetch origin` first so `origin/<base>` is current.
   - Determine local and remote existence of `<base>` (`refs/heads/<base>` and `refs/remotes/origin/<base>`).
   - If neither local `<base>` nor `origin/<base>` exists, fail with Exit Code `3` (`Base branch '<base>' does not exist locally or on origin`).
   - **Remote-only base fallback:** If `<base>` does NOT exist locally but `origin/<base>` exists, set `<sync_ref>` to `origin/<base>` and proceed without failing.
   - **Local base exists & remote exists:** Calculate ahead/behind count via `git rev-list --left-right --count <base>...origin/<base>`:
     - **Diverged or Unpushed Commits (`<ahead> > 0`):**
       - If `<behind> == 0`: hint `Switch to '<base>' and run 'git-brx publish' first`.
       - If `<behind> > 0`: hint `Switch to '<base>', run 'git-brx update', and then 'git-brx publish' first`.
       - Fail with Exit Code `5`.
     - **Clean Base Behind Origin (`<ahead> == 0 && <behind> > 0`):**
       - If `--no-auto-update-base` is specified: fail with Exit Code `5` and hint `Switch to '<base>' and run 'git-brx update' first`.
       - If `<base>` is checked out in another linked worktree (`git worktree list --porcelain`): do NOT fail `FetchRef`; log `[git-brx] Base branch '<base>' is checked out in another worktree; syncing directly against 'origin/<base>'` and set `<sync_ref>` to `origin/<base>`.
       - Otherwise fast-forward local base ref without checking it out:
         ```bash
         git fetch origin <base>:<base>
         ```
         and log `[git-brx] Fast-forwarded local '<base>' to match 'origin/<base>' (<behind> commit(s) behind)`.
5. **Execution (Rebase vs Merge):**
   - **Rebase Path:**
     ```bash
     git rebase [--autostash] [--interactive] <sync_ref>
     ```
   - **Merge Path:**
     ```bash
     git merge -m "Merge branch '<sync_ref>' into '<current_branch>'" <sync_ref>
     ```
6. **Post-Operation Completion & Conflict Interception:**
   - If rebase or merge fails due to conflicts:
     - Log: `[git-brx] Error: Sync failed due to merge conflicts`
     - Log: `[git-brx] Hint: Use 'git-brx resolve' to resolve conflicts, or 'git-brx reset' to abort without losing commits`
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
| `4` | Fetch from origin failed. | `[git-brx] Error: Unable to fetch from origin` | None. |
| `5` | In-flight rebase/merge active, base branch not up to date, or conflicts hit. | `[git-brx] Error: Sync branch '<branch>' is not up-to-date` / `[git-brx] Error: Sync failed due to merge conflicts` | Leave conflicted state for `git-brx resolve`. |

## 5. Edge Cases & Behavioral Notes
- **Flexible Branch Type Strategy Resolution:** Permits syncing on any branch type (including `release/*` or custom prefixes) using configured or explicitly requested strategies (`merge` or `rebase`).
- **In-Memory Upstream Delta Calculation:** Ahead/behind commit counts against upstream and base branches are computed purely in-memory via `git rev-list --left-right --count`, avoiding temporary files and race conditions.
- **Multi-Slash Refspec Parsing:** Branch types are parsed by splitting strictly on the first slash delimiter (e.g., `feature/subsystem/ticket-1` correctly extracts type `feature`), safely handling hierarchically structured branch names.

## 6. Black-box verification scenarios
| Scenario ID | Given | When | Then (exit code, output, state) |
| :--- | :--- | :--- | :--- |
| `TC-SYN-01` | On `issue/44` with `branch.issue/44.brx-base=epic/10` | `git-brx sync` | Syncs `issue/44` against `epic/10` (strategy: `rebase`) without requiring `epic/10` argument, exits `0`. |
| `TC-SYN-02` | `origin/epic/10` exists on remote, no local `epic/10` branch | `git-brx sync epic/10` | Fetches `origin` and syncs directly against `origin/epic/10`, exits `0`. |
| `TC-SYN-03` | `epic/10` is checked out in another linked worktree and behind `origin/epic/10` | `git-brx sync epic/10` | Logs worktree fallback notice and syncs against `origin/epic/10`, exits `0`. |
| `TC-SYN-04` | Local `main` has 1 unpushed commit (`ahead=1, behind=0`) | `git-brx sync` | Exits `5` with hint `Switch to 'main' and run 'git-brx publish' first`. |
