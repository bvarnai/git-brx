# Spec: git-brx select

## 1. Command Signature
- **Usage:** `git-brx select [flags] [<branch>]`
- **Arguments:**
  - `<branch>` *(optional)*: Target branch to switch to. Defaults to `main` (or repository default) if omitted.
- **Flags:**
  - Standard global flags: `--help` (`-h`), `--version`, `--verbose` (`-v`), `--quiet` (`-q`), `--no-color`.
  - Feature flags:
    - `--offline` (`-o`): Skip remote fetch and switch between existing local references only.
    - `--no-ff`: Skip automatic fast-forward when the selected branch is strictly behind `origin/<branch>`.

## 2. Prerequisites & Environment
- **Required host binaries:** `git` (>= 2.20.0).
- **Environment variables read:** `GIT_DIR`, `GIT_WORK_TREE`, `GIT_PREFIX`, `NO_COLOR`.
- **Working directory requirements (GIT_PREFIX behavior):** Can be invoked from any repository subdirectory; operations are executed relative to repository root (`git rev-parse --show-toplevel`).
- **Git state assertions (e.g., dirty working tree check):**
  - Must be inside a Git working tree (`git rev-parse --is-inside-work-tree` == `true`).
  - In-progress operation must NOT be active: `.git/rebase-merge`, `.git/rebase-apply`, `.git/MERGE_HEAD`, `.git/CHERRY_PICK_HEAD`, `.git/REVERT_HEAD`, or `.git/BISECT_LOG` must not exist.
  - Switching away from detached HEAD must not orphan unreferenced commits.
  - Dirty working tree modifications must not collide with changes introduced by the target branch.

## 3. Core Execution Flow
1. **Repository & State Verification:**
   - Verify repository context: `git rev-parse --is-inside-work-tree`.
   - Check in-progress operations via `checks.CheckInProgress`. If active, log `[git-brx] Error: Cannot switch branches during an active <op>` and fail with Exit Code `5`.
   - Check detached HEAD via `checks.CheckDetachedHead`. If detached with unreferenced commits, log `[git-brx] Error: You are on a detached HEAD with unreferenced commits` and fail with Exit Code `5`.
2. **Target Branch Resolution:**
   - If positional argument `<branch>` is absent, dynamically resolve default branch (`main` -> `master` -> `origin/HEAD`) and log:
     `[git-brx] Selecting '<default>' branch by default`.
   - Otherwise, set target to specified argument `<branch>`.
3. **Already on Target Branch Check:**
   - If the current branch matches target branch:
     - If online, fetch `origin`.
     - Log: `[git-brx] Already on '<branch>'`.
     - Run step 6.5 (Fast-Forward & Upstream Sync Check) and exit with Exit Code `0`.
4. **Remote Synchronization (Online Mode):**
   - If `--offline` is not specified:
     - Execute `git fetch origin`.
     - *Note:* If remote fetch fails due to network outage, log warning `[git-brx] Warning: Unable to reach remote; falling back to local references` and continue if the branch exists locally, or fail with Exit Code `4` if the branch exists only remotely.
5. **Branch Checkout Execution:**
   - Verify branch existence:
     - Check local heads: `git show-ref --verify --quiet refs/heads/<branch>`.
     - Check remote tracking: `git show-ref --verify --quiet refs/remotes/origin/<branch>`.
   - If neither exists:
     - Inspect available branch names and suggest closest match via Levenshtein distance:
       `[git-brx] Error: Branch '<branch>' not found`
       `[git-brx] Hint: Did you mean '<closest>'?` (if match found within threshold).
     - Exit with Exit Code `3`.
   - Execute branch switch:
     ```bash
     git checkout <branch>
     ```
6. **Post-Checkout Verification:**
   - If checkout fails due to uncommitted working tree conflicts:
     - Log: `[git-brx] Error: Select failed: local modifications would be overwritten by checkout`
     - Log: `[git-brx] Hint: Commit or stash your changes before switching branches`
     - Exit with Exit Code `5`.
   - If uncommitted changes were carried over to the new branch, log defensive warning via `checks.CheckDirtyWorktree`:
     - Log: `[git-brx] Warning: You have uncommitted local changes that were carried over to '<branch>'.`
     - Log: `[git-brx] Hint: If this was unintentional, run 'git-brx select <prevBranch>' and commit or stash first.`
   - **6.5. Fast-Forward & Upstream Sync Check (both on switch and when already on target):**
     - In online mode when `origin/<branch>` exists:
       - Compute delta via `ComputeDelta(<branch>, origin/<branch>)`.
       - If `AheadCount == 0 && BehindCount > 0` and `--no-ff` is NOT set and working tree is clean (`!IsDirty`):
         - Execute `git merge --ff-only origin/<branch>`.
         - Log: `[git-brx] Fast-forwarded '<branch>' to match 'origin/<branch>' (<N> new commit(s))`.
       - Else if `BehindCount > 0`:
         - Always emit `[git-brx] Hint: Your branch is behind 'origin/<branch>' by <N> commit(s). Run 'git-brx update' to update.` (applies uniformly to default, `epic/*`, `feature/*`, and `issue/*` branches).
   - Exit with Exit Code `0`.

## 4. Error Handling & Exit Codes
| Exit Code | Scenario | Emitted Stderr Pattern | Side Effect Cleanup |
| :--- | :--- | :--- | :--- |
| `0` | Switched successfully or already on target branch. | `[git-brx] Selecting '<default>' branch by default` / `[git-brx] Already on '<branch>'` | None. |
| `2` | Excess positional arguments (>1) or unknown flags. | `[git-brx] Error: Unexpected argument: <arg>` | None. |
| `3` | Outside repository or branch not found locally or remotely. | `[git-brx] Error: Branch '<branch>' not found` (with optional `Hint: Did you mean...?`) | None. |
| `4` | Mandatory remote fetch failed and branch not found locally. | `[git-brx] Error: Select failed: unable to fetch from origin` | None. |
| `5` | Active operation in progress, detached HEAD commit loss risk, or uncommitted collision. | `[git-brx] Error: Cannot switch branches during an active <op>` / `! You are on a detached HEAD...` | None. |

## 5. Edge Cases & Behavioral Notes
- **Offline & Network Resiliency:** Supports `--offline` to switch between existing local branches without network roundtrips, and provides automatic graceful fallback when the remote is unreachable.
- **In-Progress Operation Guards:** Inspects repository state before checkout to prevent switching branches during active rebases, merges, cherry-picks, or reverts.
- **Dirty Worktree Warnings:** Emits clear warning diagnostics when uncommitted changes exist in the working directory during checkout to prevent accidental cross-branch edits.
- **Detached HEAD Protection:** Prevents switching away from detached HEAD when unreferenced commits exist, prompting the user to preserve work in a named branch.
- **Dynamic Default Branch Resolution:** Dynamically detects repository default branch (`main`, `master`, or `origin/HEAD`) rather than assuming hardcoded defaults.
- **Fuzzy Branch Name Suggestions:** Suggests closest matching existing branch names when a typo occurs.

## 6. Black-box verification scenarios
| Scenario ID | Given | When | Then (exit code, output, state) |
| :--- | :--- | :--- | :--- |
| `TC-SEL-01` | Clean local `epic/10` is 2 commits behind `origin/epic/10` (ahead=0) | `git-brx select epic/10` | Switches to `epic/10`, fast-forwards via `--ff-only`, logs `Fast-forwarded 'epic/10' to match 'origin/epic/10'`, exits `0`. |
| `TC-SEL-02` | Clean local `main` is behind `origin/main` | `git-brx select main --no-ff` | Skips fast-forward, emits `Run 'git-brx update' to update` hint, exits `0`. |
| `TC-SEL-03` | Local `epic/10` has diverged (ahead=1, behind=1) from `origin/epic/10` | `git-brx select epic/10` | Emits `Run 'git-brx update' to update` hint (never `git-brx sync`), exits `0`. |
| `TC-SEL-04` | Dirty worktree on `main`, switching to `issue/20` which is behind `origin/issue/20` | `git-brx select issue/20` | Skips fast-forward, emits dirty-worktree warning and `git-brx update` hint, exits `0`. |
