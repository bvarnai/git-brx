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

### Option B: Build from source (Go >= 1.22)
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

`git-brx` uses a hierarchical discovery cascade to locate project configurations. You can run with **zero configuration** on standard GitHub or Bitbucket repositories, or customize rules using YAML files.

### Priority Order:
1. **Environment variable override:** `GIT_BRX_CONFIG_PATH` pointing to a specific file or folder.
2. **Repository configuration:** `.git-brx.yaml` or `.git-brx.yml` in the repository root directory.
3. **Repository folder configuration:** `.git-brx/config.yaml`.
4. **User-Level configuration:** `~/.config/git-brx/config.yaml` (global settings for your machine).
5. **Zero-Config auto-discovery (default):** If no config file is found, `git-brx` inspects `git remote get-url origin` and automatically discovers the platform, organization/owner, and repository name.

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
  instructions: true       # Injects standard merge instructions into PR description
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

### Bitbucket server & Jira authentication
For Atlassian enterprise servers, define:
```bash
export GIT_BRX_TOKEN="your_personal_access_token"
```
`git-brx` uses HTTP Bearer tokens or Basic Authentication negotiated safely in-memory.
