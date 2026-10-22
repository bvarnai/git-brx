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
2. **Name validation:** Matches the branch name against `branch.template` (e.g., `issue/<KEY>`, `feature/<KEY>`, `epic/<KEY>`).
3. **Collision checks:**
   - If the branch already exists locally or on `origin`, `git-brx` exits cleanly and prints a hint to run `git brx select issue/TASK-101`.
4. **Issue tracker lookup:**
   - Queries Jira or GitHub Issues for `TASK-101`.
   - Verifies that the issue type/labels are allowed for the target branch prefix (e.g., `bug` $\to$ `issue/`) and prints the issue title, status, assignee, and URL.
   - Prompts `Create branch 'issue/TASK-101' [Y/n]?` (pass `-y` / `--yes` to skip the prompt in scripts).
5. **Checkout:** Runs `git checkout -b issue/TASK-101`.

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
- **Behind-upstream hint:** If you are already on the target branch and it is behind `origin`, `git-brx` prints a hint:
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

`git brx publish` pushes with **`--force-with-lease`** by default:
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
1. **Remote sync check:** Verifies that your local branch is pushed to `origin` and up to date with `origin/<branch>`.
2. **Issue metadata lookup:** Extracts the issue key from the branch name and fetches the issue title and labels from Jira or GitHub.
3. **Reviewer routing:** Evaluates `review.mapping` in `.git-brx.yaml` in order of precedence:
   - **CLI override:** `-r, --reviewer <user>` (repeatable or comma-separated).
   - **Changed file paths (`mapping.paths`):** Matches modified files against glob patterns (e.g., `internal/ui/**`, `docs/**`) and aggregates all matched reviewers.
   - **Issue labels (`mapping.labels`):** If no path rules match, checks Jira components or GitHub labels.
   - **Branch globs (`mapping.branches`):** If no label rules match, checks the source branch name (e.g., `epic/*`).
   - **Default fallback (`mapping.default`):** Used when no earlier rules match.
   - **Author exclusion:** Removes the PR author from the resolved reviewer list.
4. **PR body generation:** Loads your PR template (e.g., `.github/pull_request_template.md`), adds the issue closing reference (`Closes #<id>`), and appends **Merge instructions**:
   ```markdown
   Closes #TASK-101

   ### Merge instructions
   - **Strategy:** Squash and merge (recommended)
     - **Commit:** `fix(#TASK-101): add user authentication API` (all commits squashed into a single commit)
   - **Branch cleanup:** Delete source branch `issue/TASK-101` after merging
   ```
   - `issue/*` and `feature/*` branches targeting `main` default to **Squash and merge**.
   - `epic/*` and `release/*` branches (as source or target) default to **Create a merge commit**.
   - Override the merge strategy for a specific PR with `-s, --strategy <squash|merge|rebase>`:
     ```bash
     git brx review -s merge
     ```
5. **API submission:** Creates the PR via the GitHub or Bitbucket Server REST API and prints the PR URL to `stdout`.

---

## 6. Safe post-merge deletion (`git brx delete`)

After your pull request is merged on GitHub or Bitbucket, clean up the branch:

```bash
git brx delete
```

### Execution steps
1. **Remote deletion check:** Verifies via `git ls-remote --heads origin <branch>` that the branch has already been deleted on `origin`.
2. **SCM status check:** If the branch still exists on `origin`, `git-brx` queries the SCM API:
   - **PR still open:** Blocks deletion and prints the open PR URL.
   - **PR merged (remote branch still present):** Informs you that the PR is merged and prompts you to delete the remote branch on the server or pass `--force` (`-f`).
3. **Switch to base branch:** Checks out `main` (or `master`) if you are currently on the branch being deleted.
4. **Local cleanup:** Runs `git branch -D <branch>` and `git remote prune origin`.
5. **Update hint:**
   ```text
   [git-brx] Deleted local branch 'issue/TASK-101' and pruned origin
   [git-brx] Hint: Your local 'main' may not be up-to-date. Use 'git-brx update' to update changes
   ```
