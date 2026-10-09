# Core Philosophy & Mental Model

## 1. The Human Side of Version Control

Git is undeniably the industry standard for source control, but its command-line interface was designed as a plumbing toolkit rather than an ergonomic user interface. As teams grow, several systemic problems reliably emerge:

1. **Tooling Intimidation & Onboarding Friction:** New hires, junior engineers, and specialists from non-software backgrounds (data scientists, hardware designers, technical writers) are often intimidated by Git's cryptic error messages and steep learning curve.
2. **Fear of Breaking Things:** Developers hesitate to rebase, synchronize, or clean up branches because one wrong command (`git push --force`, `git reset --hard`, `git checkout .`) can silently discard hours of work or overwrite a teammate's commits.
3. **Inconsistent Team Habits:** Without standard tooling, every developer invents their own ad-hoc aliases, naming conventions, and merge practices. Some squash locally, some create tangled merge webs, and some leave dozens of abandoned branches lingering on the remote.

### The `git-brx` Solution: Guardrails & Psychological Safety
`git-brx` was engineered with a clear design goal: **hide low-level Git complexities behind sensible, guardrailed automation so that developers can ship changes quickly with confidence.**

Instead of memorizing dozens of obscure Git flags, developers interact with high-level workflow verbs (`create`, `sync`, `publish`, `review`, `delete`). Under the hood, `git-brx` enforces best practices:
- Pushes always use **lease protection** (`--force-with-lease`).
- Branch switches warn if uncommitted changes would be carried across branches.
- Conflict resolution isolates and stages **only** conflicted files.
- Branch deletion validates that the change was successfully merged in your SCM platform first.

---

## 2. Issue-Driven Development: The Single Source of Truth

At the heart of `git-brx` is a strict organizational philosophy: **every code change belongs to a tracked Issue.**

```
┌────────────────────────────────────────────────────────┐
│                   THE ISSUE (Jira / GitHub)             │
│  • What is the problem?                                │
│  • Why is this change necessary?                       │
│  • What are the acceptance criteria & edge cases?      │
│  • What is the test and verification plan?             │
└───────────────────────────┬────────────────────────────┘
                            │
              1:1 Link      ▼
┌────────────────────────────────────────────────────────┐
│             THE PULL REQUEST (Code Review)             │
│  • How is the solution architected?                    │
│  • Are unit and integration tests passing?             │
│  • Is the code readable, secure, and maintainable?     │
│  • Strictly for technical peer review!                 │
└────────────────────────────────────────────────────────┘
```

### Why Separate the "What" from the "How"?

In many development teams, Pull Requests suffer from **Scope Creep during Review**:
- An engineer opens a PR with minimal explanation.
- Reviewers spend days debating what the feature was actually supposed to do.
- Halfway through code review, stakeholders realize core business requirements were misunderstood.

`git-brx` enforces a clean division of responsibility:

| Artifact | Primary Audience | Core Purpose | Discussion Topics |
| :--- | :--- | :--- | :--- |
| **The Issue** | Product Managers, QA, Engineers, Stakeholders | **Source of Truth:** Defines *What* needs to be done and *Why*. Contains specifications, acceptance criteria, reproducible steps, and test plans. | Business requirements, edge cases, scope, user expectations. |
| **The Pull Request** | Engineering Peers | **Quality Assurance:** Inspects *How* the code implements the Issue. Verifies architecture, style, test coverage, and performance. | Implementation details, code structure, algorithm efficiency, test assertions. |

When a developer runs `git brx create issue/PROJ-101`, `git-brx` reaches out to the issue tracker, verifies that `PROJ-101` is a valid, assigned ticket, and sets up a local workspace directly linked to that contract.

---

## 3. The 4-Stage Workflow Cycle

```mermaid
flowchart TD
    subgraph S1 ["Stage 1: Intent & Creation"]
        Issue["Tracker Issue<br/>(PROJ-101)"] -->|"git brx create"| TopicBranch["Validated Branch<br/>(issue/PROJ-101)"]
    end

    subgraph S2 ["Stage 2: Iteration & Sync"]
        TopicBranch -->|"Write code"| LocalCommits["Local Commits"]
        LocalCommits -->|"git brx sync"| Rebased["Clean Rebase onto Base"]
        Rebased -->|"git brx publish"| RemoteBranch["Leased Push to Origin"]
    end

    subgraph S3 ["Stage 3: Peer Review"]
        RemoteBranch -->|"git brx review"| PullRequest["Pull Request<br/>• Checklist<br/>• Reviewers<br/>• Merge Instructions"]
    end

    subgraph S4 ["Stage 4: Merge & Cleanup"]
        PullRequest -->|"Approved & Merged"| SCM["Remote Merged"]
        SCM -->|"git brx delete"| Clean["Local Deleted & Pruned"]
    end
```

### Stage 1: Intent & Creation (`git brx create`)
You never branch from stale or arbitrary commits. `git-brx create` validates that you have an assigned task in Jira or GitHub, validates the naming convention, and branches directly off the latest base branch.

### Stage 2: Iteration & Sync (`git brx sync` / `git brx publish`)
As you work, other teammates will merge changes into `master` or `main`. Instead of allowing your branch to drift into a divergence nightmare, you run `git brx sync`. For standard issue branches, `git-brx` performs a clean, linear **rebase**, replaying your commits on top of the latest trunk.

When you're ready to share your work or backup your commits, `git brx publish` pushes with `--force-with-lease` and establishes upstream tracking automatically.

### Stage 3: Peer Review (`git brx review`)
When your implementation is complete and verified, running `git brx review` assembles a pull request. It automatically:
1. Links the parent Issue.
2. Injects your team's quality checklist (tests, docs, edge cases).
3. Evaluates modified components and routes the PR to the appropriate code reviewers.
4. Appends standardized, timeless merge instructions.

### Stage 4: Merge & Cleanup (`git brx delete`)
Once approved and merged, local branch clutter can become overwhelming. `git brx delete` queries your SCM provider to ensure the branch was truly merged, switches your working tree safely back to `master`, deletes the local branch, and prunes stale remote references.

---

## 4. Branch Taxonomy & Merge Discipline

`git-brx` categorizes branches into specific types, each with its own purpose, lifecycle, and merge strategy:

| Prefix | Lifespan | Typical Source | Merge Strategy | Rationale |
| :--- | :--- | :--- | :--- | :--- |
| `issue/` | Short (hours to days) | Bugs, tasks, small improvements | **Squash & Merge** | Collapses experimental commits into a single clean commit on `master` with the issue title, preserving clean bisectability. |
| `feature/` | Medium (days to weeks) | Larger functional capabilities | **Merge Commit (`--no-ff`)** | Preserves individual milestone commits while encapsulating the feature under a distinct merge bubble. |
| `epic/` | Long (weeks to months) | Multi-team initiatives | **Merge Commit (`--no-ff`)** | Groups large architectural changes together. |
| `master` / `main` | Permanent | Trunk / Single Source of Truth | N/A | Production-ready, always buildable and releasable. |

### The Power of Squash & Merge for Issues
Why does `git-brx` recommend **Squash & Merge** for standard issue branches?
- **Linear History:** The main trunk becomes an easily readable narrative: every commit represents a complete, verified unit of work linked to an issue key.
- **Flawless `git bisect`:** If a regression is introduced, `git bisect` lands directly on the single commit that introduced the issue, rather than an intermediate "WIP: fix typo" commit where the code may not even build.
- **Freedom to Commit Locally:** Engineers can make as many micro-commits locally as they want without worrying about polluting the shared repository history.
