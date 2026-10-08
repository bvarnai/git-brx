# Spec: git-brx help

## 1. Command Signature
- **Usage:** `git-brx help [flags] [<subcommand>]`
- **Arguments:**
  - `<subcommand>` *(optional)*: Specific subcommand name to view detailed documentation for (e.g., `create`, `sync`, `review`).
- **Flags:**
  - Standard global flags: `--help` (`-h`), `--version`, `--no-color`.

## 2. Prerequisites & Environment
- **Required host binaries:** None. (Autonomous CLI help generation).
- **Environment variables read:** `NO_COLOR`.
- **Working directory requirements (GIT_PREFIX behavior):** Can be invoked from any directory on the host filesystem; does NOT require being inside a Git repository.
- **Git state assertions (e.g., dirty working tree check):**
  - No Git state requirements. Must execute reliably even in an empty or non-Git directory.

## 3. Core Execution Flow
1. **Catalog vs Subcommand Routing:**
   - If positional argument `<subcommand>` is absent:
     - Print the global CLI catalog, displaying tool overview, standard syntax, and the table of all available subcommands:
       ```text
       Usage: git-brx <command> [options]... [arguments]...

       Available commands:
         name       Get the current (local) branch name
         history    Display a formatted commit graph
         update     Update the local branch using rebase
         select     Select and switch to an existing branch ('master' by default)
         sync       Sync changes from a branch ('master' by default)
         reset      Discard working changes and restore state from origin
         resolve    Resolve merge/rebase conflicts using configured mergetool
         create     Create a new validated development branch
         publish    Push local changes to remote with lease and set upstream
         delete     Delete the current branch after remote deletion check
         review     Create a pull request in Bitbucket Server
         help       Display help information for git-brx commands
       ```
     - Exit with Exit Code `0`.
2. **Subcommand-Specific Help Routing:**
   - If positional argument `<subcommand>` is provided:
     - Validate that `<subcommand>` is one of the recognized commands.
     - If recognized, display detailed usage syntax, arguments, flags, and operational examples for that specific command.
     - Exit with Exit Code `0`.
   - If `<subcommand>` is unrecognized:
     - Log: `[git-brx] ! Unknown subcommand '<subcommand>'`
     - Log: `[git-brx] Hint: Run 'git-brx help' to see all available commands`
     - Fail with Exit Code `2`.

## 4. Error Handling & Exit Codes
| Exit Code | Scenario | Emitted Stderr Pattern | Side Effect Cleanup |
| :--- | :--- | :--- | :--- |
| `0` | Help catalog or subcommand documentation displayed. | None. | None. |
| `2` | Unknown subcommand passed to help. | `[git-brx] ! Unknown subcommand '<subcommand>'` | None. |

## 5. Discrepancies & Edge Cases Discovered
- **Non-Repository Failure Anti-Pattern:** Legacy `help.sh` invoked `branch::common::read_repository` as its first step. Consequently, running `git branch-help` outside a Git repository aborted with `Awh! This is not a git repository`! A help command must be universally available. The Go implementation decouples `help` from Git repository validation.
- **Subcommand Documentation Routing:** Legacy `help.sh` only displayed the global catalog and directed users to `git branch-<command> help`. The modern Go implementation supports querying subcommand help directly via `git-brx help <command>` or `git-brx <command> --help`.
- **Hardcoded Intranet URLs:** Legacy help output hardcoded intranet documentation URLs (`http://ies-iesd-conf.ies.mentorg.com:8090/...`). Modern Go help provides self-contained CLI usage and links to configurable repository documentation.
