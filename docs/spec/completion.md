# Spec: git-brx completion

## 1. Command Signature
- **Usage:** `git-brx completion [bash|zsh|fish|powershell]`
- **Arguments:**
  - `<shell>` (Required): Target shell interpreter (`bash`, `zsh`, `fish`, or `powershell`).
- **Flags:**
  - Standard global flags: `--help` (`-h`).
  - Positional argument validation: Rejects missing shell argument or unsupported shell with Exit Code `2`.

## 2. Prerequisites & Environment
- **Required host binaries:** None. Shell completion scripts are generated autonomously in-process via Cobra.
- **Environment variables read:** None.
- **Working directory requirements (GIT_PREFIX behavior):** Can be invoked from any directory on the host filesystem; does not require being inside a Git repository.
- **Git state assertions (e.g., dirty working tree check):**
  - None for static completion script generation.
  - During shell completion callback evaluation (`completeBranchNames`), queries Git repository state if invoked inside a valid worktree.

## 3. Core Execution Flow
1. **Argument Validation:** Verify exactly one positional argument is provided. Validate that the argument is one of `bash`, `zsh`, `fish`, or `powershell`. If missing or invalid, fail immediately with Exit Code `2`.
2. **Script Generation:**
   - For `bash`: Emit Bash completion script via `GenBashCompletion(stdout)`.
   - For `zsh`: Emit Zsh completion script via `GenZshCompletion(stdout)`.
   - For `fish`: Emit Fish completion script via `GenFishCompletion(stdout, true)`.
   - For `powershell`: Emit PowerShell completion script via `GenPowerShellCompletionWithDesc(stdout)`.
   - Output is written strictly to `stdout` with zero log prefixes or decoration, ensuring safe evaluation in shell profiles (e.g., `source <(git-brx completion bash)`).
3. **Dynamic Branch Name Autocompletion (`completeBranchNames`):**
   - Cobra registers dynamic branch name autocompletion callback for commands and flags expecting a branch name (e.g., `select`, `sync`, `delete`, `review`, and `history -b`).
   - When invoked in the shell, inspects the current working directory.
   - If not inside a Git work tree, silently returns empty completions with `cobra.ShellCompDirectiveNoFileComp`.
   - If inside a Git work tree, queries `inspector.ListAllBranchNames(ctx, workDir)` and filters by prefix match against the user's partially typed token.

## 4. Error Handling & Exit Codes
| Exit Code | Scenario | Emitted Stderr Pattern | Side Effect Cleanup |
| :--- | :--- | :--- | :--- |
| `0` | Shell completion script generated successfully. | None. | None. |
| `2` | Missing shell argument, invalid shell identifier, or unknown flags. | `[git-brx] Error: invalid argument "<shell>"` / `[git-brx] Error: unknown flag: <flag>` | None. |

## 5. Edge Cases & Behavioral Notes
- **Autonomous Execution:** `completion` never checks for Git repositories or configuration files, ensuring shell startup scripts (`~/.bashrc`, `~/.zshrc`) never block or fail even when invoked in arbitrary or non-Git directories.
- **Clean Stdout Stream:** `completion` scripts are emitted directly to `stdout` without any `[git-brx]` operational log prefixes, ensuring compatibility with standard shell sourcing pipelines.
- **Non-blocking Dynamic Callbacks:** Dynamic branch completion callbacks never emit error messages or warnings to `stderr` if the repository is dirty or origin is unreachable, preventing noise during interactive tab-completion.
