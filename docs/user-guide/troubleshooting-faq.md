# Troubleshooting & FAQ

Frequently asked questions, common error diagnostics, and recovery tips for `git-brx`.

---

## Frequently asked questions

### Q: Why does `git-brx` require an issue for every branch?
**A:** Traceability and team velocity. When every change is tethered to a ticket in Jira or GitHub Issues, the team maintains an authoritative record of *why* changes were introduced. Code reviewers don't have to guess requirements, and future maintainers can immediately see the rationale behind complex changes via `git blame`.

### Q: Can I use `git-brx` without network access or offline?
**A:** Yes! Both `create` and `select` feature an `--offline` (`-o`) flag:
```bash
git brx create --offline issue/TASK-101
git brx select --offline issue/TASK-101
```
This bypasses remote network calls and allows full offline productivity.

### Q: Does `git-brx` touch my global `~/.gitconfig`?
**A:** No. `git-brx` never mutates your global Git configuration. It respects your existing credentials, diff tools, and git settings natively.

### Q: What is the difference between `sync` and `update`?
- **`git brx sync`**: Brings in new work from your **base branch** (e.g. `master` ➔ `issue/TASK-101`).
- **`git brx update`**: Pulls remote commits on your **own branch** (e.g. `origin/issue/TASK-101` ➔ `issue/TASK-101`).

---

## Common error diagnostics

### 1. `! Publish rejected: remote has newer commits`
- **Cause:** Someone pushed commits to `origin/<your-branch>` since you last fetched, or you rebased locally without updating from remote first.
- **Solution:** Run `git brx update` to rebase your local commits on top of the remote changes, then re-run `git brx publish`.

### 2. `! Sync branch 'master' is not up-to-date`
- **Cause:** Your local `master` branch has diverged from `origin/master` (it is either behind or contains local unpushed commits).
- **Solution:**
  ```bash
  git brx select master
  git brx update
  git brx select issue/TASK-101
  git brx sync
  ```

### 3. `! You are in the middle of a rebase/merge`
- **Cause:** A previous merge or rebase was interrupted or hit unresolved conflicts.
- **Solution:**
  - If you want to finish the rebase: run `git brx resolve`.
  - If you want to abort and start over cleanly: run `git brx reset`.

### 4. `! Branch must be deleted on remote first`
- **Cause:** You ran `git brx delete`, but the branch is still alive on GitHub or Bitbucket.
- **Solution:**
  - Verify that the Pull Request has been merged on the remote platform, and delete the branch through the PR UI.
  - If you created an experimental local branch that was never pushed to remote, use the override flag:
    ```bash
    git brx delete --force
    ```

### 5. `! Authorization failure` / `HTTP 401 Unauthorized`
- **Cause:** Your personal access token has expired or is missing.
- **Solution:**
  - For GitHub: update `GITHUB_TOKEN` or `GH_TOKEN` in your environment.
  - For Bitbucket / Jira: update `GIT_BRX_TOKEN` in your environment.
  - On Windows: if your credential manager stored expired passwords, search for **Credential Manager** in Windows Start, remove the stale entry for your Git host, and re-run the command.
