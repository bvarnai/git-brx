# Spec: git-brx publish

## 1. Command Signature
- **Usage:** `git-brx publish [flags]`
- **Arguments:** None.
- **Flags:**
  - Standard global flags: `--help` (`-h`), `--version`, `--verbose` (`-v`), `--quiet` (`-q`), `--no-color`.
  - Safety override: `--no-force`: Push with standard fast-forward constraints without `--force-with-lease`.

## 2. Prerequisites & Environment
- **Required host binaries:** `git` (>= 2.20.0).
- **Environment variables read:** `GIT_DIR`, `GIT_WORK_TREE`, `GIT_PREFIX`, `NO_COLOR`.
- **Working directory requirements (GIT_PREFIX behavior):** Can be invoked from any repository subdirectory; operations are executed relative to repository root (`git rev-parse --show-toplevel`).
- **Git state assertions (e.g., dirty working tree check):**
  - Must be inside a Git working tree (`git rev-parse --is-inside-work-tree` == `true`).
  - Must NOT be in a shallow repository (`git rev-parse --is-shallow-repository` must be `false`).
  - Must NOT be in a detached HEAD state (`git symbolic-ref -q HEAD`).
  - Remote `origin` must exist.
  - Protected branch check:
    - `main` / `master`: Force-push with lease is strictly forbidden (use `--no-force` or switch to a work branch).
    - `epic/*` / `release/*`: Always push fast-forward only (no `--force-with-lease`).

## 3. Core Execution Flow
1. **Repository & State Assertions:**
   - Verify worktree: `git rev-parse --is-inside-work-tree`.
   - Verify non-shallow repository: `git rev-parse --is-shallow-repository`. If true, fail with Exit Code `3`.
   - Verify attached branch: `git symbolic-ref --short -q HEAD`. Store current branch name `<branch>`.
   - Protected branch assertion:
     - If `<branch>` is `main` or `master` and `!opts.NoForce`: fail with Exit Code `2` (`Refusing to force-push with lease directly to protected '<branch>' branch`).
     - Determine `forceWithLease`:
       - For `epic/*` and `release/*`: `forceWithLease = false` (always fast-forward only to protect shared integration history under child branches).
       - For `issue/*`, `feature/*`, and custom branches: `forceWithLease = !opts.NoForce`.
2. **Upstream Tracking Evaluation:**
   - Inspect existing upstream configuration:
     ```bash
     git for-each-ref --format='%(upstream:short)' refs/heads/<branch>
     ```
   - Determine if upstream tracking matches `origin/<branch>`.
3. **Push Execution:**
   - Execute `git push [--set-upstream] origin <branch> [--force-with-lease]` according to upstream status and resolved `forceWithLease`.
4. **Lease / Non-Fast-Forward Rejection Interception:**
   - If push is rejected due to stale lease or non-fast-forward:
     - Log: `[git-brx] Error: Publish rejected: remote has newer commits`
     - Log: `[git-brx] Hint: Run 'git-brx update' to incorporate remote changes before publishing`
     - Fail with Exit Code `4`.
   - If push fails due to network or authentication failure:
     - Log: `[git-brx] Error: Unable to reach remote or authentication rejected`
     - Fail with Exit Code `4`.
5. **Completion:**
   - Log: `[git-brx] Branch '<branch>' successfully published to origin`.
   - Exit with Exit Code `0`.

## 4. Error Handling & Exit Codes
| Exit Code | Scenario | Emitted Stderr Pattern | Side Effect Cleanup |
| :--- | :--- | :--- | :--- |
| `0` | Branch pushed and tracking configured cleanly. | `[git-brx] Branch '<branch>' successfully published to origin` | None. |
| `2` | Unknown arguments or flags passed. | `[git-brx] Error: Unexpected argument: <arg>` | None. |
| `3` | Shallow repository or detached HEAD. | `[git-brx] Error: You are in a shallow repository` / `[git-brx] Error: You are in detached HEAD state` | None. |
| `4` | Push rejected by server (lease mismatch) or remote unreachable. | `[git-brx] Error: Publish rejected: remote has newer commits` / `[git-brx] Error: Unable to reach remote or authentication rejected` | None. |

## 5. Edge Cases & Behavioral Notes
- **Actionable Lease Rejection Diagnostics:** When a push is rejected due to a lease mismatch (`--force-with-lease`), `publish` detects the condition and provides a specific diagnostic message advising the user to run `update`.
- **Protected Default Branch Safety:** Explicit client-side safety checks block accidental force-pushing to default branches (`main`, `master`), requiring explicit branch operations.
- **Shared Integration Branch Guard:** `epic/*` and `release/*` always push fast-forward only in `git-brx publish` (never `--force-with-lease`) so child branches forked from them are never silently orphaned by an accidental rebase push.
- **Robust Argument Parsing:** Positional arguments and flags are parsed cleanly through native CLI structures, preventing unexpected word splitting.

## 6. Black-box verification scenarios
| Scenario ID | Given | When | Then (exit code, output, state) |
| :--- | :--- | :--- | :--- |
| `TC-PUB-01` | Local `epic/10` rewrote a commit relative to `origin/epic/10` | `git-brx publish` | Runs fast-forward `git push` (without `--force-with-lease`), fails with Exit Code `4` and `git-brx update` hint. |
| `TC-PUB-02` | Local `issue/10` rebased onto `main` after initial publish | `git-brx publish` / `git-brx publish --no-force` | Default uses `--force-with-lease` and succeeds (`0`); `--no-force` omits `--force-with-lease` and rejects non-FF (`4`). |
