---
name: simulate-colleague
description: Runbook and playbooks for simulating realistic multi-developer collaboration, merge friction, and pair-programming workflows between the user and colleague agent 'Alex Developer' using git-brx.
---

# Multi-Developer Collaboration Playbooks with `git-brx`

This skill defines turn-based collaboration scenarios between **You** (Primary Developer in `~/workspace/sandbox-user`) and **Alex Developer** (Colleague Agent in `~/workspace/sandbox-colleague`).

---

## Workspace Setup

- **Remote:** Dedicated GitHub sandbox repository (`git-brx-sandbox`).
- **User Workspace:** `/home/bvarnai/workspace/sandbox-user` (Identity: `Balazs Varnai <bvarnai@gmail.com>`)
- **Colleague Workspace:** `/home/bvarnai/workspace/sandbox-colleague` (Identity: `Alex Developer <alex.developer@example.com>`)
- **CLI Tool:** `./bin/git-brx` or `git brx`

---

## Playbook 1: Concurrent Feature Merge & Upstream Sync (`git brx sync`)

### Objective
Verify that a developer working on a topic branch can smoothly rebase over a colleague's newly merged commits using `git brx sync`.

### Steps:
1. **[User]** In `sandbox-user`, create a topic branch:
   ```bash
   ./bin/git-brx create feature/add-divide -y
   ```
   Add a `Divide` function to `calc.go` and commit.
2. **[Colleague (Alex)]** Concurrently in `sandbox-colleague`:
   - Checks out `feature/add-subtract` using `git-brx`.
   - Adds a `Subtract` function to `calc.go`, commits, publishes (`git brx publish`), and merges into `main` on origin.
   - Alex reports: *"Merged feature/add-subtract into origin/main. Now run 'git brx sync' in your workspace!"*
3. **[User]** In `sandbox-user`, run:
   ```bash
   ./bin/git-brx sync
   ```
   Verify that `git-brx sync` fetches `origin/main`, detects that `main` is ahead, and rebases `feature/add-divide` on top of Alex's merged changes without conflicts.

---

## Playbook 2: Conflict Injection & Resolution (`git brx resolve` / `git brx reset`)

### Objective
Simulate a merge conflict between concurrent edits on the same lines, and verify diagnostic exit codes (Exit 5) and resolution workflows.

### Steps:
1. **[User]** On `feature/config-tuning`, modify line 4 of `config.json` to `"retries": 5`. Commit the change.
2. **[Colleague (Alex)]** On `main` in `sandbox-colleague`, modify line 4 of `config.json` to `"retries": 10`, commit, and push directly to `origin/main`.
   - Alex reports: *"Pushed conflicting config change to origin/main. Run 'git brx sync' to observe the conflict!"*
3. **[User]** In `sandbox-user`, run:
   ```bash
   ./bin/git-brx sync
   ```
   - Verify `git-brx sync` stops with Exit code 5 (`ExitConflict`).
   - Verify diagnostic hint: `"Use 'git-brx resolve' to resolve conflicts, or 'git-brx reset' to abort"`.
4. **[Resolution Verification]**:
   - Option A: Resolve the conflict in `config.json`, then run `./bin/git-brx resolve`. Verify rebase completes cleanly.
   - Option B: Abort using `./bin/git-brx reset`. Verify working directory is restored to pre-sync state.

---

## Playbook 3: Pair Programming on a Shared Published Branch (`git brx update`)

### Objective
Simulate two developers collaborating on the same published topic branch.

### Steps:
1. **[User]** Create and publish a shared topic branch:
   ```bash
   ./bin/git-brx create feature/shared-pipeline -y
   ./bin/git-brx publish
   ```
2. **[Colleague (Alex)]** In `sandbox-colleague`:
   - Runs `git fetch origin` and selects the branch:
     ```bash
     ./bin/git-brx select feature/shared-pipeline
     ```
   - Adds a commit and pushes to origin.
   - Alex reports: *"Added step 2 to the pipeline on origin/feature/shared-pipeline. Run 'git brx update' to pull my changes!"*
3. **[User]** In `sandbox-user`, run:
   ```bash
   ./bin/git-brx update
   ```
   Verify that `git-brx update` rebases cleanly over Alex's remote commit with `--autostash`.

---

## Playbook 4: Topic Branch Deletion After Merge (`git brx delete`)

### Objective
Verify that `git brx delete` protects unmerged remote branches and cleanly prunes merged branches.

### Steps:
1. After merging a PR on GitHub, delete the remote branch on origin.
2. In `sandbox-user`, run:
   ```bash
   ./bin/git-brx delete <branch-name>
   ```
3. Verify that `git-brx` deletes the local branch, prunes the stale remote tracking ref, and switches back to the default branch.
