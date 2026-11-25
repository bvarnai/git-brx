# Spec: git-brx create

## 1. Command Signature
- **Usage:** `git-brx create [flags] <branch>`
- **Arguments:**
  - `<branch>` *(mandatory)*: Name of the work branch to create (e.g., `issue/VSB-1234`, `feature/VSB-5678`).
- **Flags:**
  - Standard global flags: `--help` (`-h`), `--version`, `--verbose` (`-v`), `--quiet` (`-q`), `--no-color`.
  - Feature flags:
    - `-b`, `--base` `<branch>`: Explicit base branch to fork from and record in `branch.<branch>.brx-base`.
    - `-o`, `--offline`: Skip issue tracker validation and remote Git availability checks.
    - `-y`, `--yes`: Automatically accept confirmation prompts without interactive input.

## 2. Prerequisites & Environment
- **Required host binaries:** `git` (>= 2.20.0).
- **Environment variables read:** `GIT_DIR`, `GIT_WORK_TREE`, `GIT_PREFIX`, `GIT_BRX_CONFIG`, `GITHUB_TOKEN`, `GH_TOKEN`, `GIT_BRX_TOKEN`, `GIT_BRX_COLOR`, `NO_COLOR`.
- **Working directory requirements (GIT_PREFIX behavior):** Can be invoked from any repository subdirectory; operations are executed relative to repository root (`git rev-parse --show-toplevel`).
- **Git state assertions (e.g., dirty working tree check):**
  - Must be inside a Git working tree (`git rev-parse --is-inside-work-tree` == `true`).
  - Active rebase or merge must NOT be in progress.
  - Working tree should be clean; uncommitted changes will be carried over to the new branch if safe, or will trigger a Git checkout failure if conflicting.

## 3. Core Execution Flow
1. **Argument & Repository Assertions:**
   - Verify argument `<branch>` is provided. If missing, log `[git-brx] Error: No branch name specified` and exit with Exit Code `2`.
   - Verify repository context: `git rev-parse --is-inside-work-tree`.
2. **Local & Remote Branch Collision Auto-Switch:**
   - Check if `<branch>` already exists locally (`git show-ref --verify --quiet refs/heads/<branch>`), or on remote in online mode (`git ls-remote --heads origin <branch>`).
   - If remote is unreachable in online mode, log `[git-brx] Error: Unable to reach remote; try --offline if working without network access` and fail with Exit Code `4`.
   - If `<branch>` exists locally or on `origin`:
     - Enforce `select` preflight safety guards (`NoOrphanedCommits`, checkout collision handling, and dirty-worktree carryover warning; see `docs/spec/select.md`).
     - If remote-only, fetch `origin` first (`git fetch origin <branch>`).
     - Switch to `<branch>` (`git checkout <branch>`), auto-fast-forwarding if clean and strictly behind `origin/<branch>`.
     - Log: `[git-brx] Branch '<branch>' already exists; switched to '<branch>'`
     - Exit with Exit Code `0`.
4. **Configuration & Template Matching:**
   - Load configuration via the layered cascade (User Global `~/.config/git-brx/config.yaml` / `~/.git-brx.yaml` -> Shared Global `GIT_BRX_CONFIG` -> Repo Local `.git-brx.yaml` / `.git-brx/config.yaml` -> zero-config remote origin auto-discovery). If invalid, exit with Exit Code `8`.
   - Compile regular expression template from `config.branch.template` (interpolating `config.tracker.project` if set).
   - Match `<branch>` against the template. Extract:
     - `branchPath` (e.g., `issue`, `feature`, `epic`)
     - `issueKey` (e.g., `VSB-1234` or `#42` or `42`)
   - If regex does not match, log `[git-brx] Error: Branch name '<branch>' doesn't match pattern <pattern>` and exit with Exit Code `8`.
5. **Issue Tracker Validation (Online Mode):**
   - If `--offline` is NOT set:
     - Fetch issue details using the configured `IssueTracker` adapter (GitHub Issues, Jira REST, etc.).
     - If authorization fails: Fail with Exit Code `6` (`Authorization failure`).
     - If issue not found: Fail with Exit Code `6` (`Issue not found in issue tracker`).
     - Check issue type & direct label mapping:
       - Inspect issue labels (`issue.Components`) against `config.branch.mapping` (case-insensitively). Direct label matches take precedence over domain types.
       - If no direct label matches, check domain type mapping `config.branch.mapping[issue.Type]` (case-insensitively).
       - Values in `branch.mapping` support single strings (`"issue"`), comma-separated strings (`"issue, feature"`), or YAML sequences (`[issue, feature]`).
       - If the branch path is not among the allowed branch targets: Log `[git-brx] Error: Issue type '<type>' is not allowed on '<branchPath>' branch` with hint (`Use '<targets>' branch or check mapping rules`) and fail with Exit Code `8`.
       - If unmapped: Unmapped issue types are allowed on generic `issue/` branch, but restricted from specialized branches with Exit Code `8`.
     - If issue status is closed/resolved, display warning:
       `[git-brx] Warning: Issue '<issueKey>' is currently marked as '<status>'`
     - Confirmation prompt (unless `--yes` is specified):
       `Create branch '<branch>' [Y/n]?` (defaults to Yes on Enter).
       - If rejected (`n` / `N`), exit with Exit Code `7`.
6. **Base Branch Guardrails & Branch Provisioning:**
   - **Base resolution & validation:**
     - If `--base <base>` is specified: verify `<base>` exists locally or as `origin/<base>` (fetching if online). If missing, fail with Exit Code `3`. If local `<base>` is clean and strictly behind `origin/<base>`, fast-forward it (or fork directly from `origin/<base>` if `<base>` is checked out in another linked worktree).
     - If `--base` is omitted: resolve the currently attached branch (or default branch if unborn/detached) as `<base>`.
       - If the active branch is `issue/*` or `feature/*`, emit `[git-brx] Warning: Creating '<branch>' from '<current>' instead of default branch` with hint `Pass '--base main' or run 'git-brx select main' first if this was unintentional`.
       - If online and the active branch is behind `origin/<current>`, emit `[git-brx] Warning: Base branch '<current>' is behind 'origin/<current>'` with hint `Run 'git-brx update' before branching if you need the latest commits`.
   - **Dirty worktree warning:** If uncommitted local modifications exist (`git diff-index --quiet HEAD --` or untracked tracked-collision risk), emit `[git-brx] Warning: Uncommitted local changes will be carried over to '<branch>'`.
   - **Checkout & parent tracking:**
     - Execute checkout to new branch:
       ```bash
       git checkout -b <branchPath>/<issueKey> [<start_point>]
       ```
     - Record parent base branch in local Git config (when `<base>` is resolved):
       ```bash
       git config branch.<branchPath>/<issueKey>.brx-base <base>
       ```
   - Log: `[git-brx] Hint: Use 'git-brx publish' to publish your new branch`.
   - Exit with Exit Code `0`.

## 4. Error Handling & Exit Codes
| Exit Code | Scenario | Emitted Stderr Pattern | Side Effect Cleanup |
| :--- | :--- | :--- | :--- |
| `0` | Branch created, or existing branch checked out automatically. | `[git-brx] Creating branch '<branch>'` / `[git-brx] Branch '<branch>' already exists; switched to '<branch>'` | None. |
| `2` | Missing mandatory `<branch>` positional argument or unknown flag. | `[git-brx] Error: No branch name specified` | None. |
| `3` | Outside Git repository or `--base <branch>` not found. | `[git-brx] Error: Not a git repository` / `[git-brx] Error: Base branch '<base>' not found` | None. |
| `4` | Remote `origin` unreachable in online mode. | `[git-brx] Error: Unable to reach remote` | None. |
| `5` | Active merge/rebase, orphaned detached HEAD commits, or checkout overwrite collision when auto-switching. | `[git-brx] Error: Cannot switch branches during an active <op>` | None. |
| `6` | Tracker API authentication failure or issue not found. | `[git-brx] Error: Authorization failure` / `[git-brx] Error: Issue not found` | None. |
| `7` | User aborted creation during interactive confirmation. | `[git-brx] Aborted` | None. |
| `8` | Configuration file missing or branch name fails regex naming contract. | `[git-brx] Error: Configuration file not found` / `[git-brx] Error: Branch name doesn't match pattern` | None. |

## 5. Edge Cases & Behavioral Notes
- **In-Memory Credential Safety:** Tracker credentials are kept strictly in-memory within the process and never passed via command-line arguments or process environments, preventing exposure via `ps aux` or process table inspection.
- **Pre-existing Branch Auto-Switch:** When `<branch>` already exists locally or on `origin`, `create` switches directly to `<branch>` (subject to `select` safety checks) and exits `0` so agents and scripts never commit onto the wrong branch.
- **Parent Base Persistence:** Storing `branch.<name>.brx-base` in local Git config avoids custom metadata files and is automatically pruned by `git branch -D` in `delete`.
- **Subdirectory Invocation Support:** Configuration loading and repository root resolution operate relative to the top-level repository root (`git rev-parse --show-toplevel`), allowing invocation from any working subdirectory.

## 6. Black-box verification scenarios
| Scenario ID | Given | When | Then (exit code, output, state) |
| :--- | :--- | :--- | :--- |
| `TC-CRT-01` | Currently on `main`, branch `issue/42` already exists locally | `git-brx create issue/42 -y` | Exits `0`, checks out `issue/42`, logs `Branch 'issue/42' already exists; switched to 'issue/42'`. |
| `TC-CRT-02` | Branch `issue/43` exists only on `origin`, not locally | `git-brx create issue/43 -y` | Fetches `issue/43`, creates local tracking checkout, exits `0`. |
| `TC-CRT-03` | Currently on `main`, `epic/10` exists locally or on `origin` | `git-brx create issue/44 -y --base epic/10` | Forks `issue/44` from `epic/10` (or `origin/epic/10`), sets `branch.issue/44.brx-base=epic/10`, exits `0`. |
| `TC-CRT-04` | Currently on `issue/10` with uncommitted edits | `git-brx create issue/45 -y` | Emits branch-fork warning and dirty-worktree warning, creates `issue/45`, exits `0`. |
