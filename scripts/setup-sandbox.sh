#!/usr/bin/env bash
# scripts/setup-sandbox.sh
# Sets up a dedicated GitHub sandbox repository and dual local workspaces
# for simulating multi-developer collaboration with git-brx.
set -euo pipefail

REPO_OWNER="bvarnai"
REPO_NAME="git-brx-sandbox"
REMOTE_SSH="git@github.com:${REPO_OWNER}/${REPO_NAME}.git"
REMOTE_HTTPS="https://github.com/${REPO_OWNER}/${REPO_NAME}.git"

BASE_WORKSPACE_DIR="/home/bvarnai/workspace"
SANDBOX_BASE="${BASE_WORKSPACE_DIR}/sandbox"
USER_DIR="${SANDBOX_BASE}/bvarnai"
COLLEAGUE_DIR="${SANDBOX_BASE}/colleague"
GIT_BRX_ROOT="/home/bvarnai/workspace/git-brx"

GITHUB_TOKEN="${GITHUB_TOKEN:-${COWORKER_GITHUB_TOKEN:-}}"

COLLEAGUE_NAME="Alex Developer"
COLLEAGUE_EMAIL="alex.developer@example.com"

usage() {
    cat <<EOF
Usage: $0 [OPTIONS]

Options:
  --check    Verify token, remote sandbox repository, and local workspaces
  --reset    Remove local sandbox/bvarnai and sandbox/colleague directories and re-clone
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

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

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

# 5. Setup User Workspace (bvarnai)
echo "==> Setting up User workspace at ${USER_DIR}..."
if [[ ! -d "${USER_DIR}/.git" ]]; then
    git clone "${REMOTE_SSH}" "${USER_DIR}"
fi
(
    cd "${USER_DIR}"
    # Inherit default user's git config without hardcoding overrides
    mkdir -p bin
    ln -sf "${GIT_BRX_ROOT}/bin/git-brx" "bin/git-brx"
)
CURRENT_USER_NAME=$(git -C "${USER_DIR}" config user.name || echo "Default")
CURRENT_USER_EMAIL=$(git -C "${USER_DIR}" config user.email || echo "Default")
echo "    User workspace configured (${CURRENT_USER_NAME} <${CURRENT_USER_EMAIL}>)"

# 6. Setup Colleague Workspace
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

# 7. Seed Initial Sample Project on main if empty
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

## Collaborators
- **Primary Developer:** Balazs Varnai (`sandbox-user`)
- **Colleague Developer:** Alex Developer (`sandbox-colleague`)
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
echo " User Workspace:      ${USER_DIR}"
echo " Colleague Workspace: ${COLLEAGUE_DIR}"
echo " GitHub Remote:       https://github.com/${REPO_OWNER}/${REPO_NAME}"
echo " git-brx binary:      Accessible in both workspace ./bin/ directories"
echo "================================================================="
