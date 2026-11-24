# The branch lifecycle

This guide walks through the lifecycle of a branch in `git-brx`, from creating a validated topic branch to cleaning it up after merge.

---

## 1. Branch taxonomy & lifecycle strategies

`git-brx` maps every branch prefix to a default `sync` strategy and pull request merge strategy:

| Branch type | Scope & purpose | Lifespan | Default `sync` strategy | Default PR merge strategy |
| :--- | :--- | :--- | :--- | :--- |
| **`issue/<id>`** | Bug fixes, small tasks, tech debt | Short-lived | **Rebase** (`git rebase`) | **Squash & Merge** |
| **`feature/<id>`** | Backlog stories, multi-person features | Short-to-medium | **Merge** (`git merge`) | **Squash & Merge** *(or Merge commit)* |
| **`epic/<id>`** | Multi-PR initiatives | Long-lived | **Merge** (`git merge`) | **Merge commit (`--no-ff`)** |
| **`release/<milestone>`** | Release stabilization | Maintained / temporary | N/A | Tagged & archived |
| **`main` / `master`** | Default base branch | Permanent | N/A | Protected base branch |

### Why `sync` strategies differ by prefix
* **`issue/*` $\to$ Rebase:** Bug fixes and small tasks are usually single-author branches. Rebasing onto `main` keeps commit history linear and prepares the branch for a clean squash merge.
* **`feature/*` and `epic/*` $\to$ Merge:** Shared or long-running branches often have multiple contributors. Merging `main` into the branch avoids rewriting commit history under teammates working on the same branch.
* **Overriding the default:** Pass `--rebase` (`-r`) or `--merge` (`-m`) to `git brx sync` at any time.

> [!NOTE]
> **Convention over configuration:** Tracked branches follow the `<type>/<id>` pattern so `git-brx` knows how to sync, review, and clean them up. For local throwaway experiments, use standard Git (`git checkout -b scratch-spike`).

> [!TIP]
> **Branch protection & force-pushes:** Protect base branches (`main`, `master`, `release/*`) against direct pushes and deletions, while allowing `--force-with-lease` pushes on topic branches (`issue/*`, `feature/*`). See [Branch protection rules & repository permissions](multi-platform.md#5-branch-protection-rules--repository-permissions) for details.

---

## 2. Branch creation (`git brx create`)

Create a new topic branch linked to an issue:

```bash
git brx create issue/TASK-101
```

### Execution steps
1. **Worktree check:** Verifies that you are inside a Git worktree with no in-flight merge or rebase.
2. **Collision auto-switch:**
   - If the branch already exists locally or on `origin`, `git-brx` fetches if needed, safely switches to that branch (fast-forwarding clean branches behind `origin/<branch>`), and exits `0`.
3. **Name validation:** Matches the branch name against `branch.template` (e.g., `issue/<KEY>`, `feature/<KEY>`, `epic/<KEY>`).
4. **Issue tracker lookup:**
   - Queries Jira or GitHub Issues for `TASK-101`.
   - Verifies that the issue type/labels are allowed for the target branch prefix (e.g., `bug` $\to$ `issue/`) and prints the issue title, status, assignee, and URL.
   - Prompts `Create branch 'issue/TASK-101' [Y/n]?` (pass `-y` / `--yes` to skip the prompt in scripts).
5. **Base guardrails & checkout:** Validates `--base <branch>` if provided (or warns when forking from another topic branch, a behind-origin base, or a dirty worktree), runs `git checkout -b issue/TASK-101 [<base>]`, and records `branch.<name>.brx-base` in local `.git/config`.

### Offline mode
When working without network access, pass `--offline` (`-o`):
```bash
git brx create --offline issue/TASK-101
```
This skips the remote ref check and issue tracker lookup, validating only the local branch name syntax.

---

## 3. Branch switching & exploration (`git brx select`, `name`, `history`)

### Switching branches (`git brx select`)
```bash
# Switch to the default branch (main or master):
git brx select

# Switch to a specific branch:
git brx select issue/TASK-101
```

**What `select` does:**
- **Fetches remotes:** Runs `git fetch` before switching (unless `--offline` is passed).
- **Auto-fast-forward & behind-upstream hint:** When online and clean, if the target branch is strictly behind `origin/<branch>`, `git-brx` automatically fast-forwards it (`--ff-only`, unless `--no-ff` is passed). When dirty, diverged, or `--no-ff` is active, it prints a hint:
  ```text
  [git-brx] Already on 'issue/TASK-101'
  [git-brx] Hint: Your branch is behind 'origin/issue/TASK-101' by 2 commit(s). Run 'git-brx update' to update.
  ```
- **Typo suggestions:** If the target branch doesn't exist, `git-brx` suggests close matches using Levenshtein distance:
  ```text
  [git-brx] Error: Branch 'featrue/login' not found
  [git-brx] Hint: Did you mean 'feature/login'?
  ```
- **Dirty working tree warning:** Warns if uncommitted local changes are carried over into the newly checked-out branch.

### Printing the current branch (`git brx name`)
Prints the active branch name to `stdout` with no prefix, suitable for shell scripts:
```bash
CURRENT=$(git brx name)
echo "Active branch is $CURRENT"
```
If HEAD is detached, it writes the short commit SHA to `stdout` and logs a warning to `stderr`.

### Viewing commit history (`git brx history`)
Renders a compact commit graph:
```bash
# Show the last 10 commits:
git brx history -l 10

# Show only commits on the current topic branch since diverging from main:
git brx history --topic

# Search commit messages:
git brx history -s "database migration"

# Include file diffstats:
git brx history --stat
```

---

## 4. Leased publishing (`git brx publish`)

Push your branch to `origin` and configure upstream tracking:

```bash
git brx publish
```

### Default `--force-with-lease` protection
Raw `git push` fails after a rebase, while `git push --force` blindly overwrites commits pushed by teammates.

`git brx publish` pushes with **`--force-with-lease`** by default on `issue/*`, `feature/*`, and custom branches, and always enforces **fast-forward-only** pushes (never `--force-with-lease`) on shared `epic/*` and `release/*` integration branches so child branches forked from them stay intact:
- Sets upstream tracking (`--set-upstream origin <branch>`) on first push.
- Rejects the push if `origin/<branch>` has commits you haven't fetched yet:
  ```text
  [git-brx] Error: Publish rejected: remote has newer commits
  [git-brx] Hint: Run 'git-brx update' to incorporate remote changes before publishing
  ```

---

## 5. Code review submission (`git brx review`)

Open a pull request from your published branch:

```bash
git brx review
```

### How `review` builds the pull request
1. **Remote sync & target check:** Verifies that local `HEAD` matches `origin/<branch>`, resolves `<target_branch>` (explicit argument $\to$ recorded `branch.<name>.brx-base` $\to$ default branch), and verifies `<target_branch>` exists on `origin`.
2. **Issue metadata lookup:** Extracts the issue key from the branch name and fetches the issue title and labels from Jira or GitHub.
3. **Reviewer routing:** Evaluates `review.mapping` in `.git-brx.yaml` in order of precedence (skipped when `--draft` is set unless `-r` is passed):
   - **CLI override:** `-r, --reviewer <user>` (repeatable or comma-separated, deduplicated).
   - **Changed file paths (`mapping.paths`):** Matches modified files against `origin/<target>...HEAD` (falling back to `<target>...HEAD`) globs (e.g., `internal/ui/**`, `docs/**`) and aggregates all matched reviewers.
   - **Issue labels (`mapping.labels`):** If no path rules match, checks Jira components or GitHub labels.
   - **Branch globs (`mapping.branches`):** If no label rules match, checks the source branch name (e.g., `epic/*`).
   - **Default fallback (`mapping.default`):** Used when no earlier rules match.
   - **Author exclusion:** Removes the PR author from the resolved reviewer list.
4. **PR body generation:** Loads your configured `review.template` (or auto-discovers a repository PR template), adds the issue closing reference (`Closes #<id>`), and appends **Merge instructions**:
   ```markdown
   Closes #TASK-101

   ### Merge instructions
   - **Strategy:** Squash and merge (recommended)
     - **Commit:** `fix(#TASK-101): add user authentication API` (all commits squashed into a single commit)
   - **Branch cleanup:** Delete source branch `issue/TASK-101` after merging
   ```
   - `issue/*` and `feature/*` branches default to **Squash and merge** (whether targeting `main` or `epic/*`).
   - `epic/*` and `release/*` branches default to **Create a merge commit** when acting as the PR source branch (e.g., `epic/*` into `main`).
   - Override the merge strategy for a specific PR with `-s, --strategy <squash|merge|rebase>` or create a draft PR with `--draft`:
     ```bash
     git brx review -s merge
     git brx review --draft
     ```
5. **API submission:** Creates the PR via the GitHub or Bitbucket Server REST API (emitting a warning if reviewer assignment returns an API error, or including the existing PR URL on 409 conflicts) and prints the PR URL to `stdout`.

---

## 6. Safe post-merge deletion (`git brx delete`)

After your pull request is merged on GitHub or Bitbucket, clean up the branch:

```bash
git brx delete
```

### Execution steps
1. **Remote deletion check:** Verifies via `git ls-remote --heads origin <branch>` that the branch has already been deleted on `origin`.
2. **SCM status check & `--delete-remote`:** If the branch still exists on `origin`, `git-brx` queries the SCM API (prioritizing `OPEN > MERGED > CLOSED` across reused branch names):
   - **PR still open:** Blocks deletion (even with `--delete-remote`) and prints the open PR URL.
   - **PR merged (remote branch still present):** Pass `--delete-remote` to delete `origin/<branch>` automatically before removing the local branch, or delete it manually / pass `--force` (`-f`).
3. **Switch to parent or default base branch:** If currently on `<branch>`, returns to the recorded `branch.<branch>.brx-base` (or `main`/`master`) and fast-forwards it automatically when clean and behind `origin`. In linked worktrees where the return branch is checked out elsewhere (or remote-only), detaches `HEAD` onto `origin/<return>` instead of failing checkout.
4. **Local cleanup:** Runs `git branch -D <branch>` and `git remote prune origin`.
5. **Update hint:** When switched to an unfetched/dirty return branch without auto-fast-forwarding, prints a hint to run `git-brx update`.
   ```text
   [git-brx] Deleted local branch 'issue/TASK-101' and pruned origin
   [git-brx] Hint: Your local 'main' may not be up-to-date. Use 'git-brx update' to update changes
   ```
