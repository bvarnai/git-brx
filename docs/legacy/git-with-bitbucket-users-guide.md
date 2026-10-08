# Git with Bitbucket Server User's Guide (CVI, VSB)

- **Status:** STABLE
- **Responsible:** Varnai, Balazs (DI SW PLM LCS E&H DMA AI)
- **Supported Bitbucket Server versions:** 8.19.0 (Bitbucket Server on-premises, distinct from Bitbucket Cloud)
- **Team Server:** Bitbucket Server (VSB) (`http://ies-iesd-bitbucket.ies.mentorg.com`)

---

## 1. Introduction & Workflow

To work efficiently with branches, we follow an orderly, controlled, lightweight branch-based workflow similar to GitHub Flow and feature branching.
We have a stable branch called `master`, our single source of truth. Ideally this branch should be product ready at any time.

### Workflow Lifecycle Steps
1. **Create a branch:**
   Any code change must be done on a separate branch. Changes on a branch do not affect `master`, allowing experimentation and commits safely until ready for review.
2. **Add commits:**
   Whenever you add, edit, or delete a file, you create a commit on your branch to track progress.
3. **Start code review:**
   Open a Pull Request (PR) in Bitbucket. Pull requests initiate discussion about commits and show exact diffs against the target branch. PRs can be opened early (WIP/draft/ideas) or when ready for final review.
4. **Discuss and review code:**
   Reviewers provide feedback on code style, unit tests, design, etc.
5. **Merge:**
   After approval, merge changes into `master`. PRs preserve searchable historical records of code decisions.

---

## 2. Branch Types

| Branch Type | JIRA Issue Type | Branch Pattern | Lifespan (1) | Preferred Merge Strategy (2) | Branching Model (3) |
|---|---|---|---|---|---|
| **issue** | VSB Bug Issue type<br>VSB Technical Item Issue Type<br>VSB Test Case Development Issue Type | `issue/<jira-id>`<br>*(e.g., `issue/VSB-123`)* | Short-lived | Squash, fast-forward only | Bugfix |
| **feature** | VSB Backlog Issue type | `feature/<jira-id>`<br>*(e.g., `feature/VSB-123`)* | Short-lived | Merge commit | Feature |
| **release** *(on-demand)* | N/A | `release/<milestone>`<br>*(e.g., `release/2209`)* | Permanent (4) | N/A | Release |
| **epic** *(on-demand)* | Epic | `epic/<jira-id>`<br>*(e.g., `epic/VPP-123`)* | Long-lived | Merge commit | N/A |
| **master** | N/A | `master` | Permanent | N/A | N/A |

### Notes:
1. **Lifespan:** Indicates how long a branch is expected to remain open. It is best practice to close branches quickly to prevent divergence.
2. **Preferred merge strategy:** Developers can decide the strategy at merge time based on the actual change.
3. **Branching model:** Bitbucket abstraction for categorizing branches.
4. **Release branch:** Subject to archival (to prevent stale branches), eventually converted into a tag.

---

## 3. Branch Permissions

Branch permissions in Bitbucket Server control git operations by user/user group (`administrators` vs `users`). Users in the exemption group may execute the specified operations.

| Branch | Prevented Operations | Exemption Group |
|---|---|---|
| `issue` | Nothing | Full control by developers |
| `feature` | Nothing | Full control by developers |
| `release maintenance` | Rewriting history<br>Deletion<br>Changes without a pull request | `administrators`<br>`administrators`<br>`administrators` |
| `master` | Rewriting history<br>Deletion<br>Changes without a pull request | `administrators`<br>`administrators`<br>`administrators` |

Developers working on `issue` and `feature` branches have full control over their branches.

---

## 4. Code Review & Merge Strategies

Git merge strategies define how commit history appears after merging in Bitbucket Server:

- **Merge commit (`--no-ff`):** Always creates a 3-way merge commit and updates the target branch, even if fast-forward is possible. Retains source branch history intact.
- **Squash, fast-forward only (`--squash --ff-only`):** Rejects merge if the source branch is out of date with the target branch. Otherwise, squashes all source commits into a single commit on the target branch. Keeps history clean and linear.

### Merging an Issue Branch
- **Preferred strategy:** Squash, fast-forward only.
- In Bitbucket merge dialog:
  1. Select **Squash strategy**.
  2. Enable **Delete source branch after merging**.
  3. Ensure the commit message contains issue details.

### Merging a Feature Branch
- **Preferred strategy:** Merge commit (`--no-ff`).
- In Bitbucket merge dialog:
  1. Select **Merge commit**.
  2. Enable **Delete source branch after merging**.

---

## 5. Legacy `branch` CLI Tool Overview

The `branch` tool automates development workflows via shell scripts configured as Git aliases (`git branch-<command>`).
- It complements Git to make operations simpler and safer in the project context.
- Historically installed via `./installer.sh update` (Project installer).
- **Environment requirement:** Git Bash shell only.

### Central Configuration
- Managed in the `tools` repository under `tools/conf/<project>/`:
  - `branch.json`: Primary configuration file.
  - `branch-review.template`: Markdown template for pull request descriptions.
- Contact: Varnai, Balazs (DI SW PLM LCS E&H DMA AI)

---

## 6. CLI Command Reference

Command syntax:
```bash
git branch-<command> [options] [arguments]
```
To inspect help for any command:
```bash
git branch-<command> help
```

---

### `git branch-help`
Displays generic help with a list of available commands.
```bash
$ git branch-help
```

---

### `git branch-name`
Returns the current branch name, cleanly handling detached `HEAD` state.
```bash
$ git branch-name
```

---

### `git branch-select`
Switches to an existing branch (defaults to `master`).
```bash
$ git branch-select [<branch>]
```
- **Arguments:**
  - `<branch>` *(optional)*: Target branch to select. Defaults to `master`.

**Example:**
```bash
$ git branch-select
[branch] Selecting 'master' branch by default
Fetching origin
Switched to branch 'master'
Your branch is up to date with 'origin/master'.
```

---

### `git branch-update`
Updates the current local branch using a rebase strategy (similar to `git pull --rebase`).
```bash
$ git branch-update
```
- **Notes on Rebase Strategy:** Commits are temporarily stashed, the base branch is moved to the target tip, and local commits are reapplied on top. History is rewritten without creating unnecessary merge commits.

**Example:**
```bash
$ git branch-update
Already up to date.
Current branch issue/VSB-2 is up to date.
```

---

### `git branch-create`
Creates a new development branch.
```bash
$ git branch-create [-o|--offline] <branch>
```
- **Arguments:**
  - `<branch>` *(mandatory)*: Name of the branch to create (e.g., `issue/VSB-9999`).
- **Options:**
  - `-o`, `--offline`: Skip online verification checks against JIRA / Bitbucket.

**Example:**
```bash
$ git branch-create issue/VSB-9999
[branch] Checking for existing branches (local)                                       
[branch] No branch 'issue/VSB-9999' found (local)                                     
[branch] Checking for existing branches (remote)                                     
[branch] No branch 'issue/VSB-9999' found (remote)                                    
[branch] Authentication required. Sign in to your MGC account                         
Username: jdoe                                                                     
Password:                                                                             
[branch] Issue 'VSB-9999' is assigned to 'jdoe (John Doe)' in status 'Open' 
Are you sure [y/n]?: y                                                                
[branch] Creating branch 'issue/VSB-9999'                                             
Switched to a new branch 'issue/VSB-9999'
[branch] Hint: Use 'git branch-publish' to publish a new branch                       
```

---

### `git branch-publish`
Pushes local commits to the remote and sets upstream tracking. Also used to publish newly created branches.
```bash
$ git branch-publish
```

**Example:**
```bash
$ git branch-publish
Total 0 (delta 0), reused 0 (delta 0)
remote:
remote: Create pull request for issue/VSB-2:
remote:   http://ies-iesd-bitbucket.ies.mentorg.com/projects/VSB/repos/vsb/compare/commits?sourceBranch=refs/heads/issue/VSB-2
remote:
To http://ies-iesd-bitbucket.ies.mentorg.com/scm/vsb/vsb.git
 * [new branch]      issue/VSB-2 -> issue/VSB-2
Branch 'issue/VSB-2' set up to track remote branch 'issue/VSB-2' from 'origin'.
```

---

### `git branch-review`
Creates a pull request in Bitbucket Server. Defaults source to the current branch and target to `master`.
```bash
$ git branch-review [<branch>]
```
- **Arguments:**
  - `<branch>` *(optional)*: Target branch to merge into. Defaults to `master`.

**Example:**
```bash
$ git branch-review
[branch] Authentication required. Sign in to your MGC account                                                
Username: jdoe                                                                                                    
Password:                                                                                                       
[branch] Created pull-request [http://ies-iesd-bitbucket.ies.mentorg.com/projects/VSB/repos/vsb/pull-requests/26]
```

---

### `git branch-sync`
Syncs changes from the upstream branch into the current branch using the predefined strategy for the branch type.
```bash
$ git branch-sync [-m|--merge] [-r|--rebase] [-a|--autostash] [-i|--interactive] [<branch>]
```
- **Arguments:**
  - `<branch>` *(optional)*: Source branch to sync from. Defaults to `master`.
- **Options:**
  - `-m`, `--merge`: Force merge strategy.
  - `-r`, `--rebase`: Force rebase strategy.
  - `-a`, `--autostash`: Enable autostash mode (rebase only).
  - `-i`, `--interactive`: Enable interactive rebase mode (rebase only).

#### Strategy Mapping & Overrides
| Current Branch Type | Sync Source | Rebase Strategy | Merge Strategy | Notes |
|---|---|---|---|---|
| **issue** | `master` | **Default** | Allowed with `--merge` override | Exclusively used by single developer; rebase is safe. |
| **feature** | `master` | Allowed with `--rebase` override | **Default** | Often single developer, but multi-contributor possible. |
| **release maintenance** | Any | Not allowed | **Allowed** | Merge commits preserved. |
| **master** | Any | Not allowed | **Allowed** | Rebase strictly forbidden on master. |

**Example:**
```bash
$ git branch-sync
[branch] Syncing 'master' branch by default
Fetching origin
First, rewinding head to replay your work on top of it...
Applying: Added new feature
```

---

### `git branch-resolve`
Assists with conflict resolution during rebase/merge by launching the 3-way merge utility for conflicted files and continuing the rebase/merge sequence.
```bash
$ git branch-resolve
```

---

### `git branch-reset`
Discards all local uncommitted changes and unpushed modifications, restoring the exact state from `origin`.
```bash
$ git branch-reset
```

---

### `git branch-delete`
Deletes the local branch. Deletion is permitted only if the branch has already been deleted on the remote repository (e.g., via Bitbucket after PR merge). After validation, switches back to `master` and prunes remote tracking.
```bash
$ git branch-delete
```

**Example:**
```bash
$ git branch-delete
[branch] Checking for existing branches (remote)
[branch] No branch 'issue/VSB-2' found (remote)
[branch] Switching to 'master' branch
Fetching origin
...
From http://ies-iesd-bitbucket.ies.mentorg.com/scm/vsb/vsb
   dd51924..e827c9c  master     -> origin/master
Switched to branch 'master'
Your branch is behind 'origin/master' by 1 commit, and can be fast-forwarded.
  (use "git pull" to update your local branch)
Deleted branch issue/VSB-2 (was a5d5093).
Pruning origin
URL: http://ies-iesd-bitbucket.ies.mentorg.com/scm/vsb/vsb.git
 * [pruned] origin/issue/VSB-2
```

---

### `git branch-history`
Displays a clean, formatted commit tree/graph with commit hashes, dates, messages, and authors.
```bash
$ git branch-history
```

**Example Output:**
```text
*   7c27b3c 2019-07-02 | Merge pull request #2 in VSB/vsb from issue/VSB-2-fix-branch-inclusion-pattern to master [Balazs Varnai]
|\
| * 7b657c3 2019-07-02 | Added extra formatting [Balazs Varnai]
| * c0975dd 2019-07-01 | Print all environment variables [Balazs Varnai]
| * 6f5bb66 2019-07-01 | Cosmetic change [Balazs Varnai]
|/
*   8f14e53 2019-07-01 | Merge pull request #1 in VSB/vsb from feature/VSB-1-add-jenkins-automation to master [Balazs Varnai]
|\
| * 7eb1a0e 2019-07-01 | Restrict running on master only [Balazs Varnai]
| * fc19b39 2019-07-01 | Added Maven build settings [Balazs Varnai]
| * 944551b 2019-07-01 | Add minimal Jenkinsfile [Balazs Varnai]
|/
* 8d63ba8 2019-07-01 | Initial Commit [Balazs Varnai]
```
