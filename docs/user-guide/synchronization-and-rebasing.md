# Synchronization, rebasing & conflict resolution

This guide covers how `git-brx` syncs branches with `main`, pulls collaborator commits, resolves conflicts, and recovers from failed rebases or merges.

---

## 1. Synchronizing with base (`git brx sync`)

As teammates merge changes into `main`, run `git brx sync` to incorporate upstream commits into your topic branch:

```bash
# Sync with the default base branch (main or master):
git brx sync

# Sync with a specific target branch (e.g., develop or a release branch):
git brx sync develop
```

### Default strategy by branch prefix
`git brx sync` selects the strategy based on your active branch prefix:
- **`issue/*` $\to$ Rebase:** Replays your local commits on top of `main`, keeping history linear for a squash merge.
- **`feature/*` and `epic/*` $\to$ Merge:** Merges `main` into your branch with a 3-way merge commit so shared commit SHAs aren't rewritten.

### Strategy overrides
Override the default strategy with `-m` or `-r`:
```bash
# Force a merge on an issue branch:
git brx sync -m

# Force a rebase on a feature branch:
git brx sync -r
```

### Automatic base branch fast-forwarding
If your local `main` branch is behind `origin/main` with no local commits (`ahead == 0, behind > 0`), `git brx sync` fast-forwards local `main` in place (`git fetch origin main:main`) before rebasing or merging:

```text
[git-brx] Syncing 'issue/73' with 'main' base branch by default (strategy: rebase)
[git-brx] Fast-forwarded local 'main' to match 'origin/main' (2 commits behind)
```

To disable in-place base fast-forwarding, pass `--no-auto-update-base`:
```bash
git brx sync --no-auto-update-base
```

### Diverged base branch check
If your local base branch has unpushed commits or has diverged from `origin/main` (`ahead > 0`), `git brx sync` stops with Exit Code `5` to avoid rebasing onto unverified local state:
```text
[git-brx] Error: Base branch 'main' is not up-to-date with 'origin/main'
[git-brx] Hint: Switch to 'main' and run 'git-brx update' first
```

---

## 2. Updating from remote (`git brx update`)

When a teammate pushes commits to your shared branch—or after you merge a PR into `main`—use `git brx update` to pull commits from `origin/<current-branch>`:

```bash
git brx update
```

### How `update` works
1. Fetches `origin/<active-branch>`.
2. Fast-forwards if you have no local commits, or rebases your local unpushed commits on top of `origin/<active-branch>`.
3. With `--autostash` (`-a`), stashes uncommitted local changes before rebasing and reapplies them afterward:
   ```bash
   git brx update --autostash
   ```

---

## 3. Resolving conflicts (`git brx resolve`)

When a rebase or merge hits a conflict, run `git brx resolve`:

```bash
git brx resolve
```

### Resolution flow
1. **Find conflicted paths:** Lists unmerged files via `git diff --name-only --diff-filter=U`.
2. **Launch merge tool:** Opens your configured merge tool (`git config merge.tool`, or `-t <tool>`) for each conflicted file.
3. **Stage only resolved files:**
   - Checks each file after the merge tool exits to confirm conflict markers are gone.
   - Stages **only** those resolved paths (`git add -- <file>`). It never runs `git add .`, so unrelated working-tree edits or untracked files stay unstaged.
4. **Continue the operation:**
   - **Rebase:** Prints current step progress (e.g., `Resolving rebase conflict (step 2/5)`) and runs `git rebase --continue`.
   - **Merge:** Commits the merge (`git commit --no-edit`).
5. **Multi-step rebases:** If later commits in a rebase also conflict, `git-brx` loops through the remaining steps until the rebase completes.

---

## 4. Recovery (`git brx reset`)

To abort an in-flight rebase or merge and reset your branch to match `origin/<current-branch>`:

```bash
git brx reset
```

### What `git brx reset` does
1. **Aborts active operations:** Detects `.git/rebase-merge`, `.git/rebase-apply`, or `.git/MERGE_HEAD` and runs `git rebase --abort` or `git merge --abort`.
2. **Hard-resets to upstream:** Runs `git reset --hard origin/<current-branch>`.
3. **Optional untracked cleanup (`--clean` / `-c`):** Removes untracked files and directories (`git clean -fd`):
   ```bash
   git brx reset --clean
   ```
