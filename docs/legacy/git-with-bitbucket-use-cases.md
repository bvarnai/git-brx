# Git with Bitbucket Server User's Guide - Use-Cases

This document details practical development workflows and use-cases when working with Git and Bitbucket Server using the legacy `branch` tool.

---

## 1. Working on a Bug / Issue

Typical bug fixing flow from issue start to cleanup:

1. **Create the branch:**
   ```bash
   $ git branch-create issue/VSB-123
   ```
   Validates JIRA status and creates the branch locally.

2. **Commit changes:**
   ```bash
   $ git add <files>
   $ git commit -m "VSB-123 Fix issue description"
   ```

3. **Publish the branch:**
   ```bash
   $ git branch-publish
   ```
   Pushes the branch upstream and creates tracking.

4. **Initiate code review:**
   ```bash
   $ git branch-review
   ```
   Generates a Bitbucket Pull Request targeting `master`.

5. **Merge on Bitbucket:**
   Once approved and CI checks pass, merge using **Squash, fast-forward only** and check **Delete source branch after merging**.

6. **Clean up locally:**
   ```bash
   $ git branch-delete
   ```
   Verifies remote branch is gone, switches back to `master`, updates, and deletes the local branch.

---

## 2. Conflict Resolution on an Issue Branch

When working on an issue branch (e.g. `issue/VSB-001`), the `master` branch may advance with conflicting commits. The Bitbucket pull-request view will warn about merge conflicts.

### Syncing with Master
To incorporate latest changes from `master`:
```bash
$ git branch-sync
```

If conflicts occur during the rebase operation, `git branch-sync` outputs an error:
```text
[branch] Syncing 'master' branch by default
Fetching origin
First, rewinding head to replay your work on top of it...
Applying: VSB-5294 RteBswDisabledInModes container is generated automatiocally (+ VSB-4303)
...
CONFLICT (content): Merge conflict in RCPTT/Tests/VSB_ECGGeneralWizard_422/TestCases/S03e. RTE Base pages.test
CONFLICT (content): Merge conflict in RCPTT/Tests/VSB_ECGGeneralWizard_422/TestCases/S02a. Check default options and search (part 1) - 'Generation Options' page.test
error: Failed to merge in the changes.
hint: Use 'git am --show-current-patch' to see the failed patch
Patch failed at 0001 VSB-5294 RteBswDisabledInModes container is generated automatiocally (+ VSB-4303)
Resolve all conflicts manually, mark them as resolved with
"git add/rm <conflicted_files>", then run "git rebase --continue".
You can instead skip this commit: run "git rebase --skip".
To abort and get back to the state before "git rebase", run "git rebase --abort".
[branch] ! Sync failed
```

### Resolving Conflicts with `branch-resolve`
Rather than running manual git rebase steps, use:
```bash
$ git branch-resolve
```
This launches the configured merge utility (e.g., diff tool) for all conflicting files sequentially, stages resolved files, and continues the rebase:
```text
Merging:
RCPTT/Tests/VSB_ECGGeneralWizard_422/TestCases/S02a. Check default options and search (part 1) - 'Generation Options' page.test
RCPTT/Tests/VSB_ECGGeneralWizard_422/TestCases/S03e. RTE Base pages.test
Normal merge conflict for 'RCPTT/Tests/VSB_ECGGeneralWizard_422/TestCases/S02a. Check default options and search (part 1) - 'Generation Options' page.test':
  {local}: modified file
  {remote}: modified file
[branch] You are middle of a rebase, continuing
Applying: VSB-5294 RteBswDisabledInModes container is generated automatiocally (+ VSB-4303)
```
After resolution succeeds, push the rebased branch:
```bash
$ git branch-publish -f # or git branch-publish
```

---

## 3. Sync Fails on Issue Branch (Base Branch Out-of-Date)

Syncing an `issue/` branch requires an up-to-date local `master` branch to rebase behind the scenes.
The tool intentionally does not auto-update local `master` to guarantee that uncommitted/staged local work on other branches is never endangered.

**Resolution procedure:**
1. Switch to `master`:
   ```bash
   $ git branch-select master
   ```
2. Update local `master`:
   ```bash
   $ git branch-update
   ```
3. Return to the issue branch:
   ```bash
   $ git branch-select issue/<id>
   ```
4. Perform the sync:
   ```bash
   $ git branch-sync
   ```

---

## 4. Solving an Issue in Parts (Multi-Stage Merges)

Occasionally, developers work on an "umbrella" issue requiring multiple incremental merges to `master` as soon as individual parts are completed (though single PRs are preferred whenever possible).

**Workflow:**
Repeat the create -> commit -> review -> merge -> delete cycle while reusing the same branch identifier:

- **Part 1:**
  1. `git branch-create issue/VSB-123`
  2. Implement changes & commit
  3. `git branch-publish`
  4. `git branch-review`
  5. Merge PR 1 in Bitbucket (Squash & delete remote branch)
  6. `git branch-delete`
- **Part 2:**
  1. `git branch-create issue/VSB-123` (starts fresh from updated `master`)
  2. Implement changes & commit
  3. `git branch-publish`
  4. `git branch-review`
  5. Merge PR 2 in Bitbucket
  6. `git branch-delete`
- **Part n:**
  Repeat until the full issue is resolved.
