# Troubleshooting & FAQ

Common questions, error messages, and recovery steps for `git-brx`.

---

## Frequently asked questions

### Q: Why does `git-brx` require an issue for every branch?
**A:** Linking every branch to a Jira or GitHub issue keeps requirements, context, and acceptance criteria in one place. Reviewers don't have to guess what a PR is supposed to solve, and future maintainers can trace any commit back to its issue via `git blame`. For quick local spikes that don't need an issue, use `git checkout -b <branch>`.

### Q: Can I use `git-brx` offline?
**A:** Yes. Pass `--offline` (`-o`) to `create` or `select`:
```bash
git brx create --offline issue/TASK-101
git brx select --offline issue/TASK-101
```
This skips remote network checks and issue tracker API calls.

### Q: Does `git-brx` modify my global `~/.gitconfig`?
**A:** No. `git-brx` reads your existing Git configuration, credential helpers, and merge tools without modifying `~/.gitconfig`.

### Q: What is the difference between `sync` and `update`?
- **`git brx sync`**: Pulls new commits from the **base branch** into your topic branch (e.g., `main` $\to$ `issue/TASK-101`).
- **`git brx update`**: Pulls remote commits from **your current branch's upstream** (e.g., `origin/issue/TASK-101` $\to$ `issue/TASK-101`).

---

## Common errors & fixes

### 1. `[git-brx] Error: Publish rejected: remote has newer commits`
- **Cause:** Someone pushed commits to `origin/<your-branch>` since you last fetched, or your remote ref is ahead of your local lease.
- **Fix:** Run `git brx update` to rebase your local commits onto the remote branch, then run `git brx publish`.

### 2. `[git-brx] Error: Base branch 'main' is not up-to-date with 'origin/main'`
- **Cause:** Your local `main` branch has unpushed commits or has diverged from `origin/main`.
- **Fix:**
  ```bash
  git brx select main
  git brx update
  git brx select issue/TASK-101
  git brx sync
  ```

### 3. `[git-brx] Error: Rebase or merge is already in progress`
- **Cause:** A previous merge or rebase hit conflicts and was left unfinished.
- **Fix:**
  - To resolve conflicts and finish: `git brx resolve`
  - To abort and restore your branch to `origin/<branch>`: `git brx reset`

### 4. `[git-brx] Error: Branch must be deleted on remote first`
- **Cause:** You ran `git brx delete`, but `refs/heads/<branch>` still exists on `origin`.
- **Fix:**
  - Merge the PR and delete the remote branch on GitHub/Bitbucket (or enable **Automatically delete head branches** in your GitHub repo settings).
  - To delete the local branch anyway (e.g., an abandoned branch):
    ```bash
    git brx delete --force
    ```

### 5. `HTTP 401 Unauthorized` / `HTTP 403 Forbidden`
- **Cause:** Your API token is missing, expired, or lacks required repository permissions.
- **Fix:**
  - **GitHub:** Check `GITHUB_TOKEN` or `GH_TOKEN` and verify the token has `Issues: Read-only`, `Pull requests: Read & Write`, `Metadata: Read-only` (plus `Contents: Read-only` on private repos).
  - **Bitbucket / Jira:** Check `GIT_BRX_TOKEN`.
  - **Windows Credential Manager:** Open **Credential Manager**, remove stale entries for your Git host, and re-run the command.

### 6. `fatal: '.../git-doc/git-brx.html': documentation file not found`
- **Cause:** You ran `git brx --help`. Git intercepts `--help` on top-level subcommands (`git <plugin> --help`) and looks for a local manpage or HTML file.
- **Fix:**
  - Run **`git brx help`** or **`git brx -h`**.
  - On subcommands, `--help` works directly (`git brx create --help`, `git brx help sync`).
