# Spec: git-brx create

## 1. Command Signature
- **Usage:** `git-brx create [flags] <branch>`
- **Arguments:**
  - `<branch>` *(mandatory)*: Name of the topic branch to create (e.g., `issue/VSB-1234`, `feature/VSB-5678`).
- **Flags:**
  - Standard global flags: `--help` (`-h`), `--version`, `--verbose` (`-v`), `--quiet` (`-q`), `--dry-run` (`-n`), `--no-color`.
  - Feature flags:
    - `-o`, `--offline`: Skip JIRA issue validation and remote Git availability checks.
    - `-y`, `--yes`: Automatically accept confirmation prompts without interactive input.

## 2. Prerequisites & Environment
- **Required host binaries:** `git` (>= 2.20.0).
- **Environment variables read:** `GIT_DIR`, `GIT_WORK_TREE`, `GIT_PREFIX`, `GIT_BRX_CONFIG_PATH`, `GIT_BRX_TOKEN`, `HTTP_PROXY`, `HTTPS_PROXY`, `NO_COLOR`.
- **Working directory requirements (GIT_PREFIX behavior):** Can be invoked from any repository subdirectory; operations are executed relative to repository root (`git rev-parse --show-toplevel`).
- **Git state assertions (e.g., dirty working tree check):**
  - Must be inside a Git working tree (`git rev-parse --is-inside-work-tree` == `true`).
  - Active rebase or merge must NOT be in progress.
  - Working tree should be clean; uncommitted changes will be carried over to the new branch if safe, or will trigger a Git checkout failure if conflicting.

## 3. Core Execution Flow
1. **Argument & Repository Assertions:**
   - Verify argument `<branch>` is provided. If missing, log `[git-brx] ! No branch name specified` and exit with Exit Code `2`.
   - Verify repository context: `git rev-parse --is-inside-work-tree`.
2. **Local Branch Collision Check:**
   - Check if `<branch>` already exists locally:
     ```bash
     git show-ref --verify --quiet refs/heads/<branch>
     ```
   - If present:
     - Log: `[git-brx] Branch '<branch>' found (local)`
     - Log: `[git-brx] Hint: To select that branch, use 'git-brx select <branch>' instead`
     - Exit with Exit Code `0`.
3. **Remote Branch Collision Check (Online Mode):**
   - If `--offline` is NOT set:
     - Check if `<branch>` already exists on remote:
       ```bash
       git ls-remote --heads origin <branch>
       ```
     - If remote is unreachable, log `[git-brx] ! Unable to reach remote; try --offline if working without network access` and fail with Exit Code `4`.
     - If ref exists on remote:
       - Log: `[git-brx] Branch '<branch>' found (remote)`
       - Log: `[git-brx] Hint: To select this branch, use 'git-brx select <branch>' instead`
       - Exit with Exit Code `0`.
4. **Configuration & Template Matching:**
   - Load configuration via the discovery cascade (`GIT_BRX_CONFIG_PATH` -> `.git-brx.yaml` -> `.git-brx/config.yaml` -> `~/.config/git-brx/config.yaml` -> zero-config remote origin auto-discovery). If invalid, exit with Exit Code `8`.
   - Compile regular expression template from `config.branch.template` (interpolating `config.tracker.project` if set).
   - Match `<branch>` against the template. Extract:
     - `branchPath` (e.g., `issue`, `feature`, `epic`)
     - `issueKey` (e.g., `VSB-1234` or `#42` or `42`)
   - If regex does not match, log `[git-brx] ! Branch name '<branch>' doesn't match pattern <pattern>` and exit with Exit Code `8`.
5. **Issue Tracker Validation (Online Mode):**
   - If `--offline` is NOT set:
     - Fetch issue details using the configured `IssueTracker` adapter (Jira REST, GitHub Issues, etc.).
     - If authorization fails: Fail with Exit Code `6` (`Authorization failure`).
     - If issue not found: Fail with Exit Code `6` (`Issue not found in issue tracker`).
     - Check issue type mapping: If `config.branch.mapping` has an entry for `issue.Type`, verify `mapping[issue.Type] == branchPath`.
       - If mismatch: Log `[git-brx] ! Issue type '<type>' is not allowed on '<branchPath>' branch` with hint and fail with Exit Code `8`.
     - If issue status is closed/resolved, display warning:
       `[git-brx] Warning: Issue '<issueKey>' is currently marked as '<status>'`
     - Confirmation prompt (unless `--yes` is specified):
       `Create branch '<branch>' [Y/n]?` (defaults to Yes on Enter).
       - If rejected (`n` / `N`), exit with Exit Code `7`.
6. **Branch Provisioning:**
   - If `--dry-run` is active: Log planned creation and exit with Exit Code `0`.
   - Execute checkout to new branch:
     ```bash
     git checkout -b <branchPath>/<issueKey>
     ```
   - Log: `[git-brx] Hint: Use 'git-brx publish' to publish a new branch`.
   - Exit with Exit Code `0`.

## 4. Error Handling & Exit Codes
| Exit Code | Scenario | Emitted Stderr Pattern | Side Effect Cleanup |
| :--- | :--- | :--- | :--- |
| `0` | Branch successfully created, or already exists locally/remotely with guidance. | `[git-brx] Creating branch '<branch>'` | None. |
| `2` | Missing mandatory `<branch>` positional argument or unknown flag. | `[git-brx] ! No branch name specified` | None. |
| `3` | Outside Git repository. | `[git-brx] ! Awh! This is not a git repository` | None. |
| `4` | Remote `origin` unreachable in online mode. | `[git-brx] ! Unable to reach remote` | None. |
| `6` | JIRA API authentication failure or issue not found. | `[git-brx] ! Authorization failure` / `[git-brx] ! Issue not found` | None. |
| `7` | User aborted creation during interactive confirmation. | None (aborted cleanly). | None. |
| `8` | Configuration file missing or branch name fails regex naming contract. | `[git-brx] ! Configuration file not found` / `[git-brx] ! Branch name doesn't match pattern` | None. |

## 5. Discrepancies & Edge Cases Discovered
- **Credential Leak via Process Table:** In legacy `create.sh`, the script invoked Groovy by passing base64-encoded credentials on the command-line: `groovy -cp ... create.groovy "$1" "$offline" "$TOOLS_CREDENTIALS" ...`. Any local user could inspect these credentials via `ps aux`. The Go binary eliminates this by keeping credentials entirely in-memory within the single binary process.
- **Pre-existing Branch Exit Code Convention:** When the branch already exists locally or remotely, legacy exited with status `0` while outputting a hint to use `select`. Modern Go preserves this user-friendly idempotent pattern.
- **Subdirectory Invocation Failure:** Legacy `create.sh` computed project configuration via `pwd | cut -d _ -f 1`. Invoking the command from a subfolder caused config loading to fail. Modern Go dynamically locates the repository root and project configuration.
