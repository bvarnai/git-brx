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

Creates and checks out a new local topic branch after validating the branch name and issue status against the configured issue tracker.

### Flags
- `-o`, `--offline`: Skip remote ref checks and issue tracker API validation.
- `-y`, `--yes`: Accept confirmation prompts non-interactively.

### Examples
```bash
git brx create issue/VSB-101
git brx create --offline feature/NEW-FEATURE
git brx create -y issue/VSB-102
```

---

## 2. `git-brx select`
**Usage:** `git-brx select [flags] [<branch>]`

Switches the working tree to an existing branch (`main` by default). Fetches from `origin`, warns when uncommitted changes are carried over, and suggests close branch names on typos.

### Flags
- `-o`, `--offline`: Skip `git fetch` and switch using local refs only.

### Examples
```bash
git brx select                    # Switch to default base branch (main/master)
git brx select issue/VSB-101      # Switch to issue/VSB-101
git brx select -o feature/offline # Switch without fetching from origin
```

---

## 3. `git-brx sync`
**Usage:** `git-brx sync [flags] [<branch>]`

Syncs the current branch with a base branch (`main` by default). Rebases `issue/*` branches and merges `main` into `feature/*` and `epic/*` branches.

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

Fetches and rebases the current branch onto `origin/<current-branch>`.

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

Pushes the current branch to `origin` with `--force-with-lease` and sets upstream tracking (`origin/<branch>`).

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

Opens a pull request on the configured SCM platform (GitHub or Bitbucket Server), routes reviewers according to `.git-brx.yaml`, and appends merge instructions.

### Flags
- `-r`, `--reviewer <user>`: Request specific reviewer(s) instead of using configured routing rules (repeatable or comma-separated).
- `-t`, `--title <title>`: Override the generated pull request title.
- `-s`, `--strategy <strategy>`: Override the merge strategy in PR instructions (`squash`, `merge`, or `rebase`).

### Examples
```bash
git brx review
git brx review develop
git brx review -r alice
git brx review -s merge
```

---

## 7. `git-brx delete`
**Usage:** `git-brx delete [flags] [<branch>]`

Deletes a local topic branch (`issue/*`, `feature/*`, or `epic/*`) after verifying that the remote branch was removed on `origin`, then runs `git remote prune origin`. Defaults to the active branch and switches to `main` (or default base) before deleting.

### Flags
- `-f`, `--force`: Skip checking whether the branch still exists on `origin`.

### Examples
```bash
git brx delete
git brx delete issue/101
git brx delete --force
```

---

## 8. `git-brx resolve`
**Usage:** `git-brx resolve [flags]`

Opens the configured merge tool (`git config merge.tool`) for each conflicted file, verifies conflict markers are resolved, stages **only** those resolved files, and continues the active rebase or merge.

### Flags
- `-t`, `--tool <tool>`: Override the configured Git merge tool.

### Examples
```bash
git brx resolve
git brx resolve -t vimdiff
```

---

## 9. `git-brx reset`
**Usage:** `git-brx reset [flags]`

Aborts any in-flight rebase or merge and runs `git reset --hard origin/<branch>`.

### Flags
- `-c`, `--clean`: Remove untracked files and directories (`git clean -fd`).

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
- `--topic`: Show only commits on the current topic branch since diverging from base (`<base>..HEAD`).
- `-s`, `--search <query>`: Filter commits by commit message (case-insensitive).
- `--stat`: Include file diffstats for each commit.
- `-p`, `--patch`: Include full diffs for each commit.

### Examples
```bash
git brx history -l 10
git brx history --topic
git brx history -s "fix memory leak"
git brx history --stat src/api
```

---

<a id="git-brx-config"></a>
## 11. `git-brx config`
**Usage:** `git-brx config [flags]`

Prints the effective configuration resolved across all four cascade tiers (User Global, Shared Global, Repo Local, and Remote Origin) in YAML format with provenance comments.

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
