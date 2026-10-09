# User guide & wiki

Welcome to the **`git-brx` user guide**. This documentation provides comprehensive tutorials, conceptual deep dives, command catalogs, and troubleshooting advice for modern software development teams using `git-brx`.

---

## Table of contents

| Chapter | Title | Focus Area |
| :---: | :--- | :--- |
| **01** | [Core philosophy & mental model](philosophy.md) | Issue-First Development, hiding Git complexity, psychological safety, and PR review hygiene. |
| **02** | [Getting started](getting-started.md) | Installation, shell integration (`git brx`), and configuration cascade. |
| **03** | [The branch lifecycle](branch-lifecycle.md) | End-to-end walkthrough: creating, working, publishing, reviewing, and safely deleting branches. |
| **04** | [Synchronization, rebasing & conflict resolution](synchronization-and-rebasing.md) | How `sync` and `update` work, interactive conflict resolution with `resolve`, and resetting safely with `reset`. |
| **05** | [Multi-platform setup](multi-platform.md) | Configuring GitHub, Bitbucket Server, Jira, and hybrid enterprise setups. |
| **06** | [Complete command reference](command-reference.md) | Exhaustive reference of all 10 subcommands, accepted flags, exit codes, and output formats. |
| **07** | [Troubleshooting & FAQ](troubleshooting-faq.md) | Practical answers to common questions, credential troubleshooting, and recovery tips. |

---

## Reading paths

### For new engineers & onboarding developers
If you are new to the team or want to get productive quickly without worrying about complex Git commands:
1. Start with the [Quick start in the README](../../README.md#quick-start).
2. Read [Core philosophy & mental model](philosophy.md) to understand why we structure work around issues.
3. Follow the step-by-step [Branch lifecycle](branch-lifecycle.md).

### For seasoned Git users & tech leads
If you already know Git inside and out and want to understand how `git-brx` standardizes team hygiene:
1. Read [Core philosophy & mental model](philosophy.md) for rationale on branching models and Squash & Merge defaults.
2. Review [Synchronization & rebasing](synchronization-and-rebasing.md) to see how `git-brx` automates `--force-with-lease` and safe in-memory delta calculations.
3. Check [Multi-platform setup](multi-platform.md) to configure reviewer routing and pull request templates for your team.

---

## Technical specifications
For internal implementation details, error code contracts, and architecture diagrams, consult the [Technical specifications](../spec/global.md).
