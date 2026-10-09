# git-brx User Guide & Wiki

Welcome to the **`git-brx` User Guide**. This documentation provides comprehensive tutorials, conceptual deep dives, command catalogs, and troubleshooting advice for modern software development teams using `git-brx`.

---

## Table of Contents

| Chapter | Title | Focus Area |
| :---: | :--- | :--- |
| **01** | [Core Philosophy & Mental Model](philosophy.md) | Issue-First Development, hiding Git complexity, psychological safety, and PR review hygiene. |
| **02** | [Getting Started](getting-started.md) | Installation, shell integration (`git brx`), and configuration cascade. |
| **03** | [The Branch Lifecycle](branch-lifecycle.md) | End-to-end walkthrough: creating, working, publishing, reviewing, and safely deleting branches. |
| **04** | [Synchronization, Rebasing & Conflict Resolution](synchronization-and-rebasing.md) | How `sync` and `update` work, interactive conflict resolution with `resolve`, and resetting safely with `reset`. |
| **05** | [Multi-Platform Setup](multi-platform.md) | Configuring GitHub, Bitbucket Server, Jira, and hybrid enterprise setups. |
| **06** | [Complete Command Reference](command-reference.md) | Exhaustive reference of all 10 subcommands, accepted flags, exit codes, and output formats. |
| **07** | [Troubleshooting & FAQ](troubleshooting-faq.md) | Practical answers to common questions, credential troubleshooting, and recovery tips. |

---

## Reading Paths

### For New Engineers & Onboarding Developers
If you are new to the team or want to get productive quickly without worrying about complex Git commands:
1. Start with the [Quick Start in the README](../../README.md#quick-start).
2. Read [Core Philosophy & Mental Model](philosophy.md) to understand why we structure work around issues.
3. Follow the step-by-step [Branch Lifecycle](branch-lifecycle.md).

### For Seasoned Git Users & Tech Leads
If you already know Git inside and out and want to understand how `git-brx` standardizes team hygiene:
1. Read [Core Philosophy & Mental Model](philosophy.md) for rationale on branching models and Squash & Merge defaults.
2. Review [Synchronization & Rebasing](synchronization-and-rebasing.md) to see how `git-brx` automates `--force-with-lease` and safe in-memory delta calculations.
3. Check [Multi-Platform Setup](multi-platform.md) to configure reviewer routing and pull request templates for your team.

---

## Technical Specifications
For internal implementation details, error code contracts, and architecture diagrams, consult the [Technical Specifications](../spec/global.md).
