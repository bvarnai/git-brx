# The Branch Lifecycle

This guide walks you through the complete lifecycle of a development branch in `git-brx`—from validated creation to post-merge cleanup.

---

## 1. Branch Creation (`git brx create`)

Every change in your codebase begins with a validated topic branch:

```bash
git brx create issue/TASK-101
```

### What Happens Behind the Scenes:
1. **Repository Verification:** Asserts that you are inside a valid Git worktree and that no active merge or rebase is blocking your workspace.
2. **Naming Validation:** Matches the target branch against your configured regex template (e.g. `issue/<KEY>`, `feature/<KEY>`, `epic/<KEY>`).
3. **Collision Checks:**
   - Checks if the branch already exists locally. If so, `git-brx` provides an informational message and advises running `git brx select issue/TASK-101`.
   - Checks if the branch exists on the remote `origin`.
4. **Tracker Verification:**
   - Queries your issue tracker (Jira or GitHub Issues).
   - Validates that `TASK-101` exists, checks the issue type against your branch prefix mapping (e.g., verifying that a `Bug` maps to `issue/`), and prints the issue title and current assignee.
   - Prompts for confirmation: `Are you sure [y/n]?` (bypassable in automation via `-y` or `--yes`).
5. **Checkout:** Executes `git checkout -b issue/TASK-101` from your current clean base.

### Offline Mode:
Working on an airplane or without VPN access? Use `--offline` / `-o`:
```bash
git brx create --offline issue/TASK-101
```
This skips the remote Git check and tracker API lookup, provisioning the branch locally based purely on syntax rules.

---

## 2. Branch Switching & Exploration (`git brx select`, `name`, `history`)

### Safe Branch Switching (`git brx select`)
Switching branches with raw `git checkout` can lead to lost work, detached HEAD states, or dirty collisions. `git brx select` provides safe switching with intelligent defaults:

```bash
# Switch back to the default branch (master or main):
git brx select

# Switch to a specific topic branch:
git brx select issue/TASK-101
```

**Guardrails Provided by `select`:**
- **Auto-Fetch:** Automatically fetches the latest remote refs before switching.
- **Behind Hint:** If you are already on the target branch and it is behind remote origin, `git-brx` warns you:
  ```text
  [git-brx] Already on 'issue/TASK-101'
  [git-brx] Hint: Your branch is behind 'origin/issue/TASK-101' by 2 commit(s). Run 'git-brx sync' to update.
  ```
- **Fuzzy Typo Suggestions:** Made a typo like `git brx select featrue/login`? `git-brx` computes Levenshtein distances:
  ```text
  [git-brx] ! Branch 'featrue/login' not found
  [git-brx] Hint: Did you mean 'feature/login'?
  ```
- **Carried-Over Changes Warning:** If you have uncommitted changes that Git carried over into the new branch, `git-brx` prominently warns you so you don't commit unrelated edits into the wrong branch.

### Inspecting Branch Identity (`git brx name`)
Prints the current branch name to `stdout`. Designed with strict Unix plumbing discipline:
```bash
CURRENT=$(git brx name)
echo "Active branch is $CURRENT"
```
If HEAD is detached, it prints the short commit SHA and logs a diagnostic warning to `stderr` without contaminating `stdout`.

### Viewing Branch Commits (`git brx history`)
`git brx history` renders a clean, colorized commit graph:
```bash
# View recent history:
git brx history -l 10

# View only commits on your active topic branch relative to master:
git brx history --topic

# Search commit messages:
git brx history -s "database migration"

# View commit diffstat:
git brx history --stat
```

---

## 3. Leased Publishing (`git brx publish`)

When you are ready to push your branch to the remote origin or trigger CI builds:

```bash
git brx publish
```

### Safety by Default: Force-With-Lease
In raw Git, developers often alternate dangerously between `git push` (which fails if history was rebased) and `git push -f` (which blindly wipes out remote commits made by colleagues).

`git-brx publish` uses **`--force-with-lease`** automatically:
- If this is your first time publishing, it executes `git push --set-upstream origin <branch> --force-with-lease`.
- If someone else pushed commits to your remote branch since you last fetched, `git brx publish` **rejects the push** and protects the remote work:
  ```text
  [git-brx] ! Publish rejected: remote has newer commits
  [git-brx] Hint: Run 'git-brx update' to incorporate remote changes before publishing
  ```

---

## 4. Code Review Submission (`git brx review`)

Once your branch is published, tested, and ready for peer review, create your pull request directly from the terminal:

```bash
git brx review
```

### Automated PR Assembly:
1. **Publication Check:** Verifies that your local branch has actually been pushed to `origin`. If not, it halts and directs you to run `git brx publish`.
2. **Issue Metadata Extraction:** Parses the issue key from your branch name and retrieves the issue summary, description, and component tags from Jira/GitHub.
3. **Reviewer Routing:** Evaluates modified files or issue components against `.git-brx.yaml`'s `review.mapping`. If multiple reviewers are configured for a component, it automatically balances the load.
4. **PR Description & Checklist:** Loads your team's review template (e.g. `.github/pull_request_template.md`) and appends standardized **Merge Instructions**:
   ```markdown
   # Merge instructions
   - **Strategy:** Squash & Merge (recommended)
     - **Commit Title:** `TASK-101: Add user authentication API`
   - **Branch Cleanup:** Delete source branch `issue/TASK-101` after merging
   ```
5. **PR Submission:** Calls the GitHub or Bitbucket Server REST API, prints the clickable PR URL to `stdout`, and logs confirmation to `stderr`.

**Dry-Run Mode:**
Want to preview the generated PR description and assigned reviewers before creating it?
```bash
git brx review --dry-run
```

---

## 5. Safe Post-Merge Deletion (`git brx delete`)

Once your pull request is approved and merged on GitHub or Bitbucket, you need to clean up your local and remote workspace:

```bash
git brx delete
```

### How `delete` Protects You:
1. **Remote Merge Verification:** `git brx delete` checks whether your branch still exists on the remote server.
2. **SCM Diagnostics:** If the branch is still on the remote, it queries the SCM provider API:
   - If the PR is still **Open**, it blocks deletion and reminds you to finish the review first.
   - If the PR is **Merged**, it informs you that the PR was merged and directs you to delete the remote branch on the server (or use `--force`).
3. **Safe Return to Base:** Switches your local worktree back to `master` (or `main`).
4. **Local Removal & Pruning:** Deletes the local topic branch (`git branch -D`) and prunes stale remote tracking references (`git remote prune origin`).
5. **Next Step Hint:** Reminds you:
   ```text
   [git-brx] Deleted local branch 'issue/TASK-101' and pruned origin
   [git-brx] Hint: Your local 'master' may not be up-to-date. Use 'git-brx update' to update changes.
   ```
