# Getting started

This guide covers installing `git-brx`, enabling shell completion, and configuring project and authentication settings.

---

## 1. Installation

`git-brx` ships as a single static binary with no external runtime dependencies (only `git` >= 2.20).

### Option A: Download precompiled binaries
Download the archive for your OS and architecture from the [Releases](https://github.com/bvarnai/git-brx/releases) page:
- Linux (`git-brx_linux_amd64.tar.gz`)
- macOS (`git-brx_darwin_arm64.tar.gz` / `amd64`)
- Windows (`git-brx_windows_amd64.zip`)

Extract the binary into a directory on your `PATH`:
```bash
# Linux / macOS:
sudo mv git-brx /usr/local/bin/

# Windows (Git Bash):
mkdir -p ~/bin
mv git-brx.exe ~/bin/
# Make sure ~/bin is in your PATH in ~/.bashrc or ~/.bash_profile
```

### Option B: Windows via Scoop
With [Scoop](https://scoop.sh/):
```powershell
scoop bucket add bvarnai https://github.com/bvarnai/scoop-bucket
scoop install git-brx
```
Scoop adds the `git-brx` shim to your `PATH`, making both `git-brx` and `git brx` available in PowerShell, Command Prompt, and Git Bash.

### Option C: Build from source (Go >= 1.22)
```bash
git clone https://github.com/bvarnai/git-brx.git
cd git-brx
go build -o /usr/local/bin/git-brx ./cmd/git-brx
```

### Verify installation
```bash
git-brx --version
# git-brx version v1.2.0 (86d244b linux/amd64)
```

---

## 2. Shell integration (`git brx`)

Git resolves any executable named `git-<name>` on your `PATH` as a Git subcommand. Once `git-brx` is on your `PATH`, both invocations work identically without setting up Git aliases:

```bash
# Direct binary invocation:
git-brx create issue/VSB-101

# Git subcommand dispatch:
git brx create issue/VSB-101
```

### Shell completion
`git-brx` generates completion scripts for subcommands, flags, and dynamic branch names (`select`, `sync`, `review`, `delete`, and `history -b`).

#### Bash (Linux, WSL, and Git Bash on Windows)
```bash
# Add to ~/.bashrc or ~/.bash_profile:
source <(git-brx completion bash)
```

#### Zsh (macOS / Linux)
```bash
# Add to ~/.zshrc:
source <(git-brx completion zsh)
```

#### Fish
```bash
git-brx completion fish | source
```

#### PowerShell
```powershell
git-brx completion powershell | Out-String | Invoke-Expression
```

---

## 3. Configuration cascade

`git-brx` resolves configuration across four tiers, merging settings from user-level defaults down to repository-local overrides.

### Cascade hierarchy (lowest to highest precedence)
1. **Tier 1 — User global:**
   - Checked in `~/.config/git-brx/config.yaml`, `~/.config/git-brx/config.yml`, `~/.git-brx.yaml`, `~/.git-brx.yml`, or `~/.git-brx/config.yaml`.
   - Use for personal defaults across all local repositories.
2. **Tier 2 — Shared global (`GIT_BRX_CONFIG`):**
   - File path set via `export GIT_BRX_CONFIG=/path/to/shared-rules.yaml`.
   - Use for shared team or organization rules across multiple repositories.
3. **Tier 3 — Repository local:**
   - Checked in the repository root: `.git-brx.yaml`, `.git-brx.yml`, or `.git-brx/config.yaml`.
   - Use for repo-specific branch mappings, path reviewers, or merge strategies.
4. **Tier 4 — Remote origin auto-detection:**
   - Reads `git remote get-url origin` to fill in `platform`, `owner`/`project`, and `repo` when not explicitly configured.

### Merge rules
- **Maps** (`branch.mapping`, `review.mapping.paths`, `review.mapping.labels`, `review.mapping.branches`, `review.merge.strategies.*`) are deep-merged: higher tiers add new keys or override matching keys while keeping existing keys from lower tiers.
- **Scalars** (`platform`, `branch.template`, `review.merge.strategy`) and **lists** (`review.mapping.default`) replace values from lower tiers when set.

### Inspecting resolved configuration
Run `git brx config` to print the active configuration and the source tier for each field:
```bash
git brx config
```
```yaml
# Source hierarchy:
#   Tier 1 (User global [~/.config/git-brx/config.yaml, ~/.git-brx.yaml]): none
#   Tier 2 (Shared global [$GIT_BRX_CONFIG]):                             unset
#   Tier 3 (Repo local [.git-brx.yaml, .git-brx/config.yaml]):             none
#   Tier 4 (Remote origin [git remote origin]):                           git@github.com:my-org/my-service.git

platform: github # auto-discovered from remote origin
tracker:
  provider: github # inferred from platform preset
  owner: my-org # auto-discovered from remote origin
  repo: my-service # auto-discovered from remote origin
scm:
  provider: github # inferred from platform preset
  owner: my-org # auto-discovered from remote origin
  repo: my-service # auto-discovered from remote origin
branch:
  template: ^(issue|feature|epic)/[A-Za-z0-9_-]+$ # default convention
  mapping: # platform default (github)
    bug: issue
    enhancement: feature
```

To list only the configuration sources:
```bash
git brx config --sources
```

---

## 4. Configuration schema (`.git-brx.yaml`)

Example `.git-brx.yaml` showing all top-level sections:

```yaml
# Platform preset: 'github', 'bitbucket', or 'none'
platform: github

# Issue tracker settings (optional when using a platform preset)
tracker:
  provider: github         # 'github' or 'jira'
  owner: acme-corp          # GitHub owner / org
  repo: core-api            # GitHub repo name
  project: CORE             # Jira project key (e.g., 'VSB', 'CORE')
  uri: https://jira.internal.example.com

# SCM / code review settings
scm:
  provider: github         # 'github' or 'bitbucket'
  owner: acme-corp
  repo: core-api

# Branch naming & issue-type mapping rules
branch:
  template: "^(?P<type>issue|feature|epic)/(?P<key>[A-Za-z]+-\\d+)$"
  mapping:
    Bug: issue
    Task: issue
    Story: feature
    Epic: epic

# Pull request & reviewer routing rules
review:
  merge:
    instructions: true     # Appends merge strategy instructions to PR descriptions
  template: .github/pull_request_template.md
  mapping:
    labels:
      backend: [alice, bob]
      frontend: [carol, dave]
      security: [eve]
    default: [charlie]     # Fallback reviewer when no path/label rules match
```

---

## 5. Authentication & tokens

`git-brx` reads tokens from environment variables or your Git credential helper without passing secrets on command-line arguments.

### GitHub
Export `GITHUB_TOKEN` or `GH_TOKEN`:
```bash
export GITHUB_TOKEN="github_pat_xxxxxxxxxxxx"
# or
export GH_TOKEN="github_pat_xxxxxxxxxxxx"
```
If neither variable is set, `git-brx` queries `git credential fill`.

> [!TIP]
> See the [GitHub integration guide](platforms/github.md#2-authentication--personal-access-token-pat-setup) for exact fine-grained PAT permissions.

### Bitbucket Server & Jira
Set `GIT_BRX_TOKEN`:
```bash
export GIT_BRX_TOKEN="your_personal_access_token"
```

---

## 6. AI agent integration (Antigravity & coding assistants)

AI coding agents often default to raw Git commands (`git checkout -b`, `git push -f`). Use the templates in `templates/` to configure agents to use `git-brx` instead.

### Step 1: Add the `git-brx` skill to your repo
```bash
mkdir -p .agents/skills/git-brx
cp templates/skills/git-brx/SKILL.md .agents/skills/git-brx/SKILL.md
```

The skill documents:
- The branch workflow sequence (`create` $\to$ `sync`/`update` $\to$ `publish` $\to$ `review` $\to$ `delete`).
- Non-interactive flag usage (`git brx create <branch> -y`).
- Exit codes and conflict recovery (`resolve` and `reset`).

### Step 2: Add the workflow rule to `GEMINI.md` or `AGENTS.md`
Append `templates/rules/git-brx.md` to your project's `GEMINI.md` or `AGENTS.md`:

```markdown
# Git Workflow Policy
- Always use `git-brx` (or `git brx`) CLI commands for Git branch operations:
  - Creating topic branches: `git brx create <branch> -y`
  - Upstream synchronization: `git brx sync`
  - Pulling collaborator commits: `git brx update`
  - Pushing branches: `git brx publish`
  - Submitting PRs: `git brx review`
  - Cleaning merged branches: `git brx delete <branch>`
- Do not run manual `git checkout -b`, `git push -f`, or raw API scripts for PR creation.
```

### Optional: Install the skill globally
To enable the `git-brx` skill across all local workspaces in Antigravity:

```bash
mkdir -p ~/.gemini/config/skills/git-brx
cp templates/skills/git-brx/SKILL.md ~/.gemini/config/skills/git-brx/SKILL.md
```
