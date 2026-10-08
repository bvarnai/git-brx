# Git with Bitbucket Server User's Guide - FAQ

Troubleshooting and frequently asked questions for Git and Bitbucket Server using the legacy `branch` tool.

---

## 1. Why do I get fatal authentication errors using branch commands?

### Symptom
Git commands fail with fatal authentication errors when communicating with Bitbucket or internal tools.

### Cause
Bitbucket credentials saved in Windows Credential Manager are out-of-date (e.g., after updating domain/MGC account passwords).

### Solution
Clear saved credentials from Windows Credential Manager:
1. Open Windows Start menu -> search for **Manage Windows Credentials** (Credential Manager).
2. Under **Windows Credentials**, locate and remove entries for:
   - `ies-iesd-bitbucket.ies.mentorg.com`
   - `tools@ies-vsb-ub.ies.mentorg.com`
3. Run the branch command again and enter your updated credentials when prompted.

---

## 2. Why do I get "publish failed error" and fatal authentication error using branch commands?

### Symptom
Running `git branch-publish` fails with authentication rejection.

### Cause
Stale credentials for the Bitbucket remote and internal tools repository.

### Solution
Remove stored Windows Credentials for the following targets:
- `user@ies-iesd-bitbucket.ies.mentorg.com`
- `ies-iesd-bitbucket.ies.mentorg.com`
- `tools@ies-vsb-ub.ies.mentorg.com`

---

## 3. Why do I get "CAPTCHA required" error message?

### Symptom
Bitbucket rejects pushes or CLI commands with a CAPTCHA challenge requirement.

### Cause
Too many failed authentication attempts against Bitbucket Server with expired credentials.

### Solution
1. Open a browser and navigate to `https://ies-iesd-bitbucket.ies.mentorg.com`.
2. Log out if currently signed in.
3. Log back in through the web UI and solve the CAPTCHA prompt.
4. Once verified in the browser, return to the CLI.

---

## 4. Why do I get "error: could not fetch ref" error message?

### Symptom
Git fetch fails with refspec errors such as:
```text
error: could not fetch ref
```

### Cause
The local `.git/config` contains outdated fetch refspecs pointing to deleted branches (for example, a deleted epic branch `epic/java17`):
```ini
[remote "origin"]
        url = https://ies-iesd-bitbucket.ies.mentorg.com/scm/vsb/tools.git
        fetch = +refs/heads/*:refs/remotes/origin/*
        fetch = +refs/heads/epic/java17:refs/remotes/origin/epic/java17
```

### Solution
Run the project installer script to refresh git refspecs:
```bash
./installer.sh update
```
These refspecs were initially configured by the installer to support shallow clones and custom development configurations. Alternatively, manually edit `.git/config` and remove the line referencing the deleted branch.

---

## 5. How to unshallow a repository?

### Symptom
When executing `git branch-publish` or history/rebase operations:
```text
[branch] ! You are in a shallow repository, this means some git commands might not work properly
[branch] Hint: Make a full clone of the repository and try again
```

### Solution
Shallow clones (`--depth`) can cause issues with Git graph history, rebases, and merge base resolution.
- Recommended: Re-clone the repository as a full clone.
- Or unshallow the existing repository using:
  ```bash
  git fetch --unshallow
  ```
