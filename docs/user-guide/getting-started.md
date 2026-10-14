# Getting started

This guide walks you through installing `git-brx`, integrating it with your shell environment, and configuring project settings.

---

## 1. Installation

`git-brx` is distributed as a self-contained, statically linked binary with zero runtime dependencies. It does not require Java, Groovy, Python, or external scripts.

### Option A: Download pre-built binaries
Download the binary for your platform from the [Releases](https://github.com/bvarnai/git-brx/releases) page:
- Linux (`git-brx_linux_amd64.tar.gz`)
- macOS (`git-brx_darwin_arm64.tar.gz` / `amd64`)
- Windows (`git-brx_windows_amd64.zip`)

Extract the archive and move the `git-brx` executable into a directory in your system `PATH`:
```bash
# On Linux / macOS:
sudo mv git-brx /usr/local/bin/

# On Windows (Git Bash):
mkdir -p ~/bin
mv git-brx.exe ~/bin/
# Ensure ~/bin is in your PATH in ~/.bashrc or ~/.bash_profile
```

### Option B: Windows via Scoop
If you use [Scoop](https://scoop.sh/) on Windows:
```powershell
scoop bucket add bvarnai-bucket https://github.com/bvarnai/scoop-bucket
scoop install git-brx
```
This automatically configures the `git-brx` shim in your Scoop shims directory, making `git-brx` and `git brx` immediately available in Command Prompt, PowerShell, and Git Bash.

### Option C: Build from source (Go >= 1.22)
```bash
git clone https://github.com/bvarnai/git-brx.git
cd git-brx
go build -o /usr/local/bin/git-brx ./cmd/git-brx
```

### Verifying the installation
Run the `--version` flag:
```bash
git-brx --version
# Output: git-brx version v1.0.0 (commit: abc1234, built at: 2026-10-09, linux/amd64)
```

---

## 2. Shell integration (`git brx`)

Because Git automatically resolves executables named `git-<subcommand>` from your system `PATH`, placing `git-brx` in your `PATH` immediately enables native Git plugin dispatch.

You can run `git-brx` in either format:
```bash
# Direct binary execution:
git-brx create issue/VSB-101

# Native Git command dispatch (recommended):
git brx create issue/VSB-101
```

Both invocations are completely identical. You do not need to install Git aliases.

### Shell autocompletion
`git-brx` features built-in shell autocompletion for subcommands, flags, and **dynamic Git branch names** (`select`, `sync`, `review`, and `history -b`).

To enable autocompletion in your shell:

#### Bash (including Ubuntu/WSL and Git Bash on Windows):
```bash
# In your ~/.bashrc or ~/.bash_profile:
source <(git-brx completion bash)
```

#### Zsh (macOS / Linux):
```bash
# In your ~/.zshrc:
source <(git-brx completion zsh)
```

#### Fish:
```bash
git-brx completion fish | source
```

#### PowerShell:
```powershell
git-brx completion powershell | Out-String | Invoke-Expression
```

---

## 3. Configuration cascade

`git-brx` uses a layered configuration cascade to resolve project settings across personal and enterprise workflows. Instead of a rigid "first-file-wins" model, configurations are cascadingly layered and merged from global defaults down to repository-specific overrides.

### Cascade Hierarchy (from base defaults to most specific override):
1. **Tier 1: Machine / User Global:**
   - Looked up in `~/.config/git-brx/config.yaml`, `~/.config/git-brx/config.yml`, `~/.git-brx.yaml`, `~/.git-brx.yml`, or `~/.git-brx/config.yaml`.
   - Ideal for personal defaults across all repositories on your machine (e.g. personal platform defaults or reviewer preferences).
2. **Tier 2: Shared Global (`GIT_BRX_CONFIG`):**
   - Configured via the `GIT_BRX_CONFIG` environment variable pointing to a shared configuration file (e.g., `export GIT_BRX_CONFIG=/path/to/shared-rules.yaml` or a synchronized organization repository).
   - Overlays shared branch templates, organization-wide review rules, and standard issue type mappings across projects.
3. **Tier 3: Repository Local:**
   - Looked up in the repository root: `.git-brx.yaml`, `.git-brx.yml`, or `.git-brx/config.yaml`.
   - Overlays repository-specific settings (e.g. specialized branch mappings, subfolder reviewer paths, or merge strategies).
4. **Tier 4: Remote Origin Auto-Discovery:**
   - Inspects `git remote get-url origin` and auto-discovers missing platform, owner/organization, and repository slug.

### Merging Behavior:
- **Map fields** (such as `branch.mapping`, `review.mapping.paths`, `review.mapping.labels`, `review.mapping.branches`, and `review.merge.strategies.*`) are deep merged: repository rules add new entries or override specific keys while preserving untouched keys from shared and user layers.
- **Scalar fields** (such as `platform`, `branch.template`, `review.merge.strategy`) overwrite prior layers whenever defined.
- **Reviewer defaults / lists** replace prior values when explicitly defined in a higher tier.

### Inspecting effective configuration:
You can verify the exact cascading configuration `git-brx` resolves for your current directory using the `config` command:
```bash
git brx config
```
This prints the source hierarchy and complete YAML configuration with inline comments highlighting auto-discovered fields:
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
  mapping: # default convention
    Bug: issue
    Epic: epic
    Issue: issue
    Story: feature
    Task: issue
```

To view only the participating configuration files:
```bash
git brx config --sources
```

---

## 4. Configuration schema (`.git-brx.yaml`)

Here is an annotated example of a comprehensive `.git-brx.yaml` file:

```yaml
# Platform preset: 'github' or 'bitbucket'
platform: github

# Issue tracker configuration (optional if matching platform preset)
tracker:
  provider: github         # 'github' or 'jira'
  owner: acme-corp          # GitHub owner / org
  repo: core-api            # GitHub repo name
  project: CORE             # Jira project key (e.g., 'VSB', 'CORE')
  uri: https://jira.internal.example.com

# SCM / Code review configuration
scm:
  provider: github         # 'github' or 'bitbucket'
  owner: acme-corp
  repo: core-api

# Branch creation rules
branch:
  template: "^(?P<type>issue|feature|epic)/(?P<key>[A-Za-z]+-\\d+)$"
  mapping:
    Bug: issue
    Task: issue
    Story: feature
    Epic: epic

# Code review & pull request automation
review:
  merge:
    instructions: true     # Injects standard merge instructions into PR description
  template: .github/pull_request_template.md
  mapping:
    backend: [alice, bob]
    frontend: [carol, dave]
    security: [eve]
    default: [charlie]     # Fallback reviewer if no components match
```

---

## 5. Authentication & tokens

`git-brx` securely reads tokens without leaking secrets to the process table or log files:

### GitHub authentication
Set one of the standard GitHub environment variables:
```bash
export GITHUB_TOKEN="ghp_xxxxxxxxxxxx"
# or
export GH_TOKEN="ghp_xxxxxxxxxxxx"
```
Or allow `git-brx` to consult your Git credential helper automatically (`git credential fill`).

> [!TIP]
> For step-by-step instructions on generating a fine-grained personal access token with the exact permissions required by `git-brx`, see the [GitHub token configuration & permissions guide](multi-platform.md#authentication--personal-access-token-pat-setup).

### Bitbucket server & Jira authentication
For Atlassian enterprise servers, define:
```bash
export GIT_BRX_TOKEN="your_personal_access_token"
```
`git-brx` uses HTTP Bearer tokens or Basic Authentication negotiated safely in-memory.
