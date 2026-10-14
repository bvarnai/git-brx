---
name: simulate-colleague
description: Runbook and playbooks for simulating realistic multi-developer collaboration, merge friction, pair programming, and reviewer mapping workflows between the user and colleague agent 'Alex Varnai' using git-brx.
---

# Multi-Developer Collaboration Playbooks with `git-brx`

This skill defines turn-based collaboration scenarios between **You** (Primary Developer in `~/workspace/git-brx-sandbox/me`) and **Alex Varnai** (Colleague Agent in `~/workspace/git-brx-sandbox/colleague`).

---

## Workspace Setup

- **Remote:** Dedicated GitHub sandbox repository (`git-brx-sandbox`).
- **User Workspace:** `/home/bvarnai/workspace/git-brx-sandbox/me` (Inherits your standard Git identity)
- **Colleague Workspace:** `/home/bvarnai/workspace/git-brx-sandbox/colleague` (Identity: `Alex Varnai <alexvarnai@users.noreply.github.com>`, GitHub: `alexvarnai`)
- **Credentials:**
  - Primary Developer: Inherited `GITHUB_TOKEN`
  - Colleague Alex Varnai: Own personal access token `ALEX_GITHUB_TOKEN` (collaborator on sandbox repo)
- **Shell Activation:**
  - Primary Developer: `source ~/workspace/git-brx-sandbox/env.sh`
  - Colleague Alex Varnai: `source ~/workspace/git-brx-sandbox/env-colleague.sh`
- **CLI Tool:** `git brx` or `./bin/git-brx`

---

## Turn-Based Interaction Pattern

When interacting with Alex Varnai:
1. **Activate Your Terminal:** Run `source ~/workspace/git-brx-sandbox/env.sh && cd ~/workspace/git-brx-sandbox/me`.
2. **Request an Action:** In chat, specify what Alex should do (e.g. *"Alex, checkout main, add a function in calc.go, and push to origin/main"*).
3. **Alex Executes & Reports:** Alex performs the action strictly within `git-brx-sandbox/colleague` and reports back the commit hash / branch name and the exact command for you to run.
4. **You Execute & Confirm:** You run your command in `git-brx-sandbox/me` and reply with your results.

For full architectural details and message flows, see [Colleague Simulation Guide](../../../docs/user-guide/colleague-simulation.md).

---

## Sequential Execution & Inter-Playbook Transition Protocol

When running playbooks sequentially, **always ensure your local workspace starts from an up-to-date `main` branch**:

```bash
git checkout main
git pull origin main
```

This prevents topic branches from accidentally inheriting unmerged commits from preceding playbooks, avoids dirty worktree conflicts, and guarantees clean upstream rebase histories.

---

## Playbook 1: Concurrent Feature Merge & Upstream Sync (`git brx sync`)

### Objective
Verify that a developer working on a topic branch can smoothly rebase over a colleague's newly merged commits using `git brx sync`.

### Steps:
1. **[User]** In `git-brx-sandbox/me`, create a topic branch using seeded Issue #2:
   ```bash
   git brx create feature/2 -y
   ```
   Add a `Divide` function to `calc.go` and commit.
2. **[Colleague (Alex)]** Concurrently in `git-brx-sandbox/colleague`:
   - Checks out `feature/1` using `git brx create feature/1 -y`.
   - Adds a `Subtract` function to `calc.go`, commits, publishes (`git brx publish`), and merges into `main` on origin.
   - Alex reports: *"Merged feature/1 into origin/main. Now run 'git brx sync' in your workspace!"*
3. **[User]** In `git-brx-sandbox/me`, run:
   ```bash
   git brx sync
   ```
   Verify that `git-brx sync` fetches `origin/main`, detects that `main` is ahead, and rebases `feature/2` on top of Alex's merged changes without conflicts.

---

## Playbook 2: Conflict Injection & Resolution (`git brx resolve` / `git brx reset`)

### Objective
Simulate a merge conflict between concurrent edits on the same lines, and verify diagnostic exit codes (Exit 5) and resolution workflows.

### Steps:
1. **[User]** On `feature/3` in `git-brx-sandbox/me`, modify line 4 of `config.json` to `"retries": 5`. Commit the change.
2. **[Colleague (Alex)]** On `main` in `git-brx-sandbox/colleague`, modify line 4 of `config.json` to `"retries": 10`, commit, and push directly to `origin/main`.
   - Alex reports: *"Pushed conflicting config change to origin/main. Run 'git brx sync' to observe the conflict!"*
3. **[User]** In `git-brx-sandbox/me`, run:
   ```bash
   git brx sync
   ```
   - Verify `git-brx sync` stops with Exit code 5 (`ExitConflict`).
   - Verify diagnostic hint: `"Use 'git-brx resolve' to resolve conflicts, or 'git-brx reset' to abort"`.
4. **[Resolution Verification]**:
   - Option A: Resolve the conflict in `config.json`, then run `git brx resolve`. Verify rebase completes cleanly.
   - Option B: Abort using `git brx reset`. Verify working directory is restored to pre-sync state.

---

## Playbook 3: Pair Programming on a Shared Published Branch (`git brx update`)

### Objective
Simulate two developers collaborating on the same published topic branch.

### Steps:
1. **[User]** Create and publish a shared topic branch using seeded Issue #4:
   ```bash
   git brx create feature/4 -y
   git brx publish
   ```
2. **[Colleague (Alex)]** In `git-brx-sandbox/colleague`:
   - Runs `git fetch origin` and selects the branch:
     ```bash
     git brx select feature/4
     ```
   - Adds a commit and pushes to origin.
   - Alex reports: *"Added step 2 to the pipeline on origin/feature/4. Run 'git brx update' to pull my changes!"*
3. **[User]** In `git-brx-sandbox/me`, run:
   ```bash
   git brx update
   ```
   Verify that `git-brx update` rebases cleanly over Alex's remote commit with `--autostash`.

---

## Playbook 4: Topic Branch Deletion After Merge (`git brx delete`)

### Objective
Verify that `git brx delete` protects unmerged remote branches and cleanly prunes merged branches.

### Steps:
1. After merging a PR on GitHub, delete the remote branch on origin.
2. In `git-brx-sandbox/me`, run:
   ```bash
   git brx delete <branch-name>
   ```
3. Verify that `git-brx` deletes the local branch, prunes the stale remote tracking ref, and switches back to the default branch.

---

## Playbook 5: Cross-Developer Reviewer Routing & Author Exclusion (`git brx review`)

### Objective
Verify that `git brx review` accurately assigns `alexvarnai` for documentation changes (`docs/**`), routes all other changes (or author fallback) to the lead (`bvarnai`), and prevents self-assignment via author exclusion.

### Setup (.git-brx.yaml in sandbox):
```yaml
scm:
  provider: github
  owner: bvarnai
  repo: git-brx-sandbox

review:
  merge:
    instructions: true
  mapping:
    paths:
      "docs/**": [alexvarnai]
    default: [bvarnai]
```

### Steps:

#### Scenario A: Primary Developer touches docs (routes to Alex Varnai)
1. **[User]** In `git-brx-sandbox/me`, ensure clean main, create branch for seeded Issue #5, commit docs, and publish:
   ```bash
   git checkout main && git pull origin main
   git brx create feature/5 -y
   mkdir -p docs && echo "# Architecture" > docs/architecture.md
   git add docs/architecture.md
   git commit -m "docs(#5): add architecture documentation"
   git brx publish
   ```
2. **[User]** Submit for review:
   ```bash
   git brx review
   ```
3. **Verification:**
   - `git-brx review` matches `docs/architecture.md` against `mapping.paths` (`docs/**`) $\to$ resolves `alexvarnai`.
   - Your author handle (`bvarnai`) is retained/checked.
   - GitHub PR is opened with `alexvarnai` requested as reviewer.
   - Inspect PR on GitHub to confirm `alexvarnai` appears in the "Reviewers" list.

#### Scenario B: Colleague Alex touches code (cascades to Lead bvarnai)
1. **[Colleague (Alex Varnai)]** In `git-brx-sandbox/colleague`:
   - Checks out branch for seeded Issue #6, modifies code (`calc.go`), commits as `Alex Varnai`, and publishes:
     ```bash
     source ~/workspace/git-brx-sandbox/env-colleague.sh
     git checkout main && git pull origin main
     git brx create feature/6 -y
     # Edit calc.go
     git commit -am "feat(#6): add power exponent calculation"
     git brx publish
     ```
   - Submits the review using Alex's own PAT:
     ```bash
     GITHUB_TOKEN="$ALEX_GITHUB_TOKEN" git brx review
     ```
2. **Verification:**
   - `git-brx review` inspects `calc.go`, finds no match under `mapping.paths`, and falls back to `mapping.default` (`bvarnai`).
   - PR is opened on GitHub requesting `bvarnai` (Lead) as reviewer.
   - Alex reports: *"Opened PR for feature/6 on GitHub. Review requested from lead bvarnai!"*

#### Scenario C: Author Exclusion (Alex touches docs)
1. **[Colleague (Alex Varnai)]** In `git-brx-sandbox/colleague`:
   - On `feature/6`, modifies `docs/architecture.md`, commits, publishes, and submits review.
2. **Verification:**
   - `mapping.paths` matches `docs/**` $\to$ candidate `alexvarnai`.
   - **Author Exclusion:** `alexvarnai` is detected as the PR author and is excluded.
   - Cascades to `mapping.default` (`bvarnai`).
   - PR is opened with lead `bvarnai` requested as reviewer, avoiding self-assignment.

---

## Playbook 6: Non-Squash PR Merge Strategy (`merge` commit / `rebase`)

### Objective
Verify that `git brx review` renders explicit merge instructions when a PR requires a non-squash strategy (e.g., preserving full constituent commit history via a true merge commit or linear rebase), and confirm that downstream `git brx sync` correctly handles the resulting non-squashed upstream history.

### Context & Configuration
By default, `git-brx` recommends squash merges for single feature/issue branches. However, teams frequently require full history preservation:
- Multi-commit major initiatives (`epic/*` or `release/*` convention)
- Topic branches configured via `review.merge.strategies.sources` / `review.merge.strategies.targets`
- On-demand CLI overrides via `git brx review --strategy merge` or `git brx review --strategy rebase`

### Setup (`.git-brx.yaml` in sandbox):
```yaml
review:
  merge:
    instructions: true
    strategy: squash
    strategies:
      sources:
        "feature/7": merge
```

### Steps:

#### Step 1: Primary Developer opens a PR requiring a merge commit
1. **[User]** In `git-brx-sandbox/me`, create a multi-commit branch for seeded Issue #7 and publish:
   ```bash
   git checkout main && git pull origin main
   git brx create feature/7 -y
   echo "step 1" >> pipeline.txt && git add pipeline.txt && git commit -m "feat(#7): implement step 1"
   echo "step 2" >> pipeline.txt && git add pipeline.txt && git commit -m "feat(#7): implement step 2"
   git brx publish
   ```
2. **[User]** Submit for review (matching `feature/7` in config, or using explicit `--strategy merge`):
   ```bash
   git brx review
   # or with CLI override: git brx review --strategy merge
   ```
3. **Verification in PR Description:**
   Inspect the opened PR on GitHub. Verify that the generated description includes:
   ```markdown
   ### Merge instructions
   - **Strategy:** Create a merge commit
     - **Commit:** `Merge branch 'feature/7' into main` (merge commit preserving all constituent commits)
   - **Branch cleanup:** Delete source branch `feature/7` after merging
   ```

#### Step 2: Colleague Alex reviews & merges using a Merge Commit
1. **[Colleague (Alex Varnai)]** In `git-brx-sandbox/colleague`:
   - Alex reviews the PR, observes the instruction to create a merge commit instead of squash, and merges via GitHub UI using **"Create a merge commit"** (or command line equivalent).
   - Deletes the remote branch `feature/7`.
   - Alex reports: *"Merged PR for feature/7 into main with a merge commit, preserving constituent commits. Run 'git brx sync' on your other branches to test upstream rebase over a merge commit!"*

#### Step 3: Upstream Synchronization over a Merge Commit
1. **[User]** On another active topic branch in `git-brx-sandbox/me`:
   ```bash
   git brx sync
   ```
2. **Verification:**
   - Verify `git-brx sync` fetches the new merge commit and its constituent commits on `origin/main`.
   - Verify the local branch rebases smoothly on top of `origin/main` without duplicate commit conflicts.




