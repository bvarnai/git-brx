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
    - `--strategy` (`-s` `<strategy>`): Override merge strategy (`squash`, `merge`, or `rebase`).
    - `--merge-instructions`: Explicitly enable merge instructions in PR body.
    - `--no-merge-instructions`: Explicitly disable merge instructions in PR body.

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
   - If `--reviewer` (`-r`) flag was provided, use specified user.
   - Otherwise, resolve reviewers using structured `config.review.mapping`:
     - **Resolution Hierarchy:**
       1. **Changed file paths (`review.mapping.paths`):** Evaluates modified files on topic branch (`git diff --name-only <base>...HEAD`) against path globs (e.g. `internal/ui/**`, `docs/**`). Collects and deduplicates all matched reviewers.
       2. **Issue labels or components (`review.mapping.labels`):** If no path rules matched, matches issue labels (GitHub) or components (Jira).
       3. **Branch patterns (`review.mapping.branches`):** If no labels matched, matches topic branch name against glob patterns (e.g. `epic/*`).
       4. **Default fallback (`review.mapping.default`):** Falls back to default reviewer if no prior rules matched.
     - **Author Exclusion:** Automatically excludes the PR author (detected from GitHub login or local Git user identity) from the candidate reviewer list across all tiers.
     - Reviewer values support list format (`[alice, bob]`), comma-separated string (`"alice, bob"`), or single string (`alice`).
6. **Description Composition:**
   - Assemble review body from template:
     - Load markdown checklist from `config.review.template` (or fallback template).
     - Append `### Merge instructions` block via `SCMProvider.FormatMergeInstructions(opts)` (unless disabled via `config.review.merge.instructions: false` or `--no-merge-instructions` flag; overridden by `--merge-instructions`):
       - **Merge Strategy Resolution Hierarchy:**
         1. CLI override (`-s, --strategy <squash|merge|rebase>`).
         2. Target branch pattern match (`review.merge.strategies.targets`).
         3. Source branch pattern match (`review.merge.strategies.sources`).
         4. Built-in convention: Source or target branch starts with `epic/` or `release/` -> `merge` (`Create a merge commit`).
         5. Global configuration fallback: `review.merge.strategy` (defaults to `squash`).
       - **Supported Strategies:**
         - **`squash` (`Squash and merge (recommended)`):** Commit: `<prTitle>` (or fallback `<issueKey>: <summary>`) (all commits squashed into a single commit).
         - **`merge` (`Create a merge commit`):** Commit: `Merge branch '<branch>' into <target>` (merge commit preserving all constituent commits).
         - **`rebase` (`Rebase and merge`):** Commit: Linear rebase onto `<target>` (all constituent commits preserved, no merge commit).
       - **Branch cleanup:** Instructs deletion of source branch after merge.
       - **Built-in Convention as Configuration Snippet:**
         ```yaml
         review:
           merge:
             instructions: true           # Enable/disable merge instructions block
             strategy: squash             # Global fallback (squash | merge | rebase)
             strategies:
               sources:                   # Match against PR source branch (HEAD)
                 "epic/*": merge
                 "release/*": merge
               targets:                   # Match against PR target branch (BASE)
                 "epic/*": merge
                 "release/*": merge
         ```
7. **Dry-Run Check:**
   - If `--dry-run` is active: Format and print the pull request request payload and reviewer assignment without making API calls, then exit with Exit Code `0`.
8. **Pull Request Submission:**
   - Submit PR via the configured `SCMProvider` (GitHub Pull Requests API, Bitbucket Server REST API, etc.).
   - Standard payload fields:
     - `title`: `<type>(#<issueKey>): <summary>` (Conventional Commits scope format, or `<branch>` if unlinked, or custom `--title`)
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
