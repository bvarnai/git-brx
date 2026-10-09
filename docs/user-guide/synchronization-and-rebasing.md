# Synchronization, rebasing & conflict resolution

Keeping topic branches up to date with the mainline codebase is one of the most critical—and error-prone—tasks in Git. This guide covers how `git-brx` automates clean rebasing, handles remote updates, resolves merge conflicts, and provides emergency recovery mechanisms.

---

## 1. Synchronizing with base (`git brx sync`)

While you work on a feature or bugfix, your teammates are continually merging changes into `master` (or `main`). If your branch sits un-synchronized for days, divergence grows, leading to painful merge conflicts later.

Use `git brx sync` to bring in changes from your base branch:

```bash
# Sync with the default base branch (master or main):
git brx sync

# Or sync with a custom base branch (e.g. develop or a release branch):
git brx sync develop
```

### Intelligent Strategy Resolution
`git-brx sync` automatically chooses the correct synchronization strategy based on your branch prefix:
- **`issue/*` branches ➔ Rebase:** Replays your local commits cleanly on top of the newest `master` commits. This produces a linear commit history ready for Squash & Merge.
- **`feature/*` and `epic/*` branches ➔ Merge:** Performs a standard 3-way merge commit into your feature branch to preserve ongoing parallel sub-histories.

### Explicit strategy overrides
You can override the default strategy at any time:
```bash
# Force a merge even on an issue branch:
git brx sync -m

# Force a rebase even on a feature branch:
git brx sync -r
```

### Precondition checks
Before running a rebase or merge, `git brx sync` verifies that your local `master` is not diverged from `origin/master`. If your base branch is out-of-date, it halts immediately with guidance:
```text
[git-brx] ! Sync branch 'master' is not up-to-date
[git-brx] Hint: Switch to 'master' branch and use 'git-brx update' to update all changes
```

---

## 2. Updating from remote (`git brx update`)

When collaborating with another developer on the same branch, or when you switch machines, new commits will appear on `origin/<your-branch>`.

Use `git brx update` to pull remote changes into your active branch:

```bash
git brx update
```

### How `update` works:
1. Fetches the latest commits from `origin/<active-branch>`.
2. Rebases your local unpushed commits on top of the remote tracking branch (`git rebase origin/<branch>`).
3. If you have uncommitted local modifications, you can combine this with `--autostash` (`-a`) to automatically stash your dirty work, rebase, and restore your stash cleanly:
   ```bash
   git brx update --autostash
   ```

---

## 3. Resolving conflicts (`git brx resolve`)

Rebasing and merging can occasionally encounter conflicts when two developers touch the same lines of code. Raw Git expects you to manually identify conflicting files, edit conflict markers, stage them individually with `git add`, and run `git rebase --continue`.

`git brx resolve` turns this into an automated, guided procedure:

```bash
git brx resolve
```

### The Guardrailed resolution flow:
1. **Detection:** Identifies all conflicted files (`--diff-filter=U`).
2. **Interactive mergetool launch:** Launches your configured GUI diff/merge tool (Beyond Compare, VSCode, Meld, KDiff3, etc.) for each conflicted file without extra prompts.
3. **Targeted staging (no blind `git add .`):**
   - After you close the merge tool for a file, `git-brx` verifies whether conflicts in that specific file were resolved.
   - It stages **only** the resolved files (`git add -- <file>`). It **never** runs `git add .`, protecting untracked files and unrelated modifications in your workspace from accidental staging.
4. **Continuation:**
   - For rebases: Displays the current rebase step (e.g. `Resolving rebase conflict at step 2 of 5...`) and executes `git rebase --continue`.
   - For merges: Finalizes the merge commit (`git commit --no-edit`), preserving the full multi-line merge metadata.
5. **Multi-commit rebase loops:** If subsequent commits in your rebase series also conflict, `git-brx` smoothly repeats the resolution cycle until the entire branch is cleanly rebased.

---

## 4. Recovery (`git brx reset`)

What if a rebase goes horribly wrong, or you accidentally make broken changes and want to abort completely?

In raw Git, developers often guess between `git rebase --abort`, `git merge --abort`, `git checkout -- .`, or `git reset --hard HEAD`. Guessing wrong can destroy work or leave `.git` in a corrupted intermediate state.

`git brx reset` is the universal, bulletproof reset command:

```bash
git brx reset
```

### What `git brx reset` Does:
1. **Aborts in-flight operations first:** Checks for `.git/rebase-merge`, `.git/rebase-apply`, or `.git/MERGE_HEAD`. If found, it safely runs `git rebase --abort` or `git merge --abort`, restoring HEAD from detached state.
2. **Restores tracking state:** Performs `git reset --hard origin/<current-branch>`, restoring your working directory and staging area to the exact commit currently on the remote server.
3. **Optional workspace cleaning (`--clean` / `-c`):** To also remove untracked build artifacts, generated files, and editor leftovers:
   ```bash
   git brx reset --clean
   ```

You are immediately back to a known, stable, clean working state.
