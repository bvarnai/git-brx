---
name: git-brx
description: >-
  Runbook for using git-brx for Git branch lifecycle management, ticket branch creation,
  upstream synchronization, pair programming updates, pull requests, and branch cleanup.
  Activate when creating branches, working on issues, syncing branches, pushing PRs,
  or managing git workflow.
---

# `git-brx` Git Workflow Runbook

This skill guides the AI assistant in using `git-brx` (or `git brx`) as the unified CLI for branch lifecycle management.

> [!IMPORTANT]
> **Never use raw Git commands** for branch creation (`git checkout -b`), force pushing (`git push -f`), or ad-hoc pull request creation when `git-brx` is available. Always use the corresponding `git brx` subcommands.

---

## 1. Quick Reference & Command Matrix

| Stage | `git-brx` Command | Purpose & Behavior |
| :--- | :--- | :--- |
| **Branch creation** | `git brx create [--base <base>] <branch> -y` | Validates against issue tracker, forks from `<base>` (recording `brx-base`), or auto-switches if `<branch>` already exists. |
| **Branch selection** | `git brx select [--no-ff] [<branch>]` | Fetches remotes, switches working tree (default: `main`), and fast-forwards clean behind-origin branches. |
| **Upstream sync** | `git brx sync [<target>]` | Pulls base branch changes (**rebase** for `issue/`, **merge** for `feature/` and `epic/`). Uses recorded `brx-base`, `<target>`, or `main` (with remote-only/worktree fallback). |
| **Pair Update** | `git brx update` | Queries `origin` directly and pulls colleague commits (linear rebase for `issue/`, `--rebase-merges` for `feature/` and `epic/`). |
| **Conflict resolve** | `git brx resolve [--no-tool]` | Auto-stages marker-free resolved paths (or launches merge tool) and continues in-flight rebase/merge. |
| **Conflict abort** | `git brx reset` | Aborts in-flight rebase/merge preserving local commits; resets to `origin/<branch>` when idle. |
| **Publishing** | `git brx publish` | Pushes branch (`--force-with-lease` on `issue/`/`feature/`, fast-forward-only on `epic/`/`release/`) and binds upstream tracking. |
| **Code Review / PR** | `git brx review [--draft] [<target>]` | Verifies pushed `HEAD` and remote `<target>`, resolves recorded `brx-base`, assigns reviewers, and opens PR. |
| **Branch Cleanup** | `git brx delete [--delete-remote] [<branch>]` | Checks remote deletion (or deletes merged remote with `--delete-remote`), returns to recorded `brx-base`/`main` (detaching in busy worktrees), and prunes local branch. |

---

## 2. Standard End-to-End Workflow

### Step 1: Branch creation (`create`)
When starting work on an issue or ticket:
1. Ensure your local default branch is current:
   ```bash
   git brx select main
   git brx update
   ```
2. Select the branch prefix based on scope and collaboration model (the prefix determines `sync` and PR merge strategies):
   - `issue/*`: Single-PR bug fixes, tasks, and individual vertical slices of an epic (rebase sync, squash and merge).
   - `feature/*`: Collaborative single-PR branches shared by multiple contributors (merge sync, squash and merge).
   - `epic/*`: Multi-PR integration branches aggregating child slices (merge sync, merge commit into `main`).
   The user confirms the branch type when creating standalone branches.
3. **GitHub `epic` label prerequisite:** GitHub repositories do not include an `epic` label by default. Before running `git brx create epic/<E> -y`, ensure the `epic` label exists in the repository's GitHub label settings and is applied to issue `#<E>`. Child slice tickets use standard labels (`enhancement`, `bug`, `feature`, or unlabeled for `issue/*`).
4. Create the branch non-interactively using `-y` (pass `--base <branch>` to fork from a specific base such as `epic/<E>` without switching first):
   ```bash
   git brx create issue/123 -y
   ```
   *(If the branch already exists locally or on `origin`, `create` switches to it automatically. If offline or bypassing tracker validation, add `-o`/`--offline`.)*

### Step 2: Local Commits
- Make incremental changes.
- Commit message format comes from the project's own conventions (for example the `agy-standards` plugin). `git-brx` writes the squash commit `<type>(#<key>): <issue title>` when the PR is merged.

### Step 3: Upstream Synchronization (`sync` & `update`)
- **Syncing with Base Branch (`main`):**
  Run regularly before publishing or opening review:
  ```bash
  git brx sync
  ```
  - For `issue/*`: Executes a linear rebase on top of latest `main`.
  - For `feature/*`: Creates a true merge commit from `main`.
- **Syncing with Collaborator on Shared Branch:**
  If a teammate pushed commits to the same work branch:
  ```bash
  git brx update
  ```

### Step 4: Conflict handling (`resolve` / `reset`)
If `git brx sync` or `update` encounters a conflict (Exit Code 5):
1. **To resolve:** Inspect conflicted files (`git status`), edit and remove all conflict markers (`<<<<<<<`, `=======`, `>>>>>>>`), then run:
   ```bash
   git brx resolve
   ```
   `resolve` detects that conflict markers are cleared, skips `git mergetool`, stages only the resolved paths, and continues the rebase/merge. Pass `--no-tool` in automated sessions to fail fast listing any files that still have markers.
2. **To abort:** Abort the in-flight rebase or merge while preserving unpushed local commits:
   ```bash
   git brx reset
   ```
   *(Running `git brx reset` again when idle discards local commits and resets to `origin/<branch>`.)*

### Step 5: Publishing (`publish`)
Push the local branch and configure remote tracking safely:
```bash
git brx publish
```
*(If rejected due to newer remote commits, run `git brx update` first).*

### Step 6: Code review & pull request (`review`)
Create the PR with automated reviewer assignment and merge instructions:
```bash
git brx review [--draft] [<target>]
```
- Resolves `<target>` in order: explicit `<target>`, recorded `branch.<name>.brx-base`, then default branch (`main`).
- Verifies that `<branch>` matches `origin/<branch>` `HEAD` SHA and `<target>` exists on `origin`.
- Assigns reviewers via `-r` (repeatable/comma-separated) or `.git-brx.yaml` (`review.mapping.*`, skipped for `--draft`), excluding the author.
- Child `issue/*` and `feature/*` PRs targeting `epic/*` default to **Squash and merge**, while `epic/*` PRs targeting `main` automatically specify non-squash **Create a merge commit** instructions.

### Step 7: Post-merge cleanup (`delete`)
Once the PR is merged (with auto-delete enabled or via `--delete-remote`):
```bash
git brx delete [<branch-name>]
```
*(Note: When deleting the active branch, `git brx delete` returns to its recorded `brx-base` or `main` and fast-forwards if clean and behind `origin`, or detaches `HEAD` when that branch is checked out in another worktree.)*

---

## 3. Multi-PR epic workflow (`epic/<E>`)

Use an `epic/<E>` branch as a long-lived integration branch when a feature or epic spans multiple child PRs:

1. **Provision and publish the epic branch from `main`:**
   Ensure the `epic` label exists on GitHub issue `#<E>`, then create and publish `epic/<E>`:
   ```bash
   git brx select main
   git brx update
   git brx create epic/<E> -y
   git brx publish
   ```
2. **Fork a child slice (`issue/<N>`) off an up-to-date `epic/<E>`:**
   ```bash
   git brx select epic/<E>
   git brx create issue/<N> -y
   ```
   *(Or fork directly with `git brx create --base epic/<E> issue/<N> -y`; `create` records `branch.issue/<N>.brx-base=epic/<E>`, and `select epic/<E>` fast-forwards clean behind-origin branches automatically.)*
3. **Sync, publish, and open the child PR against `epic/<E>`:**
   `git brx sync` and `git brx review` use the recorded `branch.issue/<N>.brx-base` (`epic/<E>`) automatically when `<target>` is omitted:
   ```bash
   git brx sync
   git brx publish
   git brx review
   ```
   Child `issue/*` and `feature/*` PRs targeting `epic/<E>` emit **Squash and merge** instructions by default.
4. **Clean up merged child slice and return to `epic/<E>`:**
   Because `create` records `branch.issue/<N>.brx-base=epic/<E>`, `git brx delete` switches directly back to `epic/<E>` and fast-forwards it to match `origin/epic/<E>` (or run `git brx select epic/<E>` and `git brx update` if it had local changes):
   ```bash
   git brx delete
   ```
5. **Close the epic branch into `main`:**
   After all child slices merge into `epic/<E>`, sync with `main`, publish, and open the integration PR:
   ```bash
   git brx sync
   git brx publish
   git brx review
   ```
   `git brx review` targets `main` and emits **Create a merge commit** instructions to preserve the squashed slice commits on `main`. After the epic PR merges, clean up:
   ```bash
   git brx delete epic/<E>
   git brx update
   ```

---

## 4. Automation guidelines for AI agents

- **Flag Usage:** Always pass `-y` / `--yes` to commands that support confirmation prompts (`create`) to avoid hanging non-interactive sessions.
- **Diagnostic Inspection:** Check exit codes per spec:
  - `0`: Success / already exists
  - `3`: Git repository precondition error (detached HEAD, shallow clone)
  - `4`: Remote network / unreachable error
  - `5`: Merge/rebase conflict active (invoke `resolve` or `reset`)
  - `6`: Tracker or SCM API authorization/not found error
  - `8`: Configuration regex or branch naming violation
