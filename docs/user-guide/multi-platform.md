# Multi-platform setup

`git-brx` is architected with a decoupled provider model to support multiple issue trackers and code review systems.

## Platform support status

| Platform / Backend | Issue Tracker | Code Review (PR / MR) | Status | Notes |
| :--- | :--- | :--- | :--- | :--- |
| **GitHub** | GitHub Issues | GitHub Pull Requests | **Done** | Tested and verified in real-world usage. |
| **Atlassian (Bitbucket + Jira)** | Jira Server / DC | Bitbucket Server / DC | **Implemented (untested)** | Client code and unit tests present; not yet tested on live instances. |
| **Hybrid (Jira + GitHub)** | Jira Server / DC | GitHub Pull Requests | **Implemented (untested)** | Client code present; needs real-world testing. |
| **GitLab** | GitLab Issues | GitLab Merge Requests | **Not yet implemented** | Planned for future release. |
| **Bitbucket Cloud** | Jira Cloud / Bitbucket | Bitbucket Cloud PRs | **Not yet implemented** | Planned for future release. |

---

## 1. Zero-config mode (GitHub auto-detection)

If your repository uses GitHub, you **do not need any `.git-brx.yaml` file at all**.

When `git-brx` initializes, it inspects your remote origin URL:
- `git@github.com:my-org/my-project.git` ➔ Configures `platform: github`, `owner: my-org`, `repo: my-project`.

All branch naming rules and issue validations immediately work with standard defaults.

---

## 2. GitHub configuration (Done)

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

## 3. Atlassian Bitbucket Server & Jira configuration (Implemented, untested)

> [!NOTE]
> Client code and mock test suites exist for Bitbucket Server and Jira REST APIs, but this backend has **not yet been verified against live production instances**. If you test this configuration, please report any issues on GitHub.

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

## 4. Hybrid setups: Jira + GitHub (Implemented, untested)

> [!NOTE]
> Hybrid support (Jira for issue tracking and GitHub for code review) is implemented but untested in live workflows.

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
- `git brx create issue/TASK-101`: Validates against **Jira**.
- `git brx review`: Creates the pull request on **GitHub**, automatically linking back to Jira ticket `TASK-101`.

---

## 5. Other platforms (Not yet implemented)

Support for the following platforms is not yet implemented:
- **GitLab:** Native support for GitLab Issues and Merge Requests.
- **Bitbucket Cloud:** Atlassian Cloud 2.0 REST API support.
