# Atlassian (Bitbucket Server & Jira) integration guide

> [!WARNING]
> **Untested on live servers:**
> Client implementations and contract schema tests exist for Bitbucket Server and Jira Data Center, but this backend has **not yet been verified against live production servers**. If you run into issues on Bitbucket Server or Jira Data Center, please open an issue on GitHub.

---

## 1. Overview

`git-brx` includes REST clients for self-hosted Atlassian servers:
- **Jira Server / Data Center:** Queries issue status, issue type, components, and assignee during `git brx create`.
- **Bitbucket Server / Data Center:** Creates pull requests (`git brx review`) and checks PR merge state (`git brx delete`).

---

## 2. Configuration schema (`.git-brx.yaml`)

Set `platform: bitbucket` in `.git-brx.yaml` and provide your Jira and Bitbucket Server URLs:

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

Export your Atlassian Personal Access Token (HTTP Bearer token) in `GIT_BRX_TOKEN`:

```bash
export GIT_BRX_TOKEN="your_atlassian_personal_access_token"
```

If `GIT_BRX_TOKEN` is unset, `git-brx` falls back to your system Git credential helper (`git credential fill`).

---

## 4. Hybrid setups: Jira + GitHub

If your team tracks issues in **Jira** and hosts code and pull requests on **GitHub**, configure `tracker` and `scm` separately:

```yaml
# Issue tracking in Jira Server / DC
tracker:
  provider: jira
  project: CORE
  uri: https://jira.internal.example.com

# Pull requests on GitHub
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

### How hybrid mode works
1. `git brx create issue/CORE-101`: Validates the issue key, type, and status against **Jira**.
2. `git brx review`: Opens a pull request on **GitHub** and links it back to Jira issue `CORE-101`.
3. `git brx delete`: Checks PR merge state on **GitHub** when verifying remote branch cleanup.

### Hybrid authentication
Export tokens for both services:
```bash
export GIT_BRX_TOKEN="your_jira_token"
export GITHUB_TOKEN="your_github_token"
```
