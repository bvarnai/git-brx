# Atlassian (Bitbucket Server & Jira) integration guide

> [!WARNING]
> **Experimental / Untested on Live Instances**
> While client code and comprehensive unit test suites exist for the Bitbucket Server and Jira REST API integrations, this backend has **not yet been verified against live production instances**. If your organization uses on-premises Bitbucket Server or Jira Data Center, please report any discrepancies on GitHub.

---

## 1. Overview

`git-brx` includes built-in client libraries for on-premises Atlassian enterprise stacks:
- **Jira Server / Data Center:** Used for issue status, issue types, components, and assignee inspection during `git brx create`.
- **Bitbucket Server / Data Center:** Used for opening Pull Requests (`git brx review`) and validating remote merge status (`git brx delete`).

---

## 2. Configuration schema (`.git-brx.yaml`)

To configure an Atlassian enterprise stack, define `platform: bitbucket` in `.git-brx.yaml`:

```yaml
platform: bitbucket

tracker:
  provider: jira
  project: CORE
  uri: https://jira.internal.example.com

scm:
  provider: bitbucket
  project: CORE
  repo: core-service
  uri: https://bitbucket.internal.example.com

branch:
  template: "^(?P<type>issue|feature|epic)/(?P<key>CORE-\\d+)$"
  mapping:
    Bug: issue
    Technical Item: issue
    Backlog: feature
    Epic: epic

review:
  merge:
    instructions: true
    strategy: squash
  template: etc/review-template.md
  mapping:
    labels:
      UI: [jsmith, mjones]
      Database: [dba-team]
    default: [lead-dev]
```

---

## 3. Authentication & Personal Access Token (PAT)

Authentication with Atlassian servers uses a Personal Access Token (HTTP Bearer token) exported in your shell environment:

```bash
export GIT_BRX_TOKEN="your_atlassian_personal_access_token"
```

If `GIT_BRX_TOKEN` is unset, `git-brx` will attempt to consult your system Git credential helper (`git credential fill`).

---

## 4. Hybrid setups: Jira + GitHub

Many engineering organizations use **Jira** for issue tracking and sprint planning, but use **GitHub** for repository hosting and code review. `git-brx` supports this decoupled combination seamlessly:

```yaml
# Issue tracking handled by Jira Server / DC
tracker:
  provider: jira
  project: CORE
  uri: https://jira.internal.example.com

# Code review handled by GitHub
scm:
  provider: github
  owner: acme-corp
  repo: core-service

review:
  merge:
    instructions: true
  template: .github/pull_request_template.md
  mapping:
    labels:
      core: [alice]
      api: [bob]
    default: [team-lead]
```

### How hybrid mode works:
1. `git brx create issue/CORE-101`: Validates the issue key, type, and status against **Jira**.
2. `git brx review`: Opens the pull request on **GitHub**, automatically injecting a markdown link back to Jira issue `CORE-101`.
3. `git brx delete`: Queries **GitHub** to ensure the PR was merged before pruning the branch.

### Hybrid authentication:
When operating in hybrid mode, provide both tokens:
```bash
export GIT_BRX_TOKEN="your_jira_token"
export GITHUB_TOKEN="your_github_token"
```
