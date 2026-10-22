# GitHub integration guide

GitHub is the primary and fully verified platform supported by `git-brx`. It provides seamless zero-config issue validation, automated pull request creation, reviewer routing, and post-merge branch deletion.

---

## 1. Zero-config & auto-detection

If your repository origin points to GitHub (e.g. `git@github.com:owner/repo.git` or `https://github.com/owner/repo.git`), `git-brx` requires **zero configuration**.

When initialized in a GitHub repository:
- `platform` is automatically set to `github`.
- `owner` and `repo` are parsed directly from `origin`.
- Branch naming templates default to standard prefixes (`issue/<id>`, `feature/<id>`, `epic/<id>`).
- Issue validation and code review automatically communicate with GitHub's REST API.

### Partial configuration
If you want to customize reviewer mappings or branch labels without overriding the auto-discovered platform, simply define only the fields you wish to customize in `.git-brx.yaml`:

```yaml
# .git-brx.yaml
review:
  default: [team-lead]
```
`git-brx` auto-detects `platform: github` from `origin` while applying your custom review defaults.

---

## 2. Authentication & Personal Access Token (PAT) setup

`git-brx` interacts with GitHub's REST API during:
- `git brx create <branch>`: Inspects issue titles, status, and labels.
- `git brx review`: Opens Pull Requests and assigns reviewers.
- `git brx delete`: Queries PR merge status before pruning branches.

### Token resolution order
`git-brx` checks the following sources in order:
1. `GITHUB_TOKEN` environment variable.
2. `GH_TOKEN` environment variable.
3. System Git credential helper (`git credential fill`).

### Creating a Fine-Grained Personal Access Token

#### Step 1: Navigate to Token Settings
1. In GitHub, click your profile icon in the top right and select **Settings**.
2. In the left navigation sidebar under **Access**, click **Credentials** (or **Developer settings** $\to$ **Personal access tokens**).
3. Select **Fine-grained personal access tokens**.
4. Click **Generate new token**.

![GitHub Credentials Navigation](../../../assets/github-token-credentials-nav.png)

#### Step 2: Configure Repository Access
In the **Repository access** section:
- Select **Only select repositories** and choose your repository (e.g. `owner/repo`), or select **All repositories** if you use `git-brx` across all repositories on your workstation.

#### Step 3: Required Permissions (Least Privilege)
`git-brx` operates under strict least-privilege principles. You only need the following permissions:

| Permission Category | Scope | Access Level | Why `git-brx` needs it |
| :--- | :--- | :--- | :--- |
| **Repository permissions** | **Issues** | **Read-only** | Reads issue title, status, and labels to validate branch prefixes (`issue/`, `feature/`, `epic/`). |
| **Repository permissions** | **Metadata** *(Required)* | **Read-only** | Enforced by GitHub for repository discovery and collaborator validation. |
| **Repository permissions** | **Pull requests** | **Read and write** | Opens pull requests during `git brx review` and verifies merge state during `git brx delete`. |
| **Repository permissions** | **Contents** *(Private repos)* | **Read-only** | Required on **private** repositories during `git brx review` so GitHub can compare commits between `head` and `base` (preventing `422: not all refs are readable`). |
| **User / Account permissions** | *(None)* | *None* | `git-brx` does **not** require any account-level permissions. |

![GitHub Fine-grained token repository permissions](../../../assets/github-token-permissions.png)

Verify your token overview reflects only these permissions:

![GitHub Fine-grained token overview](../../../assets/github-token-overview.png)

#### Step 4: Export the Token
Generate and copy your token (starting with `github_pat_...`), then export it in your shell environment (`~/.bashrc`, `~/.zshrc`, or Git Bash):
```bash
export GITHUB_TOKEN="github_pat_xxxxxxxxxxxxxxxxxxxx"
# or
export GH_TOKEN="github_pat_xxxxxxxxxxxxxxxxxxxx"
```

---

## 3. Git transport authentication (SSH vs. HTTPS) vs. API tokens

A common question is whether using an SSH remote URL conflicts with GitHub API tokens:

| Layer | Transport / Client | Authentication Credential | Responsibilities in `git-brx` |
| :--- | :--- | :--- | :--- |
| **Git Wire Transport** | Git protocol via SSH (`git@github.com:...`) or HTTPS (`https://github.com/...`) | SSH Key (`~/.ssh/id_*` / ssh-agent) or Git Credential Manager | Transferring commits, pushing branches (`git brx publish`), fetching remotes (`git fetch`), and branch rebasing. |
| **Platform REST API** | HTTPS REST requests to `api.github.com` | Personal Access Token (`GITHUB_TOKEN` / `GH_TOKEN`) | Issue metadata validation (`git brx create`), Pull Request creation (`git brx review`), and PR status inspection (`git brx delete`). |

> [!NOTE]
> **Can you use SSH with `git-brx`?** Yes! Using SSH keys for Git push/fetch operations while using a Personal Access Token for GitHub API requests is the recommended, standard setup. Standard Git commands seamlessly handle SSH.

---

## 4. Recommended repository settings

In your GitHub repository under **Settings** $\to$ **General** $\to$ **Pull Requests**, configure the following two settings for the smoothest `git-brx` workflow:

### 1. Squash merge commit message
To ensure GitHub pre-populates the squash commit message using the clean PR title generated by `git-brx` (`feat(#<issue>): <summary> (#<pr>)`):

1. Under **Pull Requests** $\to$ **Allow squash merging**, change the dropdown to **Pull request title** (or **Pull request title and commit details**).

![GitHub Allow squash merging dropdown set to Pull request title](../../../assets/github-squash-merge-setting.png)

If left on GitHub's default ("Default message"), GitHub will populate the squash title from the *single commit message* when a PR has only 1 commit, rather than preserving the PR title.

### 2. Automatically delete head branches
Enable **Automatically delete head branches** so GitHub automatically deletes the remote topic branch (`issue/*`, `feature/*`, `epic/*`) on `origin` as soon as a pull request is merged:

1. Under **Pull Requests**, check **Automatically delete head branches**.

With this setting enabled, running `git brx delete` after a PR is merged immediately verifies that the remote branch is gone, switches your worktree back to `main`, deletes the local branch, and prunes `origin`—without needing `--force` or manual branch deletion on GitHub.

---

## 5. Branch prefixes & label mapping

When creating a branch via `git brx create <branch>`, `git-brx` inspects the target issue's GitHub labels to validate that your branch prefix matches the nature of the change.

### GitHub default labels (Out of the box)
By default, newly created GitHub repositories only include standard labels such as `bug` and `enhancement`. `git-brx` recognizes these out of the box with zero configuration:

* **`bug` (GitHub default):** Mapped to `issue/` — restricted to solo bugfixes and small patches (attempting to create `feature/` or `epic/` is blocked)
* **`enhancement` (GitHub default):** Dual-mapped to `[issue, feature]` — flexible routing! Enhancements can be branched either as solo tasks (`issue/<id>`) or collaborative stories (`feature/<id>`)
* **Unlabeled issues:** Fall back to `issue/<id>` by default

> [!NOTE]
> GitHub does **not** provide `feature` or `epic` labels by default. While `git-brx` has built-in preset mappings for `feature` and `epic` if those labels exist in your repository, you need to create them in your GitHub repository label settings if your team wishes to use them.

### Extending with custom labels & multi-branch routing
If your team creates custom labels in GitHub (e.g. `feature`, `epic`, `kind/feature`, `type:bug`) or wants custom routing rules, configure `branch.mapping` in `.git-brx.yaml`:

```yaml
branch:
  template: '^(issue|feature|epic)/[1-9][0-9]*$'
  mapping:
    # Direct raw GitHub labels (case-insensitive):
    "kind/bug": "issue"
    "enhancement": [issue, feature]   # Dual-mapping: allow either issue/ or feature/
    "feature": "feature"              # Custom label created in GitHub
    "epic": "epic"                    # Custom label created in GitHub
    "task": "issue, feature, epic"   # Comma-separated alternative syntax

    # Domain types:
    Bug: "issue"
    Story: "feature"
    Epic: "epic"
```

If an engineer attempts to create `feature/10` for an issue labeled `bug`, `git-brx` guards against mismatched categorization:
```text
[git-brx] Error: Issue type 'bug' is not allowed on 'feature' branch
[git-brx] Hint: Use 'issue' branch or check mapping rules
```

---

## 6. Full GitHub configuration reference (`.git-brx.yaml`)

```yaml
platform: github

# Reviewer routing & merge rules
review:
  merge:
    instructions: true              # Injects merge strategy guidance and branch cleanup into PR
    strategy: squash                 # Global fallback strategy (squash | merge | rebase)
    strategies:
      sources:                       # Match against PR source branch (HEAD)
        "epic/*": merge
        "release/*": merge
      targets:                       # Match against PR target branch (BASE)
        "epic/*": merge
        "release/*": merge
  template: .github/pull_request_template.md
  mapping:
    paths:
      "internal/ui/**": [carol, dave]
      "internal/git/**": [bob]
      "docs/**": [tech-writer]
    labels:
      security: [charlie]
      backend: [alice, bob]
    branches:
      "epic/*": [lead-architect]
    default: [team-lead]            # Fallback reviewer
```
