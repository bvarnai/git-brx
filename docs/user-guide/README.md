# User guide & wiki

This guide covers how to install, configure, and use `git-brx` for day-to-day branch management and code review.

---

## Table of contents

| Chapter | Title | Topics Covered |
| :---: | :--- | :--- |
| **01** | [Core philosophy & mental model](philosophy.md) | Issue-first workflow, branch roles, and separating issue specs from code review |
| **02** | [Getting started](getting-started.md) | Installation, shell integration (`git brx`), 4-tier config cascade, and AI agent setup |
| **03** | [The branch lifecycle](branch-lifecycle.md) | Branch prefixes, sync strategies, and full walkthrough (`create` to `delete`) |
| **04** | [Synchronization, rebasing & conflict resolution](synchronization-and-rebasing.md) | How `sync` and `update` work, resolving conflicts with `resolve`, and recovering with `reset` |
| **05** | [Supported platforms & integration matrix](multi-platform.md) | Platform status, zero-config detection, branch protection rules, and platform guides |
| **06** | [Complete command reference](command-reference.md) | Subcommands, flags, arguments, and examples |
| **07** | [Troubleshooting & FAQ](troubleshooting-faq.md) | Common errors, token troubleshooting, and recovery commands |
| **08** | [Colleague simulation & multi-developer testing](colleague-simulation.md) | Testing multi-developer rebase, conflict, and review flows with a simulated peer agent |

---

## Reading paths

### New to `git-brx`
1. Run through the [Quick start in the README](../../README.md#quick-start).
2. Read [Core philosophy & mental model](philosophy.md) for the issue-first workflow and branch types.
3. Follow [The branch lifecycle](branch-lifecycle.md) step by step.

### Configuring `git-brx` for a team
1. Read [Core philosophy & mental model](philosophy.md) for the rationale behind branch prefixes and Squash & Merge defaults.
2. Review [Synchronization, rebasing & conflict resolution](synchronization-and-rebasing.md) for `--force-with-lease` and conflict handling details.
3. Use [Supported platforms & integration matrix](multi-platform.md) and the [GitHub guide](platforms/github.md) to set up reviewer routing, PR templates, and repository settings.

---

## Technical specifications
For CLI contracts, exit codes, and plumbing specifications, see [`docs/spec/global.md`](../spec/global.md).
