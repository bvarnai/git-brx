#!/usr/bin/env bash
# scripts/setup-sandbox.sh
# Sets up an isolated git-brx-sandbox directory with dual workspaces:
#   - git-brx-sandbox/me (primary developer, inheriting your usual git config)
#   - git-brx-sandbox/colleague (Alex Developer)
#   - git-brx-sandbox/env.sh (sourceable script for PATH and autocomplete)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GIT_BRX_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
BASE_WORKSPACE_DIR="$(cd "${GIT_BRX_ROOT}/.." && pwd)"

SANDBOX_BASE="${BASE_WORKSPACE_DIR}/git-brx-sandbox"
USER_DIR="${SANDBOX_BASE}/me"
COLLEAGUE_DIR="${SANDBOX_BASE}/colleague"

REPO_NAME="${REPO_NAME:-git-brx-sandbox}"
GITHUB_TOKEN="${GITHUB_TOKEN:-${COWORKER_GITHUB_TOKEN:-}}"

# Resolve authenticated GitHub user dynamically
resolve_owner() {
    if [[ -n "${REPO_OWNER:-}" ]]; then
        return 0
    fi
    if [[ -n "${GITHUB_TOKEN:-}" ]]; then
        local user_login
        user_login=$(curl -s -H "Authorization: Bearer ${GITHUB_TOKEN}" https://api.github.com/user | jq -r '.login // empty' 2>/dev/null || true)
        if [[ -n "$user_login" && "$user_login" != "null" ]]; then
            REPO_OWNER="$user_login"
            return 0
        fi
    fi
    local origin_url
    origin_url=$(git -C "${GIT_BRX_ROOT}" remote get-url origin 2>/dev/null || true)
    REPO_OWNER=$(echo "$origin_url" | sed -E 's/.*[:/]([^/]+)\/[^/]+(\.git)?$/\1/' || true)
    if [[ -z "$REPO_OWNER" ]]; then
        REPO_OWNER="bvarnai"
    fi
}
resolve_owner

REMOTE_SSH="git@github.com:${REPO_OWNER}/${REPO_NAME}.git"
COLLEAGUE_NAME="Alex Developer"
COLLEAGUE_EMAIL="alex.developer@example.com"

usage() {
    cat <<EOF
Usage: $0 [OPTIONS]

Options:
  --check    Verify token, remote sandbox repository, and local workspaces
  --reset    Remove local git-brx-sandbox/me and git-brx-sandbox/colleague directories and re-clone
  --help     Show this help message
EOF
}

CHECK_MODE=false
RESET_MODE=false

for arg in "$@"; do
    case "$arg" in
        --check)
            CHECK_MODE=true
            ;;
        --reset)
            RESET_MODE=true
            ;;
        --help|-h)
            usage
            exit 0
            ;;
        *)
            echo "Unknown option: $arg" >&2
            usage >&2
            exit 1
            ;;
    esac
done

echo "==> [Simulation Setup] Initializing Sandbox Environment..."

# 1. Verify GITHUB_TOKEN
if [[ -z "$GITHUB_TOKEN" ]]; then
    echo "Error: GITHUB_TOKEN is not set. Please export GITHUB_TOKEN with repo permissions." >&2
    exit 1
fi

# 2. Check/Create Remote Repository and Seed Issues on GitHub
"${SCRIPT_DIR}/sandbox-github.sh" --init-repo
if [[ "$CHECK_MODE" == "true" ]]; then
    echo "==> Check complete. Remote repository is ready."
    exit 0
fi
"${SCRIPT_DIR}/sandbox-github.sh" --seed-issues

# 3. Ensure git-brx binary is compiled
echo "==> Ensuring git-brx binary is compiled..."
if [[ ! -f "${GIT_BRX_ROOT}/bin/git-brx" ]]; then
    (cd "${GIT_BRX_ROOT}" && go build -o bin/git-brx ./cmd/git-brx)
fi
echo "    git-brx binary ready at ${GIT_BRX_ROOT}/bin/git-brx"

# 4. Handle Reset if requested
if [[ "$RESET_MODE" == "true" ]]; then
    echo "==> Resetting local sandbox workspaces..."
    rm -rf "${USER_DIR}" "${COLLEAGUE_DIR}"
fi

mkdir -p "${SANDBOX_BASE}"

# 5. Generate sourceable environment script (env.sh)
echo "==> Generating shell activation script at ${SANDBOX_BASE}/env.sh..."
cat <<ENV_EOF > "${SANDBOX_BASE}/env.sh"
# Environment activation script for git-brx simulation sandbox
# Usage: source ${SANDBOX_BASE}/env.sh

export PATH="${GIT_BRX_ROOT}/bin:\${PATH}"

# 1. Enable direct 'git-brx' completion via Cobra
if command -v git-brx >/dev/null 2>&1; then
    source <("${GIT_BRX_ROOT}/bin/git-brx" completion bash 2>/dev/null)
fi

# 2. Ensure system git completion is loaded
if ! declare -f _git >/dev/null 2>&1; then
    if [ -f /usr/share/bash-completion/completions/git ]; then
        source /usr/share/bash-completion/completions/git
    elif [ -f /etc/bash_completion.d/git-prompt ]; then
        source /etc/bash_completion.d/git-prompt
    fi
fi

# 3. Enable Git subcommand completion for 'git brx <TAB>'
_git_brx() {
    local cur="\${words[cword]}"
    local sub_args=("\${words[@]:2}")
    local out
    out=\$(git-brx __complete "\${sub_args[@]}" 2>/dev/null)
    local completions
    completions=\$(echo "\$out" | grep -v '^:' | awk '{print \$1}')
    COMPREPLY=(\$(compgen -W "\$completions" -- "\$cur"))
}

echo "[git-brx-sandbox] Environment activated:"
echo "  PATH prepended:  ${GIT_BRX_ROOT}/bin"
echo "  Autocomplete:    Enabled for 'git-brx' and 'git brx'"
echo "  Workspaces:"
echo "    Me:            ${USER_DIR}"
echo "    Colleague:     ${COLLEAGUE_DIR}"
ENV_EOF
chmod +x "${SANDBOX_BASE}/env.sh"
echo "    env.sh generated."

# 6. Setup User Workspace (me)
echo "==> Setting up User workspace at ${USER_DIR}..."
if [[ ! -d "${USER_DIR}/.git" ]]; then
    git clone "${REMOTE_SSH}" "${USER_DIR}"
fi
(
    cd "${USER_DIR}"
    # Inherits user's standard global git config without hardcoded overrides
    mkdir -p bin
    ln -sf "${GIT_BRX_ROOT}/bin/git-brx" "bin/git-brx"
)
CURRENT_USER_NAME=$(git -C "${USER_DIR}" config user.name || echo "Default User")
CURRENT_USER_EMAIL=$(git -C "${USER_DIR}" config user.email || echo "default@example.com")
echo "    User workspace configured (${CURRENT_USER_NAME} <${CURRENT_USER_EMAIL}>)"

# 7. Setup Colleague Workspace (colleague)
echo "==> Setting up Colleague workspace at ${COLLEAGUE_DIR}..."
if [[ ! -d "${COLLEAGUE_DIR}/.git" ]]; then
    git clone "${REMOTE_SSH}" "${COLLEAGUE_DIR}"
fi
(
    cd "${COLLEAGUE_DIR}"
    git config user.name "${COLLEAGUE_NAME}"
    git config user.email "${COLLEAGUE_EMAIL}"
    mkdir -p bin
    ln -sf "${GIT_BRX_ROOT}/bin/git-brx" "bin/git-brx"
)
echo "    Colleague workspace configured (${COLLEAGUE_NAME} <${COLLEAGUE_EMAIL}>)"

# 8. Seed Initial Sample Project on main if empty
(
    cd "${USER_DIR}"
    if [[ ! -f "calc.go" ]]; then
        echo "==> Seeding initial sample files in sandbox repository..."
        cat <<'GO_EOF' > calc.go
package main

import "fmt"

func Add(a, b int) int {
	return a + b
}

func Multiply(a, b int) int {
	return a * b
}

func main() {
	fmt.Printf("Calculation result: %d\n", Add(10, 20))
}
GO_EOF

        cat <<'CFG_EOF' > config.json
{
  "serviceName": "sandbox-app",
  "version": "1.0.0",
  "environment": "development",
  "retries": 3,
  "timeoutSeconds": 30
}
CFG_EOF

        cat <<'MD_EOF' > README.md
# git-brx Sandbox Project

This repository is dedicated to simulating multi-developer collaborative workflows,
friction points, and edge cases using `git-brx`.

## Workspaces
- **Primary Developer:** `git-brx-sandbox/me`
- **Colleague Developer:** `git-brx-sandbox/colleague` (Alex Developer)
MD_EOF

        git add calc.go config.json README.md
        if git commit -m "chore: seed initial sandbox project files"; then
            git push origin main
            echo "    Sample files pushed to origin/main."
            # Sync colleague clone
            (cd "${COLLEAGUE_DIR}" && git pull origin main)
        fi
    fi
)

echo ""
echo "================================================================="
echo " Sandbox Environment Ready!"
echo "================================================================="
echo " Base Directory:      ${SANDBOX_BASE}"
echo " Source in Shell:     source ${SANDBOX_BASE}/env.sh"
echo " User Workspace:      ${USER_DIR}"
echo " Colleague Workspace: ${COLLEAGUE_DIR}"
echo " GitHub Remote:       https://github.com/${REPO_OWNER}/${REPO_NAME}"
echo " git-brx binary:      ${GIT_BRX_ROOT}/bin/git-brx"
echo "================================================================="
