# Spec: git-brx history

## 1. Command Signature
- **Usage:** `git-brx history [flags] [<path>]`
- **Arguments:**
  - `[<path>]` (Optional): Scope history to a specific file or directory path. Automatically passes `--follow` for regular files to track commits across renames.
- **Flags:**
  - Standard global flags: `--help` (`-h`), `--version`, `--verbose` (`-v`), `--quiet` (`-q`), `--dry-run` (`-n`), `--no-color`.
  - Extension flags:
    - `--limit` (`-l` `<int>`): Limit the number of commits rendered in the graph (default: unconstrained or pager-governed).
    - `--branch` (`-b` `<string>`): Scope history to a specific named branch.
    - `--topic`: Show only commits on active topic branch relative to the base branch (`master` / default).
    - `--search` (`-s` `<string>`): Filter commits whose message matches the query string (case-insensitive).
    - `--stat`: Show diffstat summary of changed files for each commit.
    - `--patch` (`-p`): Show code diff patch for each commit.

## 2. Prerequisites & Environment
- **Required host binaries:** `git` (>= 2.20.0).
- **Environment variables read:** `GIT_DIR`, `GIT_WORK_TREE`, `GIT_PREFIX`, `PAGER`, `NO_COLOR`.
- **Working directory requirements (GIT_PREFIX behavior):** Can be invoked from any subdirectory inside a Git repository. Operates invariant of `GIT_PREFIX`. Path arguments are resolved relative to the invoking directory.
- **Git state assertions (e.g., dirty working tree check):**
  - Must be inside a Git working tree (`git rev-parse --is-inside-work-tree` == `true`).
  - Repository must contain at least one valid commit ref (HEAD must not be an unborn root).
  - Safe to execute regardless of dirty working tree, detached HEAD, or uncommitted modifications.

## 3. Core Execution Flow
1. **Repository Verification:** Run `git rev-parse --is-inside-work-tree`. If non-zero, fail with Exit Code `3`.
2. **Commit Existence Check:** Verify HEAD points to an existing commit: `git rev-parse --verify -q HEAD`. If non-zero, output diagnostic note: `[git-brx] Error: Repository has no commits yet` and exit with Exit Code `3`.
3. **Query Parameter Assembly:**
   - Base command: `git log --pretty=tformat:"%h %ad | %s%d [%an]" --graph --decorate --date=short`
   - Limit: If `--limit` > 0, append `--max-count=<N>`.
   - Search: If `--search` is non-empty, append `--grep=<query>` and `-i`.
   - Diff peeking: If `--stat` is active, append `--stat`. If `--patch` / `-p` is active, append `-p`.
   - Branch relativity: If `--branch` / `-b` is provided, append `<branch>`. If `--topic` is active, resolve base branch (`master` or autodetected default) and append `<base>..<current_branch>`. If current branch is identical to base branch, emit notice (`[git-brx] You are on base branch '<base>'; there is no topic branch delta to display`) and return cleanly without outputting the full base branch log.
   - Path scoping: If `<path>` is provided, check if path is a directory. If directory, append `-- <path>`. If regular file or past path, append `--follow -- <path>`.
4. **Color & Pager Management:** If `--no-color` or `NO_COLOR` is active, pass `--no-color` to Git. If `stdout` is connected to a TTY, attach standard Git pager configuration (`PAGER` or `less`).

## 4. Error Handling & Exit Codes
| Exit Code | Scenario | Emitted Stderr Pattern | Side Effect Cleanup |
| :--- | :--- | :--- | :--- |
| `0` | History graph rendered cleanly. | None. | None. |
| `2` | Unknown flags or unexpected positional arguments passed. | `[git-brx] Error: Unexpected argument: <arg>` | None. |
| `3` | Outside Git repository or empty repository without commits. | `[git-brx] Error: Awh! This is not a git repository` / `[git-brx] Error: Repository has no commits yet` | None. |
| `1` | Subprocess execution or pager pipe failure. | `[git-brx] Error: Getting history failed (git log failed)` | None. |

## 5. Discrepancies & Edge Cases Discovered
- **Empty / Unborn Repositories:** Legacy `history.sh` blindly executed `git log`, which fails abruptly with `fatal: your current branch 'master' does not have any commits yet` if invoked on a newly initialized repository before the first commit. Modern Go implementation explicitly checks for valid commits before attempting to format the log.
- **Argument Discard Bug:** Legacy script accepted extra arguments, logged a discard notice to `stdout`, and dropped them. Modern Go parser must enforce strict CLI contracts (Exit Code `2`).
- **Terminal Width & Pager Discrepancy:** On large repositories, legacy Bash scripts had no pager interception, flooding terminal buffers. Modern implementation respects standard pager settings when stdout is an interactive TTY.
