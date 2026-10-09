# Spec: git-brx review

## 1. Command Signature
- **Usage:** `git-brx review [flags] [<target_branch>]`
- **Arguments:**
  - `<target_branch>` *(optional)*: Target branch to merge into. Defaults to `master` if omitted.
- **Flags:**
  - Standard global flags: `--help` (`-h`), `--version`, `--verbose` (`-v`), `--quiet` (`-q`), `--dry-run` (`-n`), `--no-color`.
  - Feature flags:
    - `--reviewer` (`-r` `<username>`): Explicitly assign a designated reviewer instead of automated component mapping.
    - `--title` (`-t` `<title>`): Override default pull request title.

## 2. Prerequisites & Environment
- **Required host binaries:** `git` (>= 2.20.0).
- **Environment variables read:** `GIT_DIR`, `GIT_WORK_TREE`, `GIT_PREFIX`, `GIT_BRX_CONFIG_PATH`, `GIT_BRX_TOKEN`, `HTTP_PROXY`, `HTTPS_PROXY`, `NO_COLOR`.
- **Working directory requirements (GIT_PREFIX behavior):** Can be invoked from any repository subdirectory; operations are executed relative to repository root (`git rev-parse --show-toplevel`).
- **Git state assertions (e.g., dirty working tree check):**
  - Must be inside a Git working tree (`git rev-parse --is-inside-work-tree` == `true`).
  - Must NOT be in a detached HEAD state (`git symbolic-ref -q HEAD`).
  - Active branch must be an eligible topic branch (`issue/*`, `feature/*`, or `epic/*`).
  - Source branch must already be published to remote origin (`git ls-remote --heads origin <branch>`).

## 3. Core Execution Flow
1. **Repository & State Verification:**
   - Verify worktree: `git rev-parse --is-inside-work-tree`.
   - Assert attached branch: `git symbolic-ref --short -q HEAD`. Store current branch `<branch>`.
   - Validate branch prefix (`issue`, `feature`, or `epic`). If unrecognized, fail with Exit Code `3`.
2. **Target Branch Resolution:**
   - If `<target_branch>` argument is omitted, default to `master` and log:
     `[git-brx] Selecting 'master' target branch by default`.
3. **Publication Precondition Check:**
   - Verify that `<branch>` has been published to remote `origin`:
     ```bash
     git ls-remote --heads origin <branch>
     ```
   - If missing on remote:
     - Log: `[git-brx] Error: Branch '<branch>' has not been pushed to remote origin`
     - Log: `[git-brx] Hint: Run 'git-brx publish' to push your branch before creating a review`
     - Fail with Exit Code `4`.
4. **Configuration & Issue Loading:**
   - Load configuration via the discovery cascade (`GIT_BRX_CONFIG_PATH` -> `.git-brx.yaml` -> `.git-brx/config.yaml` -> `~/.config/git-brx/config.yaml` -> zero-config remote origin auto-discovery).
   - Extract `issueKey` from `<branch>` using the configured branch template regex.
   - Fetch issue details using the configured `IssueTracker` (Jira REST, GitHub, etc.) to retrieve components/labels and summary.
   - If issue call fails (401/404): Fail with Exit Code `6`.
5. **Reviewer Resolution:**
   - If `--reviewer` flag was provided, use specified user.
   - Otherwise, resolve reviewers from issue components using `config.review.mapping`:
     - Lookup each component in mapping.
     - If unmapped or no components specified, fallback to `config.review.mapping.default`.
     - Supports list or comma-separated candidate reviewers.
6. **Description Composition:**
   - Assemble review body from template:
     - Load markdown checklist from `config.review.template` (or fallback template).
     - Append `# Merge instructions` block via `SCMProvider.FormatMergeInstructions(opts)` (unless disabled via `config.review.instructions: false`):
       - **Squash & Merge (recommended):** Commit title formatted as `<issueKey>: <summary>` (or `<branch>` if issue is unlinked).
       - **Branch Cleanup:** Instructs deletion of source branch after merge.
       - Timeless, platform-agnostic format focusing on Git intent rather than volatile web UI button labels.
7. **Dry-Run Check:**
   - If `--dry-run` is active: Format and print the pull request request payload and reviewer assignment without making API calls, then exit with Exit Code `0`.
8. **Pull Request Submission:**
   - Submit PR via the configured `SCMProvider` (GitHub Pull Requests API, Bitbucket Server REST API, etc.).
   - Standard payload fields:
     - `title`: `<branch>` (or custom `--title`)
     - `description`: `<reviewDescription>`
     - `sourceBranch`: `<branch>`
     - `targetBranch`: `<targetBranch>`
     - `reviewers`: List of resolved reviewer user handles.
9. **Response Handling:**
   - If HTTP 201 Created:
     - Log: `[git-brx] Created pull-request <links.self.href>`.
     - Emit URL to stdout for machine consumption.
     - Exit with Exit Code `0`.
   - If HTTP 409 Conflict (e.g., open pull request already exists):
     - Log: `[git-brx] Error: A pull request for branch '<branch>' already exists`.
     - Fail with Exit Code `6`.
   - If HTTP 400/401/403/404:
     - Extract and log API error messages: `[git-brx] Error: <error_message>`.
     - Fail with Exit Code `6`.

## 4. Error Handling & Exit Codes
| Exit Code | Scenario | Emitted Stderr Pattern | Side Effect Cleanup |
| :--- | :--- | :--- | :--- |
| `0` | Pull request created successfully. | `[git-brx] Created pull-request <url>` | None. |
| `2` | Excess positional arguments or unknown flags. | `[git-brx] Error: Unexpected argument: <arg>` | None. |
| `3` | Detached HEAD or unpermitted branch type (e.g. running on `master`). | `[git-brx] Error: You must be on an 'issue', 'feature', or 'epic' branch` | None. |
| `4` | Source branch not published on remote origin. | `[git-brx] Error: Branch '<branch>' has not been pushed to remote origin` | None. |
| `6` | JIRA or Bitbucket API failure (HTTP 401, 404, 409 conflict). | `[git-brx] Error: A pull request for branch '<branch>' already exists` / `[git-brx] Error: Request failure` | None. |
| `8` | Configuration file or review template missing / corrupt. | `[git-brx] Error: Configuration file not found` | None. |

## 5. Discrepancies & Edge Cases Discovered
- **Silent Failure on Unpublished Branch:** Legacy `review.sh` never checked if the current branch was published before calling `review.groovy`. Bitbucket Server would return a confusing 400 Bad Request error stating that the ref did not exist. Modern Go explicitly tests remote ref availability first and hints to run `publish`.
- **Command-Line Security Leak:** Legacy `review.sh` passed basic auth credentials to Groovy via process arguments, exposing secrets to `ps`. The Go rewrite manages all authentication internally within process memory.
- **Random Reviewer Uniformity:** When multiple reviewers were mapped to a component (e.g. `atnemeth,zkrajovs`), legacy picked one via `java.util.Random`. The Go spec preserves this randomized load-balancing while adding an explicit `--reviewer` flag override.
