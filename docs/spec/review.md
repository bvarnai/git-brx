# Spec: git-brx review

## 1. Command Signature
- **Usage:** `git-brx review [flags] [<target_branch>]`
- **Arguments:**
  - `<target_branch>` *(optional)*: Target branch to merge into. Defaults to repository default branch (`main` or `master`) if omitted.
- **Flags:**
  - Standard global flags: `--help` (`-h`), `--version`, `--verbose` (`-v`), `--quiet` (`-q`), `--no-color`.
  - Feature flags:
    - `--reviewer` (`-r` `<username>`): Explicitly assign designated reviewer(s) instead of automated component mapping. Repeatable and comma-separated (e.g., `-r alice,bob` or `-r alice -r bob`), trimmed and deduplicated.
    - `--title` (`-t` `<title>`): Override default pull request title.
    - `--strategy` (`-s` `<strategy>`): Override merge strategy (`squash`, `merge`, or `rebase`).
    - `--draft`: Create the pull request in draft state (skips automated `review.mapping` reviewer assignment unless `--reviewer` is explicitly provided).
    - `--merge-instructions`: Explicitly enable merge instructions in PR body.
    - `--no-merge-instructions`: Explicitly disable merge instructions in PR body.

## 2. Prerequisites & Environment
- **Required host binaries:** `git` (>= 2.20.0).
- **Environment variables read:** `GIT_DIR`, `GIT_WORK_TREE`, `GIT_PREFIX`, `GIT_BRX_CONFIG`, `GITHUB_TOKEN`, `GH_TOKEN`, `GIT_BRX_TOKEN`, `GIT_BRX_COLOR`, `NO_COLOR`.
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
2. **Target Branch Resolution & Remote Existence Validation:**
   - Resolve `<targetBranch>` in strict order:
     1. Explicit positional `<target_branch>` argument, if supplied.
     2. Recorded parent base (`git config --get branch.<branch>.brx-base`), logging `[git-brx] Selecting recorded base '<base>' as target branch`.
     3. Repository default base branch (`main`, `master`, or `origin/HEAD`), logging `[git-brx] Selecting '<default>' target branch by default`.
   - Validate that `<targetBranch>` exists on `origin`:
     ```bash
     git ls-remote --heads origin <targetBranch>
     ```
     If missing on `origin`, fail with Exit Code `4` (with Levenshtein closest-branch suggestion if available).
3. **Publication & Up-to-Date Precondition Check:**
   - Verify that `<branch>` has been published to remote `origin` and matches local `HEAD`:
     ```bash
     git ls-remote --heads origin <branch>
     git rev-parse HEAD
     ```
   - If missing on remote:
     - Log: `[git-brx] Error: Branch '<branch>' has not been pushed to remote origin`
     - Log: `[git-brx] Hint: Run 'git-brx publish' to push your branch before creating a review`
     - Fail with Exit Code `4`.
   - If remote SHA differs from local `HEAD` SHA:
     - Log: `[git-brx] Error: Branch '<branch>' has unpushed local commits (out of sync with origin)`
     - Log: `[git-brx] Hint: Run 'git-brx publish' to push your latest commits before creating a review`
     - Fail with Exit Code `4`.
4. **Configuration & Issue Loading:**
   - Load configuration via the layered cascade (User Global `~/.config/git-brx/config.yaml` / `~/.git-brx.yaml` -> Shared Global `GIT_BRX_CONFIG` -> Repo Local `.git-brx.yaml` / `.git-brx/config.yaml` -> zero-config remote origin auto-discovery).
   - Extract `issueKey` from `<branch>` using the configured branch template regex.
   - Fetch issue details using the configured `IssueTracker` (Jira REST, GitHub, etc.) to retrieve components/labels and summary.
   - If issue call fails (401/404): Fail with Exit Code `6`.
5. **Reviewer Resolution:**
   - If `--reviewer` (`-r`) flag(s) were provided, split values on commas, trim whitespace, drop empty items, and deduplicate (preserving order).
   - Else if `--draft` is set, skip automated `review.mapping` resolution (`reviewers = nil`) so draft PRs do not auto-ping mapped reviewers.
   - Otherwise, resolve reviewers using structured `config.review.mapping`:
     - **Resolution Hierarchy:**
       1. **Changed file paths (`review.mapping.paths`):** Evaluates modified files on topic branch (`git diff --name-only origin/<target>...HEAD`, falling back to `<target>...HEAD` if needed) against path globs (e.g. `internal/ui/**`, `docs/**`). Collects and deduplicates all matched reviewers.
       2. **Issue labels or components (`review.mapping.labels`):** If no path rules matched, matches issue labels (GitHub) or components (Jira).
       3. **Branch patterns (`review.mapping.branches`):** If no labels matched, matches topic branch name against glob patterns (e.g. `epic/*`).
       4. **Default fallback (`review.mapping.default`):** Falls back to default reviewer if no prior rules matched.
     - **Author Exclusion:** Automatically excludes the PR author (detected from GitHub login or local Git user identity) from the candidate reviewer list across all tiers.
     - Reviewer values support list format (`[alice, bob]`), comma-separated string (`"alice, bob"`), or single string (`alice`).
6. **Description Composition:**
   - Assemble review body:
     - Load template from `config.review.template`, or auto-discover standard repository templates (`.github/pull_request_template.md`, `.github/PULL_REQUEST_TEMPLATE.md`, `pull_request_template.md`).
     - If custom template is present, append horizontal divider (`---`).
     - Append closing reference (`Closes #<issueKey>`) if branch is linked to an issue.
     - Append `### Merge instructions` block via `SCMProvider.FormatMergeInstructions(opts)` (unless disabled via `config.review.merge.instructions: false` or `--no-merge-instructions` flag; overridden by `--merge-instructions`):
       - **Merge Strategy Resolution Hierarchy:**
         1. CLI override (`-s, --strategy <squash|merge|rebase>`).
         2. Target branch pattern match (`review.merge.strategies.targets`).
         3. Source branch pattern match (`review.merge.strategies.sources`).
         4. Built-in convention: Source branch starts with `epic/` or `release/` -> `merge` (`Create a merge commit`).
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
         ```
7. **Pull Request Submission:**
   - Submit PR via the configured `SCMProvider` (GitHub Pull Requests API, Bitbucket Server REST API, etc.).
   - Standard payload fields:
     - `title`: `<type>(#<issueKey>): <summary>` (Conventional Commits scope format, or `<branch>` if unlinked, or custom `--title`)
     - `description`: `<reviewDescription>`
     - `sourceBranch`: `<branch>`
     - `targetBranch`: `<targetBranch>`
     - `reviewers`: List of resolved reviewer user handles.
     - `draft`: Boolean from `--draft`.
   - **Reviewer assignment failure handling:** If PR creation succeeds (HTTP 201) but requesting reviewers fails, log `[git-brx] Warning: Pull request created, but failed to assign reviewers: <err>` to `stderr` (do not fail the command since the PR already exists).
8. **Response Handling:**
   - If HTTP 201 Created:
     - Log: `[git-brx] Created pull-request:` (to stderr).
     - Emit URL to stdout for machine consumption.
     - Exit with Exit Code `0`.
   - If HTTP 409 Conflict (e.g., open pull request already exists):
     - Query `SCMProvider.GetPullRequestForBranch(ctx, <branch>)` for the existing PR URL.
     - Log: `[git-brx] Error: A pull request for branch '<branch>' already exists (<url>)` (or without `(<url>)` if lookup fails).
     - Fail with Exit Code `6`.
   - If HTTP 400/401/403/404:
     - Extract and log API error messages: `[git-brx] Error: <error_message>`.
     - Fail with Exit Code `6`.

## 4. Error Handling & Exit Codes
| Exit Code | Scenario | Emitted Stderr Pattern | Side Effect Cleanup |
| :--- | :--- | :--- | :--- |
| `0` | Pull request created successfully. | `[git-brx] Created pull-request:` (URL emitted to stdout) | None. |
| `2` | Excess positional arguments or unknown flags. | `[git-brx] Error: Unexpected argument: <arg>` | None. |
| `3` | Detached HEAD or unpermitted branch type (e.g. running on `main` or `master`). | `[git-brx] Error: You must be on an 'issue', 'feature', or 'epic' branch` | None. |
| `4` | Source branch missing/out-of-sync on `origin`, or target branch missing on `origin`. | `[git-brx] Error: Branch '<branch>' has not been pushed to remote origin` / `...has unpushed local commits` | None. |
| `6` | SCM/Tracker API failure (HTTP 401, 404, 409 conflict). | `[git-brx] Error: A pull request for branch '<branch>' already exists (<url>)` / `[git-brx] Error: Request failure` | None. |
| `8` | Configuration file or review template missing / corrupt. | `[git-brx] Error: Configuration file not found` | None. |

## 5. Edge Cases & Behavioral Notes
- **Pre-flight Remote Sync & Target Verification:** Verifies that the remote tracking branch exists on `origin` and matches local `HEAD` before submitting pull request creation requests, and verifies that `<targetBranch>` exists on `origin`.
- **Non-Default Target Closing References:** Child PRs targeting `epic/*` retain `Closes #<issueKey>` in the PR body for issue cross-linking, though GitHub only auto-closes `#<issueKey>` when the PR merges into the repository default branch (`main`).
- **Secure In-Memory Credential Handling:** Authentication credentials and tokens are maintained exclusively within process memory for API calls, preventing secrets exposure in process lists.
- **Reviewer List Parsing:** `-r` supports both repeated flags and comma-separated usernames, deduplicating while preserving order.

## 6. Black-box verification scenarios
| Scenario ID | Given | When | Then (exit code, output, state) |
| :--- | :--- | :--- | :--- |
| `TC-REV-01` | Published `issue/50` with 1 new unpushed local commit | `git-brx review` | Exits `4` with `unpushed local commits` error and `git-brx publish` hint; no API POST sent. |
| `TC-REV-02` | Published `issue/50` targeting non-existent `epic/999` | `git-brx review epic/999` | Exits `4` reporting missing target branch on `origin`. |
| `TC-REV-03` | `issue/50` has `branch.issue/50.brx-base=epic/10` | `git-brx review` | Opens PR with `base=epic/10` and `Squash and merge` instructions without explicit `<target>`. |
| `TC-REV-04` | Published `issue/50` and reviewer API returns 422 | `git-brx review -r alice,bob -r bob` | Sends `["alice", "bob"]` to reviewer endpoint, logs stderr warning, still emits PR URL and exits `0`. |
| `TC-REV-05` | Published `issue/50` with `review.mapping.default` configured | `git-brx review --draft` / `git-brx review --draft -r alice` | Sends `draft: true` in PR payload; skips automated `review.mapping` reviewers when `-r` is omitted and assigns `["alice"]` when `-r alice` is explicitly passed, exits `0`. |
| `TC-REV-06` | `issue/50` already has an open PR `#77` | `git-brx review` | Queries existing PR and exits `6` with URL for `#77`. |
| `TC-REV-07` | `origin/epic/10` exists remotely, no local `epic/10` | `git-brx review epic/10` | Computes `ChangedFiles` against `origin/epic/10...HEAD` for `review.mapping.paths`. |
