# Spec: git-brx config

## 1. Command Signature
- **Usage:** `git-brx config [flags]`
- **Arguments:** None. (Positional arguments are rejected).
- **Flags:**
  - `--sources`: Outputs solely the resolved configuration sources hierarchy (file paths and remote origin) without dumping the configuration payload.
  - Standard global flags: `--help` (`-h`), `--version`, `--verbose` (`-v`), `--quiet` (`-q`), `--no-color`.

## 2. Prerequisites & Environment
- **Required host binaries:** `git` (>= 2.20.0, optional if user/shared global configuration files exist).
- **Environment variables read:** `GIT_BRX_CONFIG`, `HOME`, `NO_COLOR`.
- **Working directory requirements (GIT_PREFIX behavior):** Can be invoked from any repository subdirectory, or from outside a repository if global configuration is defined.
- **Git state assertions:** Read-only inspection command; safe to execute in any repository state (clean, dirty, rebase, merge, detached HEAD).

## 3. Core Execution Flow
1. **Working Directory & Worktree Context:**
   - Detect repository root if inside a Git worktree.
   - If inside a worktree, inspect the `origin` remote URL via `git remote get-url origin`.
2. **Cascade Evaluation:**
   - Resolve configuration across the 4-tier cascade:
     - **Tier 1:** User Global (`~/.config/git-brx/config.yaml`, `~/.git-brx.yaml`).
     - **Tier 2:** Shared Global (`GIT_BRX_CONFIG` environment variable).
     - **Tier 3:** Repo Local (`.git-brx.yaml`, `.git-brx/config.yaml` in repo root).
     - **Tier 4:** Remote Origin Auto-Discovery (inferred platform, owner, repo, branch conventions).
3. **Provenance Annotation:**
   - Prepend the Source hierarchy header as YAML comments:
     ```yaml
     # Source hierarchy:
     #   Tier 1 (User global [~/.config/git-brx/config.yaml, ~/.git-brx.yaml]): <path or "none">
     #   Tier 2 (Shared global [$GIT_BRX_CONFIG]):                             <path or "unset">
     #   Tier 3 (Repo local [.git-brx.yaml, .git-brx/config.yaml]):             <path or "none">
     #   Tier 4 (Remote origin [git remote origin]):                           <originURL or "none">
     ```
   - If `--sources` is specified, emit only the Source hierarchy header and exit cleanly with Exit Code `0`.
   - Otherwise, serialize the effective configuration to YAML and annotate auto-discovered or defaulted fields inline with YAML comments (e.g., `# auto-discovered from remote origin`, `# inferred from platform preset`, `# default convention`).
4. **Standard Output Protocol:**
   - Output directly to `stdout`. The resulting output is 100% valid YAML and safe to redirect directly into a `.git-brx.yaml` file (`git brx config > .git-brx.yaml`).

## 4. Error Handling & Exit Codes
| Exit Code | Scenario | Emitted Stderr Pattern |
| :--- | :--- | :--- |
| `0` | Configuration evaluated and printed successfully. | None. |
| `2` | Unexpected positional arguments passed (`git-brx config extra`). | `[git-brx] Error: Unexpected argument: <arg>` |
| `8` | No configuration files found and no remote origin detected. | `[git-brx] Error: No configuration file found and unable to detect platform from remote origin` |
| `8` | `GIT_BRX_CONFIG` path specified but file does not exist or is unreadable. | `[git-brx] Error: configuration file specified by GIT_BRX_CONFIG not found: <path>` |
