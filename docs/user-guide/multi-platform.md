# Supported platforms & integration matrix

`git-brx` decouples issue tracker clients (`TrackerProvider`) from source control review clients (`SCMProvider`), allowing single-platform or hybrid setups.

---

## Platform support status

| Platform / Backend | Issue tracker | Code review (PR / MR) | Status | Guide & documentation |
| :--- | :--- | :--- | :--- | :--- |
| ✅ **GitHub** | GitHub Issues | GitHub Pull Requests | **Done** | [GitHub integration guide](platforms/github.md) |
| ⌛ **Atlassian (Bitbucket + Jira)** | Jira Server / DC | Bitbucket Server / DC | *Implemented (untested)* | [Atlassian integration guide](platforms/atlassian.md) |
| 🚧 **Hybrid (Jira + GitHub)** | Jira Server / DC | GitHub Pull Requests | *Implemented (untested)* | [Hybrid setup guide](platforms/atlassian.md#4-hybrid-setups-jira--github) |
| 🚧 **GitLab** | GitLab Issues | GitLab Merge Requests | *Not yet implemented* | Planned |
| 🚧 **Bitbucket Cloud** | Jira Cloud / Bitbucket | Bitbucket Cloud PRs | *Not yet implemented* | Planned |

---

## 1. Zero-config & partial-config modes (Remote auto-detection)

### Zero-config mode
For GitHub repositories, **no `.git-brx.yaml` file is needed**.

`git-brx` parses `remote.origin.url` automatically:
- `git@github.com:my-org/my-project.git` $\to$ sets `platform: github`, `owner: my-org`, `repo: my-project`.

Default branch templates (`issue/*`, `feature/*`, `epic/*`), GitHub issue validation, and PR creation work immediately. See the [GitHub integration guide](platforms/github.md) for token setup.

### Partial-config mode
If you add a `.git-brx.yaml` file to customize specific fields (like `branch.mapping` or `review.mapping`) and omit `platform:`, `git-brx` still detects the platform, owner, and repo from `origin`:

```yaml
# .git-brx.yaml
branch:
  mapping:
    enhancement: "issue"
```

### Disabling platform integrations (`platform: none`)
To enforce local branch naming templates without querying a remote issue tracker or SCM API, set `platform: none`:

```yaml
# .git-brx.yaml
platform: none
```

---

<a id="5-branch-protection-rules--repository-permissions"></a>
## 2. Branch protection rules & repository permissions

### Developer Git permissions
`git-brx` requires standard repository permissions:
* **Read / clone access:** Fetching remotes, inspecting commits, and listing branches.
* **Write / push access:** Pushing topic branches (`issue/*`, `feature/*`, `epic/*`) to `origin` via `git brx publish`.

> [!NOTE]
> Contributors on read-only upstreams can use `git-brx` for local branch operations, but `git brx publish` requires push access to `origin`.

### Branch protection rules (Base branches vs. Topic branches)

Configure branch protection rules differently for base branches and topic branches:

| Branch category | Target branches | Recommended protection rules | Why |
| :--- | :--- | :--- | :--- |
| **Base Branches** | `main`, `master`, `release/*` | • Require pull requests before merging<br>• Block direct pushes<br>• Block branch deletion<br>• Block force-pushes | Base branches are updated only through merged pull requests (`git brx review`). |
| **Topic Branches** | `issue/*`, `feature/*`, `epic/*` | • Allow force-pushes (`--force-with-lease`)<br>• Allow branch deletion on merge | `git brx sync` and `git brx update` rebase topic branches, which rewrites commit hashes and requires `--force-with-lease` on `git brx publish`. |

> [!WARNING]
> **Do not apply no-force-push rulesets to `*` (all branches):**
> If a repository ruleset blocks force-pushes across `*`, `git brx publish` will fail whenever you push a rebased `issue/*` branch. Scope branch protection rules strictly to `main`, `master`, and `release/*`.

---

## 3. Platform guides

- 📘 [GitHub integration guide](platforms/github.md) — Zero-config setup, fine-grained PAT permissions, repository merge settings, and label mappings.
- 📙 [Atlassian (Bitbucket Server & Jira) integration guide](platforms/atlassian.md) — Bitbucket Server, Jira Data Center, and Jira + GitHub hybrid configs (implemented, untested on live servers).
