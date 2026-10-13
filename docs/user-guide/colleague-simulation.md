# Colleague simulation & multi-developer testing guide

Testing real-world collaborative Git workflows—such as rebasing over upstream merges, resolving merge conflicts, and pair-programming on shared branches—normally requires multiple developers or tedious manual juggling of Git clones and identities.

`git-brx` includes an end-to-end **Colleague Simulation Harness** featuring **"Alex Developer"**, an autonomous AI peer agent running in Antigravity CLI (`agy`). Alex operates in an isolated workspace clone with their own Git author identity, allowing you to test complex, multi-developer scenarios side-by-side in real time.

---

## 1. Architecture & workspace topology

```mermaid
flowchart LR
    subgraph Host["Developer Machine"]
        UserWorkspace["User Workspace<br/>~/workspace/git-brx-sandbox/me<br/>Author: Inherited Git identity"]
        ColleagueWorkspace["Colleague Workspace<br/>~/workspace/git-brx-sandbox/colleague<br/>Author: Alex Developer"]
    end

    subgraph GitHub["GitHub Remote Server"]
        SandboxRepo["Remote: git-brx-sandbox<br/>(origin)"]
    end

    UserWorkspace <-->|git-brx push/sync| SandboxRepo
    ColleagueWorkspace <-->|git-brx push/sync| SandboxRepo
```

### Identity decoupling
Both workspaces target the same GitHub repository (`git-brx-sandbox`), but use distinct local Git configurations:
- **User Workspace (`git-brx-sandbox/me`):** Inherits your standard global Git settings (`git config user.name`/`user.email`) exactly like your usual projects.
- **Colleague Workspace (`git-brx-sandbox/colleague`):** Configured with Alex's identity (`Alex Developer <alex.developer@example.com>`).

This guarantees that Git commits, author history, and logs cleanly distinguish between both engineers.

---

## 2. One-time setup & shell activation

Run the automated provisioning script from the `git-brx` root:

```bash
GITHUB_TOKEN="<your_pat>" ./scripts/setup-sandbox.sh
```

### What the setup script does:
1. Dynamically detects your authenticated GitHub account and ensures `git-brx-sandbox` exists.
2. Seeds test issues (#1..#4) for turnkey scenario execution.
3. Clones the repository into `~/workspace/git-brx-sandbox/me`.
4. Clones the repository into `~/workspace/git-brx-sandbox/colleague`.
5. Generates `~/workspace/git-brx-sandbox/env.sh`.

### Activating your shell
In any terminal where you want to work as yourself:

```bash
source ~/workspace/git-brx-sandbox/env.sh
cd ~/workspace/git-brx-sandbox/me
```

This immediately:
- Prepends `git-brx/bin` to your `PATH` so `git brx` executes the local compiled binary.
- Enables Bash tab autocompletion for both `git-brx` and `git brx`.

---

## 3. Turn-based interaction model

To ensure you maintain complete control over each step and are never rushed, the simulation follows a **turn-based checkpoint model**:

```mermaid
sequenceDiagram
    autonumber
    actor You as You (Terminal: ~/workspace/sandbox-user)
    participant Chat as Antigravity Chat
    participant Alex as Alex Developer (~/workspace/sandbox-colleague)
    participant GitHub as Remote GitHub Sandbox Repo

    Note over You,Alex: Phase 1: You Request a Collaborative Action
    You->>Chat: "Alex, let's test Playbook 2: inject a conflict into config.json"
    Chat->>Alex: Dispatches task to Alex in colleague workspace
    Alex->>GitHub: Modifies config.json line 4 to "retries": 10 and pushes to origin/main
    Alex-->>Chat: Reports completion & exact handoff command
    Chat-->>You: "[Alex]: Pushed commit a1b2c3d to origin/main. Now run 'git brx sync' in your workspace!"

    Note over You,Alex: Phase 2: You Execute in Your Workspace
    You->>You: Run './bin/git-brx sync' in ~/workspace/git-brx-sandbox/me
    Note right of You: Observes Exit code 5 (ExitConflict) and diagnostic hints
    You->>You: Resolves conflict or runs './bin/git-brx reset'

    Note over You,Alex: Phase 3: You Report Back
    You->>Chat: "I resolved the conflict with 'git brx resolve' and published my branch"
    Chat->>Alex: Next instruction or verification
```

---

## 4. Example message flows

### Example Flow A: Simulating an upstream merge & rebase sync (`git brx sync`)

#### Prompt to Antigravity:
> *"Alex, I am starting work on `feature/1` in `git-brx-sandbox/me`. While I work on this, please implement a new function in `calc.go`, commit it, and merge it directly into `main` on origin."*

#### Alex's response in Chat:
> `[Alex Developer]:`
> I switched to `main` in `~/workspace/git-brx-sandbox/colleague`, implemented `func Subtract(a, b int) int` in `calc.go`, and pushed commit `7f21a9c` directly to `origin/main`.
>
> **Your Turn:**
> Switch to `~/workspace/git-brx-sandbox/me` and run:
> ```bash
> git brx sync
> ```
> `git-brx` will fetch `origin/main`, detect that `main` is ahead by 1 commit, and cleanly rebase your `feature/1` branch on top of my new commit!

---

### Example Flow B: Injecting a merge conflict (`git brx resolve` / `git brx reset`)

#### Prompt to Antigravity:
> *"Alex, let's simulate a merge conflict. I just changed line 4 of `config.json` on my branch to `"retries": 5`. Please push a conflicting change on `origin/main`."*

#### Alex's response in Chat:
> `[Alex Developer]:`
> I've edited line 4 of `config.json` in `~/workspace/git-brx-sandbox/colleague` to `"retries": 10`, committed as `Alex Developer`, and pushed to `origin/main` (commit `3b8d104`).
>
> **Your Turn:**
> In `~/workspace/git-brx-sandbox/me`, run:
> ```bash
> git brx sync
> ```
> Expected behavior:
> 1. `git-brx` halts with Exit code 5 (`ExitConflict`).
> 2. You will see the hint: `Use 'git-brx resolve' to resolve conflicts, or 'git-brx reset' to abort`.
> 3. You can either fix the conflict markers in `config.json` and run `git brx resolve`, or abort cleanly with `git brx reset`.

---

### Example Flow C: Pair-programming on a shared topic branch (`git brx update`)

#### Prompt to Antigravity:
> *"Alex, I just published `feature/4` from `git-brx-sandbox/me`. Please pull it down in your workspace, add a second step, and push it back to origin."*

#### Alex's response in Chat:
> `[Alex Developer]:`
> I ran `git brx select feature/4` in `~/workspace/git-brx-sandbox/colleague`, added `Step 2: Validation` to the pipeline file, and pushed commit `8c91d2e` to `origin/feature/4`.
>
> **Your Turn:**
> In `~/workspace/git-brx-sandbox/me`, run:
> ```bash
> git brx update
> ```
> `git-brx` will autostash any uncommitted work and rebase cleanly over my latest commit!

---

## 5. Resetting the sandbox environment

If you want to wipe local changes and start completely fresh from GitHub `origin/main`:

```bash
./scripts/setup-sandbox.sh --reset
```

This cleans both `git-brx-sandbox/me` and `git-brx-sandbox/colleague` and re-clones a pristine copy from `origin/main`.
