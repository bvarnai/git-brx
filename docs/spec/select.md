# Spec: git-brx select

## 1. Command Signature
- **Usage:** `git-brx select [flags] [<branch>]`
- **Arguments:**
  - `<branch>` *(optional)*: Target branch to switch to. Defaults to `master` if omitted.
- **Flags:**
  - Standard global flags: `--help` (`-h`), `--version`, `--verbose` (`-v`), `--quiet` (`-q`), `--dry-run` (`-n`), `--no-color`.
  - Feature flags:
    - `--offline` (`-o`): Skip remote fetch and switch between existing local references only.
    - `--no-fetch`: Do not trigger remote synchronization before checkout.

## 2. Prerequisites & Environment
- **Required host binaries:** `git` (>= 2.20.0).
- **Environment variables read:** `GIT_DIR`, `GIT_WORK_TREE`, `GIT_PREFIX`, `NO_COLOR`.
- **Working directory requirements (GIT_PREFIX behavior):** Can be invoked from any repository subdirectory; operations are executed relative to repository root (`git rev-parse --show-toplevel`).
- **Git state assertions (e.g., dirty working tree check):**
  - Must be inside a Git working tree (`git rev-parse --is-inside-work-tree` == `true`).
  - Active rebase or merge must NOT be in progress (`.git/rebase-merge`, `.git/rebase-apply`, or `.git/MERGE_HEAD` must not exist).
  - Dirty working tree modifications must not collide with changes introduced by the target branch.

## 3. Core Execution Flow
1. **Repository & State Verification:**
   - Verify repository context: `git rev-parse --is-inside-work-tree`.
   - Check for in-flight rebase/merge. If present, log `[git-brx] ! Cannot switch branches during an active merge/rebase` and fail with Exit Code `5`.
2. **Target Branch Resolution:**
   - If positional argument `<branch>` is absent, set target to `master` and log:
     `[git-brx] Selecting 'master' branch by default`.
   - Otherwise, set target to specified argument `<branch>`.
3. **Remote Synchronization (Online Mode):**
   - If neither `--offline` nor `--no-fetch` is specified:
     - Execute `git fetch origin`.
     - *Note:* If remote fetch fails due to network outage, log warning `[git-brx] Warning: Unable to reach remote; falling back to local references` and continue if the branch exists locally, or fail with Exit Code `4` if the branch exists only remotely.
4. **Dry-Run Check:**
   - If `--dry-run` is active: Validate branch existence locally or on remote, verify working tree safety, log planned switch, and exit with Exit Code `0`.
5. **Branch Checkout Execution:**
   - Verify branch existence:
     - Check local heads: `git show-ref --verify --quiet refs/heads/<branch>`.
     - Check remote tracking: `git show-ref --verify --quiet refs/remotes/origin/<branch>`.
   - If neither exists, log: `[git-brx] ! Branch '<branch>' not found` and exit with Exit Code `3`.
   - Execute branch switch:
     ```bash
     git checkout <branch>
     ```
6. **Completion:**
   - If checkout fails due to uncommitted working tree conflicts:
     - Log: `[git-brx] ! Select failed: local modifications would be overwritten by checkout`
     - Log: `[git-brx] Hint: Commit or stash your changes before switching branches`
     - Exit with Exit Code `5`.
   - Otherwise, emit confirmation and exit with Exit Code `0`.

## 4. Error Handling & Exit Codes
| Exit Code | Scenario | Emitted Stderr Pattern | Side Effect Cleanup |
| :--- | :--- | :--- | :--- |
| `0` | Switched successfully to target branch. | `[git-brx] Selecting 'master' branch by default` (if default) | None. |
| `2` | Excess positional arguments (>1) or unknown flags. | `[git-brx] ! Unexpected argument: <arg>` | None. |
| `3` | Outside repository or branch not found locally or remotely. | `[git-brx] ! Branch '<branch>' not found` | None. |
| `4` | Mandatory remote fetch failed and branch not found locally. | `[git-brx] ! Select failed (git fetch failed)` | None. |
| `5` | Active merge/rebase in progress or uncommitted changes prevent checkout. | `[git-brx] ! Cannot switch branches during an active merge/rebase` / `[git-brx] ! Select failed: local modifications would be overwritten` | None. |

## 5. Discrepancies & Edge Cases Discovered
- **Hard Dependency on Network (`git fetch --all`):** Legacy `select.sh` unconditionally executed `git fetch --all`. If a developer was on an airplane or disconnected from VPN, switching between existing local branches failed completely! The modern Go spec introduces `--offline` and network failure fallbacks for local branches.
- **Uncontrolled `fetch --all` Performance Drag:** Fetching *all* remotes on large enterprise repositories is slow and unnecessary. Modern spec restricts fetch to `origin` or tracking remote.
- **In-Flight Rebase Ignored:** Legacy `select.sh` had no precondition check against in-flight rebases or merges, allowing developers to switch branches and leave orphaned `.git/rebase-*` state behind. Modern spec enforces an explicit check.
