# Spec: git-brx delete

## 1. Command Signature
- **Usage:** `git-brx delete [flags] [<branch>]`
- **Arguments:**
  - `[<branch>]` (optional): Name of the topic branch to delete. Defaults to the current active branch if omitted.
- **Flags:**
  - Standard global flags: `--help` (`-h`), `--version`, `--verbose` (`-v`), `--quiet` (`-q`), `--dry-run` (`-n`), `--no-color`.
  - Override flag: `--force` (`-f`): Skip verification that the remote branch has already been deleted on origin.

## 2. Prerequisites & Environment
- **Required host binaries:** `git` (>= 2.20.0).
- **Environment variables read:** `GIT_DIR`, `GIT_WORK_TREE`, `GIT_PREFIX`, `NO_COLOR`.
- **Working directory requirements (GIT_PREFIX behavior):** Can be invoked from any repository subdirectory; operations are executed relative to repository root (`git rev-parse --show-toplevel`).
- **Git state assertions (e.g., dirty working tree check):**
  - Must be inside a Git working tree (`git rev-parse --is-inside-work-tree` == `true`).
  - Remote `origin` must be configured (`git remote get-url origin`).
  - If deleting current branch (no argument): Must NOT be in a detached HEAD state (`git symbolic-ref -q HEAD`).
  - Target branch MUST be a deletable topic branch (`issue/*`, `feature/*`, or `epic/*`). Deletion of `master`, `main`, or `release/*` is strictly forbidden.
  - If checked out on target branch: Working tree must be clean enough to permit switching back to default branch (`master`/`main`).

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
         - Query configured SCM provider (`scm.GetPullRequestForBranch(ctx, target_branch)`).
         - If an active or merged pull request is discovered, provide tailored status context:
           - **Merged:** `[git-brx] Pull request is merged (<url>), but the remote branch has not been deleted yet.`
           - **Open:** `[git-brx] Error: Pull request is still open (<url>). Merge or close it before deleting.`
           - **Closed:** `[git-brx] Pull request is closed without merge (<url>).`
       - Log: `[git-brx] Error: Branch must be deleted on remote first (e.g., after merging pull request)`
       - Log: `[git-brx] Hint: Use '--force' to bypass remote check if you intend to delete an unpublished branch`
       - Fail with Exit Code `3`.
4. **Dry-Run Check:**
   - Resolve base branch (`master` or `main`).
   - Determine if branch switch is required (true if currently on target branch).
   - If `--dry-run` is active:
     - If switch required: Log `[git-brx] Would checkout branch '<base_branch>'`.
     - Log `[git-brx] Would delete local branch '<target_branch>'`.
     - Log `[git-brx] Would prune stale tracking references from origin`.
     - Exit with Exit Code `0`.
5. **Switch to Base Branch (if currently on target branch):**
   - If currently checked out on target branch:
     - Execute:
       ```bash
       git checkout <base_branch>
       ```
     - If checkout fails (e.g., uncommitted local edits collide with base branch):
       - Log: `[git-brx] Error: Unable to switch to '<base_branch>' branch`
       - Fail with Exit Code `5`.
6. **Delete Local Branch:**
   - Force delete the local branch reference:
     ```bash
     git branch -D <target_branch>
     ```
7. **Remote Tracking Reference Pruning:**
   - Prune stale remote tracking refs:
     ```bash
     git remote prune origin
     ```
8. **Completion & Advice:**
   - Log: `[git-brx] Deleted local branch '<target_branch>' and pruned origin`.
   - Log: `[git-brx] Hint: Your local '<base_branch>' may not be up-to-date. Use 'git-brx update' to update changes`.
   - Exit with Exit Code `0`.

## 4. Error Handling & Exit Codes
| Exit Code | Scenario | Emitted Stderr Pattern | Side Effect Cleanup |
| :--- | :--- | :--- | :--- |
| `0` | Switched to `master`, deleted local branch, and pruned origin. | `[git-brx] Deleted local branch '<branch>' and pruned origin` | None. |
| `2` | Positional arguments or unknown flags supplied. | `[git-brx] Error: Unexpected argument: <arg>` | None. |
| `3` | On `master`, detached HEAD, or remote branch still exists on server. | `[git-brx] Error: Branch must be deleted on remote first` | None. |
| `4` | Remote unreachable during remote existence check. | `[git-brx] Error: Unable to reach remote; are you offline?` | None. |
| `5` | Switching to `master` failed or `git branch -D` failed. | `[git-brx] Error: Unable to switch to 'master' branch` | None. |

## 5. Discrepancies & Edge Cases Discovered
- **Typo in Legacy Error Message:** In `legacy/delete.sh` line 34:
  `branch::common::err "Unable to reach remote, are you offline? (gitish: 'git ls-remote origin -D ${GIT_PROMPT_BRANCH}' failed)"`
  The string contains a phantom `-D` flag that does not exist in `git ls-remote`. Fixed in modern spec.
- **Master Divergence After Deletion:** Legacy deletes the branch and switches to `master`, but does not pull latest `master` commits, leaving `master` out-of-date after a PR squash. The spec emits a clear hint to run `update`.
- **Recursive Command Dispatch:** Legacy executed `git branch-select master` (invoking another shell script via Git alias) rather than invoking `git checkout master` directly. Modern Go calls internal Git checkout logic directly.
- **SCM Pull Request Context:** Legacy script gave an unhelpful generic error if the remote branch still existed. Modern Go queries the SCM provider API (GitHub, Bitbucket) and advises whether the PR is already merged, still open, or closed, guiding the user to cleanly delete the remote branch or use `--force`.
