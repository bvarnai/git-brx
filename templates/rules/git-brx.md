# Git workflow guidelines for git-brx

When working in this repository, always follow the `git-brx` workflow and use the `git-brx` CLI (or `git brx`) for all branch management operations:

1. **Branch creation:**
   - Always create work branches via `git brx create <branch> -y` (pass `--base <epic>` when forking a child slice from an `epic/*` integration branch).
   - Never use raw `git checkout -b` or `git switch -c` directly for work branches.

2. **Upstream synchronization:**
   - Synchronize with the recorded `brx-base` or `main` using `git brx sync`.
   - Pull teammate commits on shared branches using `git brx update`.

3. **Conflict handling:**
   - After editing conflict markers, continue with `git brx resolve`.
   - To abort a failed sync/rebase, use `git brx reset`.

4. **Publishing:**
   - Publish branches to origin with upstream tracking using `git brx publish`.
   - Do not run `git push -f` or manual `--force-with-lease` manually.

5. **Code review:**
   - Submit pull requests using `git brx review`.

6. **Branch cleanup:**
   - After the pull request merges, delete the local branch and return to its base with `git brx delete` (or `git brx delete --delete-remote` if automatic remote branch deletion is disabled).
