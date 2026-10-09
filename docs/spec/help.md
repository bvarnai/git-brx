# Spec: git-brx help

## 1. Command Signature
- **Usage:** `git-brx help [<subcommand>]`
- **Arguments:**
  - `<subcommand>` *(optional)*: Specific subcommand name to view detailed documentation for (e.g., `create`, `sync`, `review`).
- **Flags:**
  - Standard global flags: `--help` (`-h`), `--version`, `--no-color`, `--quiet` (`-q`), `--verbose` (`-v`).

## 2. Prerequisites & Environment
- **Required host binaries:** None. (Autonomous CLI help generation).
- **Environment variables read:** `NO_COLOR`.
- **Working directory requirements (GIT_PREFIX behavior):** Can be invoked from any directory on the host filesystem; does NOT require being inside a Git repository.
- **Git state assertions (e.g., dirty working tree check):**
  - No Git state requirements. Executes reliably in empty or non-Git directories.

## 3. Core Execution Flow
1. **Catalog vs Subcommand Routing:**
   - If positional argument `<subcommand>` is absent (e.g. `git brx help`):
     - Displays the global CLI catalog, showing tool overview, standard usage, and the table of all available subcommands.
     - Serves as the primary cross-platform help mechanism when `git brx --help` is intercepted by Git on Windows/Linux environments looking for external manpages or HTML documentation (`git-doc`).
     - Exits with Exit Code `0`.
2. **Subcommand-Specific Help Routing:**
   - If positional argument `<subcommand>` is provided (e.g. `git brx help create`):
     - Displays detailed usage syntax, arguments, flags, and operational description for that specific subcommand.
     - Exits with Exit Code `0`.

## 4. Error Handling & Exit Codes
| Exit Code | Scenario | Emitted Stderr Pattern | Side Effect Cleanup |
| :--- | :--- | :--- | :--- |
| `0` | Help catalog or subcommand documentation displayed cleanly. | None. | None. |

## 5. Architectural Context
- **Git Plugin Help Interception:** When running `git <subcommand> --help`, Git's external command wrapper intercepts `--help` before launching the plugin and searches Git's internal documentation directory (e.g., `git-doc/git-brx.html` or `man git-brx`).
- By providing a native `help` subcommand (`git brx help`), users can view full CLI usage and command documentation in all Git environments (Windows Git Bash, PowerShell, Linux, macOS) without triggering Git's doc interception error.
