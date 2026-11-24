# Spec: git-brx delete

## 1. Command Signature
- **Usage:** `git-brx delete [flags] [<branch>]`
- **Arguments:**
  - `[<branch>]` (optional): Name of the topic branch to delete. Defaults to the current active branch if omitted.
- **Flags:**
  - Standard global flags: `--help` (`-h`), `--version`, `--verbose` (`-v`), `--quiet` (`-q`), `--no-color`.
  - Feature flags:
    - `-f`, `--force`: Skip verification that the remote branch has already been deleted on origin.
    - `--delete-remote`: When the branch still exists on `origin` and the SCM API confirms its pull request is `MERGED`, delete the remote branch (`git push origin --delete <branch>`) before deleting locally.

## 2. Prerequisites & Environment
- **Required host binaries:** `git` (>= 2.20.0).
- **Environment variables read:** `GIT_DIR`, `GIT_WORK_TREE`, `GIT_PREFIX`, `NO_COLOR`.
- **Working directory requirements (GIT_PREFIX behavior):** Can be invoked from any repository subdirectory; operations are executed relative to repository root (`git rev-parse --show-toplevel`).
- **Git state assertions (e.g., dirty working tree check):**
  - Must be inside a Git working tree (`git rev-parse --is-inside-work-tree` == `true`).
  - Remote `origin` must be configured (`git remote get-url origin`).
  - If deleting current branch (no argument): Must NOT be in a detached HEAD state (`git symbolic-ref -q HEAD`).
  - Target branch MUST be a deletable topic branch (`issue/*`, `feature/*`, or `epic/*`). Deletion of `main`, `master`, or `release/*` is strictly forbidden.
  - If checked out on target branch: Working tree must be clean enough to permit switching back to default branch (`main`/`master`).

## 3. Core Execution Flow
1. **Repository & Working Tree Assertions:**
   - Verify worktree: `git rev-parse --is-inside-work-tree`.
   - Assert origin exists: `git remote get-url origin`.
   - Assert repository has commits: `git rev-parse --verify -q HEAD`.
2. **Resolve Target Branch:**
   - If argument provided:
     - Verify no excess arguments (`len(args) <= 1`).
     - Assert target branch exists locally: `git show-ref --verify --quiet refs/heads/<branch>`.
       If not found: Log `[git-brx] Error: Branch '<branch>' does not exist locally` and fail with Exit Code `3`.
     - Validate target branch prefix: `issue/*`, `feature/*`, or `epic/*`.
       If invalid: Log `[git-brx] Error: You can only delete an 'issue', 'feature', or 'epic' branch` and fail with Exit Code `3`.
   - If no argument provided:
     - Assert attached branch: `git symbolic-ref --short -q HEAD`.
       If detached: Log `[git-brx] Error: Cannot delete branch in detached HEAD state` and fail with Exit Code `3`.
     - Validate active branch prefix: `issue/*`, `feature/*`, or `epic/*`.
       If invalid: Log `[git-brx] Error: You must be on an 'issue', 'feature', or 'epic' branch to delete it` and fail with Exit Code `3`.
     - Set target branch to current branch.
3. **Remote Deletion Assertion (Safe Deletion Gate):**
   - Unless `--force` is active:
     - Check whether target branch still exists on the remote:
       ```bash
       git ls-remote --heads origin <target_branch>
       ```
     - If command fails due to network outage:
       Log `[git-brx] Error: Unable to reach remote; are you offline?` and fail with Exit Code `4`.
     - If ref output is non-empty (branch still exists on remote):
       - Log: `[git-brx] Branch '<target_branch>' found on remote origin`
       - **SCM-Aware Pull Request Diagnostic:**
         - Query configured SCM provider (`scm.GetPullRequestForBranch(ctx, target_branch)`). When multiple PRs exist for a reused branch name, prioritize `OPEN` > `MERGED` > `CLOSED`.
         - If `--delete-remote` is set and PR state is `MERGED`:
           - Execute `git push origin --delete <target_branch>` and log `[git-brx] Deleted remote branch 'origin/<target_branch>'`.
           - Proceed to step 4.
         - Otherwise, if an active, merged, or closed pull request is discovered, provide tailored status context and exit with Exit Code `3`:
           - **Merged:** `[git-brx] Pull request is merged (<url>), but the remote branch has not been deleted yet.` (hint: pass `--delete-remote` or `--force`).
           - **Open:** `[git-brx] Error: Pull request is still open (<url>). Merge or close it before deleting.` (never deleted even with `--delete-remote`).
           - **Closed:** `[git-brx] Pull request is closed without merge (<url>).`
       - Log: `[git-brx] Error: Branch must be deleted on remote first (e.g., after merging pull request)`
       - Log: `[git-brx] Hint: Use '--force' to bypass remote check if you intend to delete an unpublished branch`
       - Fail with Exit Code `3`.
4. **Switch to Parent or Base Branch (if currently on target branch):**
   - Resolve return branch `<return_branch>` in strict order:
     1. Recorded parent base (`git config --get branch.<target_branch>.brx-base`), if that branch still exists locally or on `origin`.
     2. Repository default base branch (`main`, `master`, or `origin/HEAD`).
   - If currently checked out on `<target_branch>`:
     - Check whether `<return_branch>` is already checked out in another linked worktree (`git worktree list --porcelain`) or absent locally (only `origin/<return_branch>` exists):
       - If checked out in another worktree or remote-only: detach `HEAD` onto `origin/<return_branch>` (or `<return_branch>`) via `git checkout --detach <ref>` and log `[git-brx] Branch '<return_branch>' is unavailable for checkout (checked out in another worktree or remote-only); detached HEAD before deleting '<target_branch>'`.
       - Otherwise: execute `git checkout <return_branch>`. If online, clean, and `<return_branch>` is strictly behind `origin/<return_branch>` (`AheadCount == 0 && BehindCount > 0`), auto-fast-forward it (`git merge --ff-only origin/<return_branch>`) and log `[git-brx] Fast-forwarded '<return_branch>' to match 'origin/<return_branch>' (<N> new commit(s))`.
     - If checkout fails (e.g., uncommitted local edits collide with base branch):
       - Log: `[git-brx] Error: Unable to switch to '<return_branch>' branch`
       - Fail with Exit Code `5`.
5. **Delete Local Branch:**
   - Force delete the local branch reference (which also removes `branch.<target_branch>.brx-base` from local Git config):
     ```bash
     git branch -D <target_branch>
     ```
6. **Remote Tracking Reference Pruning:**
   - Prune stale remote tracking refs:
     ```bash
     git remote prune origin
     ```
7. **Completion & Advice:**
   - Log: `[git-brx] Deleted local branch '<target_branch>' and pruned origin`.
   - If switched to `<return_branch>` and it was not already fast-forwarded, log `[git-brx] Hint: Your local '<return_branch>' may not be up-to-date. Use 'git-brx update' to update changes`.
   - Exit with Exit Code `0`.

## 4. Error Handling & Exit Codes
| Exit Code | Scenario | Emitted Stderr Pattern | Side Effect Cleanup |
| :--- | :--- | :--- | :--- |
| `0` | Switched to `main` (or default branch), deleted local branch, and pruned origin. | `[git-brx] Deleted local branch '<branch>' and pruned origin` | None. |
| `2` | Positional arguments or unknown flags supplied. | `[git-brx] Error: Unexpected argument: <arg>` | None. |
| `3` | On `main`/`master`, detached HEAD, or remote branch still exists on server. | `[git-brx] Error: Branch must be deleted on remote first` | None. |
| `4` | Remote unreachable during remote existence check. | `[git-brx] Error: Unable to reach remote; are you offline?` | None. |
| `5` | Switching to base branch failed or `git branch -D` failed. | `[git-brx] Error: Unable to switch to '<base_branch>' branch` | None. |

## 5. Edge Cases & Behavioral Notes
- **Accurate Remote Verification:** Remote branch existence and accessibility are verified using standard `git ls-remote --heads origin <branch>`, emitting clean diagnostic messages if unreachable or offline.
- **Default Branch Divergence Hint:** After deleting the topic branch and switching to default (`main` / `master`), `delete` emits a clear hint suggesting running `git-brx update` to fast-forward any newly squashed/merged remote commits.
- **Direct Git Branch Switching:** Switching to the parent/default branch is performed directly through internal Git operations rather than indirect command dispatch, with linked-worktree detach fallback.
- **SCM Pull Request Context Awareness:** If the remote branch still exists on the server, `delete` queries the SCM provider API (prioritizing `OPEN` > `MERGED` > `CLOSED` across reused branch names) and advises or executes `--delete-remote` when `MERGED`.

## 6. Black-box verification scenarios
| Scenario ID | Given | When | Then (exit code, output, state) |
| :--- | :--- | :--- | :--- |
| `TC-DEL-01` | Active `issue/44` with `branch.issue/44.brx-base=epic/10`, remote deleted, clean worktree | `git-brx delete` | Switches to `epic/10` (fast-forwarding if behind `origin/epic/10`), deletes `issue/44`, exits `0`. |
| `TC-DEL-02` | Linked worktree on `issue/44`; `main` checked out in primary worktree | `git-brx delete` | Detaches `HEAD` onto `origin/main` (or `main`), deletes `issue/44`, exits `0`. |
| `TC-DEL-03` | `issue/44` still exists on `origin` with PR `MERGED` vs. PR `OPEN` | `git-brx delete --delete-remote` | Deletes remote then local when `MERGED` (exit `0`); refuses and exits `3` when `OPEN`. |
| `TC-DEL-04` | Reused branch `issue/44` has an older `CLOSED` PR and a newer `OPEN` PR | `git-brx delete` | Reports `OPEN` PR status and blocks deletion with Exit Code `3`. |
