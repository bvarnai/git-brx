# Supported platforms & integration matrix

`git-brx` is architected with a decoupled provider model to support multiple issue trackers and code review systems.

---

## Platform support status

| Platform / Backend | Issue tacker | Code review (PR / MR) | Status | Guide & documentation |
| :--- | :--- | :--- | :--- | :--- |
| ✅ **GitHub** | GitHub issues | GitHub pull requests | **Done** | [GitHub integration guide](platforms/github.md) |
| ⌛ **Atlassian (Bitbucket + Jira)** | Jira Server / DC | Bitbucket Server / DC | *Implemented (untested)* | [Atlassian integration guide](platforms/atlassian.md) |
| 🚧 **Hybrid (Jira + GitHub)** | Jira Server / DC | GitHub pull requests | *Implemented (untested)* | [Hybrid setup guide](platforms/atlassian.md#4-hybrid-setups-jira--github) |
| 🚧 **GitLab** | GitLab issues | GitLab merge requests | *Not yet implemented* | Planned for future release |
| 🚧 **Bitbucket Cloud** | Jira Cloud / Bitbucket | Bitbucket cloud PRs | *Not yet implemented* | Planned for future release |

---

## 1. Zero-config & partial-config modes (Remote auto-detection)

### Zero-config mode
If your repository uses GitHub, you **do not need any `.git-brx.yaml` file at all**.

When `git-brx` initializes, it inspects your remote origin URL:
- `git@github.com:my-org/my-project.git` ➔ Configures `platform: github`, `owner: my-org`, `repo: my-project`

All branch naming rules and issue validations immediately work with standard defaults. For full details on authentication and permissions, see the [GitHub integration guide](platforms/github.md).

### Partial-config mode
When you create a `.git-brx.yaml` to customize specific rules (such as custom `branch.mapping` or `review.mapping`) but omit `platform:`, `git-brx` automatically falls back to detecting your platform from the remote `origin` URL:

```yaml
# .git-brx.yaml
branch:
  mapping:
    enhancement: "issue"
```

In this case, `git-brx` detects `platform: github` from `origin`, auto-populates GitHub issue tracker and code review presets, and applies your custom branch mapping.

### Disabling platform integrations (`platform: none`)
If you want to maintain a `.git-brx.yaml` file (for example, to enforce branch naming templates or local workflows) but disable remote issue tracker checks and review integrations entirely, explicitly set:

```yaml
# .git-brx.yaml
platform: none
```

---

<a id="5-branch-protection-rules--repository-permissions"></a>
## 2. Branch protection rules & repository permissions

To maintain an orderly and reproducible codebase, remote Git hosting platforms (GitHub, Bitbucket Server, GitLab) enforce access control through **repository permissions** and **branch protection rules**.

### Developer Git permissions
`git-brx` assumes standard developer permissions:
* **Read / Clone access:** Required to fetch remotes, inspect commits, and discover branches
* **Write / Push access:** Required to publish topic branches (`issue/*`, `feature/*`, `epic/*`) to the remote repository (`git brx publish`)

> [!NOTE]
> External contributors working on public forks who lack direct push access can still run `git-brx` locally, but `git brx publish` will be rejected by the upstream remote if push access is restricted.

### Branch protection rules (Base branches vs. Topic branches)

`git-brx` workflows are designed around branch protection and isolation:

| Branch category | Target branches | Recommended protection rules | why |
| :--- | :--- | :--- | :--- |
| **Base Branches** | `main`, `master`, `release/*` | • Require pull requests before merging<br>• Prevent direct pushes<br>• Prevent branch deletion<br>• Prevent force pushes (history rewriting) | `git-brx` treats base branches as the production single source of truth. Developers never commit or push directly to `main`; all changes integrate via automated pull requests (`git brx review`) |
| **Topic Branches** | `issue/*`, `feature/*`, `epic/*` | • Allow force-pushes with lease (`--force-with-lease`)<br>• Allow branch deletion after merge<br>• Full control by developers | `git-brx` uses rebase workflows (`git brx update`, `git brx sync`) to keep history clean and linear. Rebasing changes commit hashes, requiring leased force-pushes during `git brx publish` |

> [!WARNING]
> **Avoid wildcard branch protection on topic branches:**
> If your repository administrator configures a blanket branch protection rule or ruleset matching all branches (`*`) that disables force-pushes or requires pull requests to update branches, `git brx publish` will fail when pushing rebased topic branches. Always scope branch protection rules strictly to `main`, `master`, and `release/*`.

---

## 3. Platform guides

- 📘 [GitHub integration guide](platforms/github.md) — Zero-config mode, Fine-grained PAT configuration, squash merge settings, and label mappings
- 📙 [Atlassian (Bitbucket Server & Jira) integration guide](platforms/atlassian.md) — Bitbucket Server, Jira Data Center, and hybrid configurations (Experimental / Untested)
