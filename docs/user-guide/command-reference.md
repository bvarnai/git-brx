# Complete Command Reference

This chapter documents all 10 subcommands available in `git-brx`, including flag definitions, accepted arguments, error codes, and practical examples.

---

## Global Flags

The following flags are accepted across all commands:

| Flag | Shorthand | Description |
| :--- | :--- | :--- |
| `--verbose` | `-v` | Enables verbose diagnostic output to `stderr` (e.g. raw Git commands, HTTP requests). |
| `--quiet` | `-q` | Suppresses informational notices and hints; only errors and primary stdout data are printed. |
| `--dry-run` | `-n` | Simulates execution without mutating Git references, working tree, or remote services. |
| `--no-color` | | Explicitly disables ANSI color formatting. |
| `--help` | `-h` | Displays usage and help text for the command. |
| `--version` | | (Root only) Displays semantic version, commit hash, build date, and compiler runtime. |

---

## 1. `git-brx create`
**Usage:** `git-brx create [flags] <branch>`

Provisions a new local topic branch after validating branch naming rules and issue status with the configured issue tracker.

### Flags:
- `-o`, `--offline`: Skip issue tracker verification and remote ref availability checks.
- `-y`, `--yes`: Automatically accept confirmation prompts without interactive confirmation.

### Examples:
```bash
git brx create issue/VSB-101
git brx create --offline feature/NEW-FEATURE
git brx create -y issue/VSB-102
```

---

## 2. `git-brx select`
**Usage:** `git-brx select [flags] [<branch>]`

Selects and switches the working tree to an existing branch (`master` by default). Performs remote fetch, warns on uncommitted changes, and provides fuzzy typo suggestions.

### Flags:
- `-o`, `--offline`: Skip remote `git fetch` and switch between local branches only.

### Examples:
```bash
git brx select                    # Switches to default master/main branch
git brx select issue/VSB-101       # Switches to issue/VSB-101
git brx select -o feature/offline # Switches offline without fetching
```

---

## 3. `git-brx sync`
**Usage:** `git-brx sync [flags] [<branch>]`

Synchronizes the active branch with target base branch (`master` by default). Automatically rebases for `issue/*` branches and merges for `feature/*` branches.

### Flags:
- `-m`, `--merge`: Force merge strategy regardless of branch default.
- `-r`, `--rebase`: Force rebase strategy regardless of branch default.
- `-a`, `--autostash`: Automatically stash and re-apply dirty working tree changes (rebase only).
- `-i`, `--interactive`: Launch interactive rebase (rebase only).

### Examples:
```bash
git brx sync
git brx sync -m develop
git brx sync -a
```

---

## 4. `git-brx update`
**Usage:** `git-brx update [flags]`

Pulls and rebases remote changes for the current branch from `origin/<current-branch>`.

### Flags:
- `-a`, `--autostash`: Automatically stash local changes before rebasing and pop afterwards.

### Examples:
```bash
git brx update
git brx update --autostash
```

---

## 5. `git-brx publish`
**Usage:** `git-brx publish [flags]`

Pushes current branch to remote origin with `--force-with-lease` and establishes upstream tracking.

### Flags:
- `--no-force`: Push with standard fast-forward constraints without lease override.

### Examples:
```bash
git brx publish
git brx publish --dry-run
```

---

## 6. `git-brx review`
**Usage:** `git-brx review [flags] [<target_branch>]`

Creates a pull request / code review on the configured SCM platform (GitHub, Bitbucket Server), automatically assigning reviewers and appending standardized merge instructions.

### Flags:
- `-r`, `--reviewer <user>`: Explicitly specify a reviewer instead of automated component routing.
- `-t`, `--title <title>`: Override the default pull request title.

### Examples:
```bash
git brx review
git brx review develop
git brx review -r alice
git brx review --dry-run
```

---

## 7. `git-brx delete`
**Usage:** `git-brx delete [flags]`

Validates that the current topic branch was merged on the remote SCM, switches to `master`, deletes the local branch, and prunes stale remote references.

### Flags:
- `-f`, `--force`: Bypass remote deletion verification (useful for discarding unpublished local branches).

### Examples:
```bash
git brx delete
git brx delete --force
```

---

## 8. `git-brx resolve`
**Usage:** `git-brx resolve [flags]`

Launches the configured GUI merge tool (`git config merge.tool`) for each conflicted file, verifies conflict resolution, stages **only** the resolved files, and continues the active rebase or merge.

### Flags:
- `-t`, `--tool <tool>`: Specify explicit merge tool override.

### Examples:
```bash
git brx resolve
```

---

## 9. `git-brx reset`
**Usage:** `git-brx reset [flags]`

Aborts any in-flight rebase or merge and hard-resets the working tree to the current remote tracking ref (`origin/<branch>`).

### Flags:
- `-c`, `--clean`: Also remove untracked files and directories (`git clean -fd`).

### Examples:
```bash
git brx reset
git brx reset --clean
```

---

## 10. `git-brx history`
**Usage:** `git-brx history [flags] [<path>]`

Renders a colorized, formatted Git commit graph with date and author details.

### Flags:
- `-l`, `--limit <N>`: Limit number of commits displayed.
- `-b`, `--branch <name>`: Scope history to a specific branch.
- `--topic`: Show only commits on active topic branch relative to the base branch (`master..HEAD`).
- `-s`, `--search <query>`: Case-insensitive search of commit messages.
- `--stat`: Show diffstat summary of modified files.
- `-p`, `--patch`: Show code diff patch for each commit.

### Examples:
```bash
git brx history -l 10
git brx history --topic
git brx history -s "fix memory leak"
git brx history --stat src/api
```

---

## 11. `git-brx name`
**Usage:** `git-brx name [flags]`

Prints the current branch name to `stdout` with a trailing newline. Plumbing-safe for script variable capture (`BRANCH=$(git brx name)`).
