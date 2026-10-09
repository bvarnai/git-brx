# Multi-Platform Setup

`git-brx` is architected with a decoupled provider model, allowing it to seamlessly integrate with GitHub, Atlassian Bitbucket Server & Jira, or hybrid toolchains.

---

## 1. Zero-Config Mode (GitHub / Bitbucket Auto-Detection)

If your repository uses GitHub or Bitbucket Server, you typically **do not need any `.git-brx.yaml` file at all**.

When `git-brx` initializes, it inspects your remote origin URL:
- `git@github.com:my-org/my-project.git` ➔ Configures `platform: github`, `owner: my-org`, `repo: my-project`.
- `https://bitbucket.corp.net/scm/PROJ/repo.git` ➔ Configures `platform: bitbucket`, `project: PROJ`, `repo: repo`.

All branch naming rules and issue validations immediately work with standard defaults.

---

## 2. GitHub Configuration

To customize GitHub integration, place `.git-brx.yaml` in your repository root:

```yaml
platform: github

# Reviewer Routing Configuration
review:
  instructions: true                # Injects Squash & Merge guidance into PR
  template: .github/pull_request_template.md
  mapping:
    backend: [alice, bob]
    frontend: [carol, dave]
    security: [charlie]
    default: [team-lead]            # Fallback reviewer
```

### Authentication:
`git-brx` will read your personal access token (PAT) from:
1. `GITHUB_TOKEN` environment variable.
2. `GH_TOKEN` environment variable.
3. System Git credential helper (`git credential fill`).

---

## 3. Atlassian Bitbucket Server & Jira Configuration

For on-premises Atlassian enterprise deployments (Bitbucket Server / Data Center and Jira Server):

```yaml
platform: bitbucket

tracker:
  provider: jira
  project: VSB
  uri: https://jira.internal.example.com

scm:
  provider: bitbucket
  project: VSB
  repo: core-tools
  uri: https://bitbucket.internal.example.com

branch:
  template: "^(?P<type>issue|feature|epic)/(?P<key>VSB-\\d+)$"
  mapping:
    Bug: issue
    Technical Item: issue
    Backlog: feature
    Epic: epic

review:
  instructions: true
  template: etc/review-template.md
  mapping:
    UI: [jsmith, mjones]
    Database: [dba-team]
    default: [lead-dev]
```

### Authentication:
Set your Personal Access Token (PAT) via:
```bash
export GIT_BRX_TOKEN="your_atlassian_pat"
```

---

## 4. Hybrid Setups (Jira + GitHub)

Many modern organizations track user stories and bugs in Atlassian Jira, while hosting source code and doing code reviews on GitHub. `git-brx` natively supports this split configuration:

```yaml
# Issue tracking handled by Jira
tracker:
  provider: jira
  project: PROJ
  uri: https://jira.internal.example.com

# Code review handled by GitHub
scm:
  provider: github
  owner: my-org
  repo: backend-service

review:
  instructions: true
  template: .github/pull_request_template.md
  mapping:
    core: [alice]
    api: [bob]
    default: [charlie]
```

When you run:
- `git brx create issue/PROJ-101`: Validates against **Jira**.
- `git brx review`: Creates the pull request on **GitHub**, automatically linking back to Jira ticket `PROJ-101`.
