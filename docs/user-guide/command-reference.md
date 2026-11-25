# Complete command reference

Reference for all `git-brx` subcommands, flags, arguments, and examples.

---

## Global flags

Accepted across all commands:

| Flag | Shorthand | Description |
| :--- | :--- | :--- |
| `--verbose` | `-v` | Print debug diagnostics to `stderr` (Git commands, HTTP requests, config resolution). |
| `--quiet` | `-q` | Suppress informational logs and hints; print only errors and primary `stdout` output. |
| `--no-color` | | Disable ANSI color output across all streams. |
| `--help` | `-h` | Show command usage and flag reference. |
| `--version` | | (Root only) Print version, commit hash, and target OS/architecture. |

---

## 1. `git-brx create`
**Usage:** `git-brx create [flags] <branch>`

Creates and checks out a new local branch after validating the branch name and issue status against the configured issue tracker, and records the fork base in `branch.<branch>.brx-base`. If `<branch>` already exists locally or on `origin`, automatically switches to it.

### Flags
- `-b`, `--base <branch>`: Explicit base branch to fork from and record as parent base.
- `-o`, `--offline`: Skip remote ref checks and issue tracker API validation.
- `-y`, `--yes`: Accept confirmation prompts non-interactively.

### Examples
```bash
git brx create issue/VSB-101
git brx create --base epic/10 issue/VSB-103 -y
git brx create --offline feature/NEW-FEATURE
git brx create -y issue/VSB-102
```

---

## 2. `git-brx select`
**Usage:** `git-brx select [flags] [<branch>]`

Switches the working tree to an existing branch (`main` by default). Fetches from `origin`, automatically fast-forwards clean branches when strictly behind `origin/<branch>`, warns when uncommitted changes are carried over, and suggests close branch names on typos.

### Flags
- `-o`, `--offline`: Skip `git fetch` and switch using local refs only.
- `--no-ff`: Skip automatic fast-forward when the selected branch is strictly behind `origin`.

### Examples
```bash
git brx select                    # Switch to default base branch (main/master)
git brx select issue/VSB-101      # Switch to issue/VSB-101
git brx select --no-ff main       # Switch without fast-forwarding
git brx select -o feature/offline # Switch without fetching from origin
```

---

## 3. `git-brx sync`
**Usage:** `git-brx sync [flags] [<branch>]`

Syncs the current branch with its recorded base branch (`branch.<name>.brx-base`), an explicit `<branch>`, or `main` by default (including remote-only `origin/<base>` or bases checked out in another linked worktree). Rebases `issue/*` branches and merges into `feature/*` and `epic/*` branches.

### Flags
- `-m`, `--merge`: Force a merge instead of the branch's default strategy.
- `-r`, `--rebase`: Force a rebase instead of the branch's default strategy.
- `-a`, `--autostash`: Stash dirty working-tree changes before rebasing and reapply afterward (rebase only).
- `-i`, `--interactive`: Start an interactive rebase (rebase only).
- `--no-auto-update-base`: Do not fast-forward local base branch from `origin` automatically.

### Examples
```bash
git brx sync
git brx sync -m develop
git brx sync -a
```

---

## 4. `git-brx update`
**Usage:** `git-brx update [flags]`

Queries `origin` directly, fetches, and rebases the current branch onto `origin/<current-branch>` (preserving 3-way merge commits with `--rebase-merges` on `feature/*` and `epic/*` branches).

### Flags
- `-a`, `--autostash`: Stash local changes before rebasing and reapply afterward.

### Examples
```bash
git brx update
git brx update --autostash
```

---

## 5. `git-brx publish`
**Usage:** `git-brx publish [flags]`

Pushes the current branch to `origin` and sets upstream tracking (`origin/<branch>`). Uses `--force-with-lease` by default on `issue/*`, `feature/*`, and custom branches, and enforces fast-forward-only pushes on `epic/*` and `release/*` integration branches.

### Flags
- `--no-force`: Push without `--force-with-lease` (fast-forward only).

### Examples
```bash
git brx publish
git brx publish --no-force
```

---

## 6. `git-brx review`
**Usage:** `git-brx review [flags] [<target_branch>]`

Verifies that the active branch is pushed and matches local `HEAD`, resolves `<target_branch>` (`<arg>` $\to$ recorded `branch.<name>.brx-base` $\to$ repository default) and checks its remote existence, opens a pull request via the configured SCM provider, routes reviewers from `.git-brx.yaml`, and appends merge instructions. On `epic/*` branches, it also aggregates `Closes #<N>` references from merged child pull requests targeting the epic. When `review.template` is unset, it auto-discovers a repository PR template (such as `pull_request_template.md`).

### Flags
- `-r`, `--reviewer <user>`: Request specific reviewer(s) instead of using configured routing rules (repeatable or comma-separated).
- `-t`, `--title <title>`: Override the generated pull request title.
- `-s`, `--strategy <strategy>`: Override the merge strategy in PR instructions (`squash`, `merge`, or `rebase`).
- `--draft`: Create the pull request in draft state (skips automated `review.mapping` assignment unless `--reviewer` is explicitly provided).
- `--merge-instructions`: Explicitly enable merge instructions in the PR description.
- `--no-merge-instructions`: Explicitly disable merge instructions in the PR description.

### Examples
```bash
git brx review
git brx review develop
git brx review -r alice,bob -r bob
git brx review --draft
git brx review -s merge
```

---

## 7. `git-brx delete`
**Usage:** `git-brx delete [flags] [<branch>]`

Deletes a local branch (`issue/*`, `feature/*`, or `epic/*`) after verifying that the remote branch was removed on `origin` (or deleting it safely via `--delete-remote` when its PR is `MERGED`), then runs `git remote prune origin`. Defaults to the active branch; returns to the recorded parent base (`branch.<branch>.brx-base`) or `main` (auto-fast-forwarding if clean and behind `origin`, or detaching `HEAD` when the return branch is checked out in another linked worktree).

### Flags
- `-f`, `--force`: Skip checking whether the branch still exists on `origin`.
- `--delete-remote`: Delete the remote branch on `origin` when the SCM API confirms its pull request is `MERGED`, then delete locally.

### Examples
```bash
git brx delete
git brx delete --delete-remote
git brx delete issue/101
git brx delete --force
```

---

## 8. `git-brx resolve`
**Usage:** `git-brx resolve [flags]`

Scans unmerged files for conflict markers: if markers are already cleared (for example, edited manually), stages **only** those resolved files and continues without launching a merge tool; otherwise launches the configured merge tool (`git config merge.tool`), verifies conflict markers are resolved, stages those paths, and continues the active rebase or merge.

### Flags
- `-t`, `--tool <tool>`: Override the configured Git merge tool.
- `--no-tool`: Never launch an external merge tool; fail listing conflicted files if markers remain.

### Examples
```bash
git brx resolve
git brx resolve --no-tool
git brx resolve -t vimdiff
```

---

## 9. `git-brx reset`
**Usage:** `git-brx reset [flags]`

When a merge or rebase conflict is active, aborts the in-flight operation while preserving local commits. When idle (no active conflict), resets the branch to match `origin/<branch>`.

### Flags
- `-c`, `--clean`: Remove untracked files and directories (`git clean -fd`, idle mode only).

### Examples
```bash
git brx reset
git brx reset --clean
```

---

<a id="git-brx-history"></a>
## 10. `git-brx history`
**Usage:** `git-brx history [flags] [<path>]`

Prints a formatted commit graph with commit hashes, timestamps, authors, and refs.

### Flags
- `-l`, `--limit <N>`: Maximum number of commits to show.
- `-b`, `--branch <name>`: Show history for a specific branch.
- `--delta`: Show only commits on the current branch since diverging from its recorded parent base (`branch.<name>.brx-base`) or the repository default base (`<base>..HEAD`).
- `-s`, `--search <query>`: Filter commits by commit message (case-insensitive).
- `--stat`: Include file diffstats for each commit.
- `-p`, `--patch`: Include full diffs for each commit.

### Examples
```bash
git brx history -l 10
git brx history --delta
git brx history -s "fix memory leak"
git brx history --stat src/api
```

---

<a id="git-brx-config"></a>
## 11. `git-brx config`
**Usage:** `git-brx config [flags]`

Prints the effective configuration resolved across all four cascade tiers (user global, shared global, repo local, and remote origin) in YAML format with provenance comments.

### Flags
- `--sources`: Print only the list of active configuration source tiers.

### Examples
```bash
git brx config
git brx config --sources
```

---

<a id="git-brx-name"></a>
## 12. `git-brx name`
**Usage:** `git-brx name [flags]`

Prints the current branch name to `stdout` followed by a newline (`BRANCH=$(git brx name)`).

---

<a id="git-brx-help"></a>
## 13. `git-brx help`
**Usage:** `git-brx help [<subcommand>]`

Prints the command list or detailed usage for a subcommand. Useful on Windows or environments where `git brx --help` is intercepted by Git's `man` / HTML doc viewer.

### Examples
```bash
git brx help
git brx help create
git brx help sync
```

---

<a id="git-brx-completion"></a>
## 14. `git-brx completion`
**Usage:** `git-brx completion <bash|zsh|fish|powershell>`

Generates shell autocompletion scripts for subcommands, flags, and dynamic branch names.

### Examples
```bash
source <(git-brx completion bash)
source <(git-brx completion zsh)
git-brx completion fish | source
git-brx completion powershell | Out-String | Invoke-Expression
```
