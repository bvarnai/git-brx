# Git Workflow Guidelines for git-brx

When working in this repository, always follow the `git-brx` workflow and use the `git-brx` CLI (or `git brx`) for all branch management operations:

1. **Branch Creation:**
   - Always create topic branches via `git brx create <branch> -y`.
   - Never use raw `git checkout -b` or `git switch -c` directly for topic branches.
   - Use the appropriate prefix:
     - `issue/<key>` for standard tasks and bugs (linear rebase sync)
     - `feature/<key>` for collaborative features (merge sync)
     - `epic/<key>` for multi-commit initiatives

2. **Upstream Synchronization:**
   - Synchronize with the base branch (`main`) using `git brx sync`.
   - Pull teammate commits on shared branches using `git brx update`.

3. **Conflict Handling:**
   - After editing conflict markers, continue with `git brx resolve`.
   - To abort a failed sync/rebase, use `git brx reset`.

4. **Publishing:**
   - Publish branches to origin with upstream tracking using `git brx publish`.
   - Do not run `git push -f` or manual `--force-with-lease` manually.

5. **Code Review:**
   - Submit pull requests using `git brx review`.
