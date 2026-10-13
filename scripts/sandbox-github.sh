#!/usr/bin/env bash
# scripts/sandbox-github.sh
# Handles remote GitHub API operations for the sandbox repository:
# creating the repository, seeding test issues, and creating ad-hoc issues.
set -euo pipefail

ADMIN_GITHUB_TOKEN="${ADMIN_GITHUB_TOKEN:-${GITHUB_ADMIN_TOKEN:-${GITHUB_TOKEN:-}}}"
ME_GITHUB_TOKEN="${ME_GITHUB_TOKEN:-${USER_GITHUB_TOKEN:-${GITHUB_TOKEN:-}}}"
ALEX_GITHUB_TOKEN="${ALEX_GITHUB_TOKEN:-${COWORKER_GITHUB_TOKEN:-}}"

resolve_owner() {
    if [[ -n "${REPO_OWNER:-}" ]]; then
        return 0
    fi
    local probe_token="${ADMIN_GITHUB_TOKEN:-${ME_GITHUB_TOKEN:-}}"
    if [[ -n "$probe_token" ]]; then
        local user_login
        user_login=$(curl -s -H "Authorization: Bearer ${probe_token}" https://api.github.com/user | jq -r '.login // empty' 2>/dev/null || true)
        if [[ -n "$user_login" && "$user_login" != "null" ]]; then
            REPO_OWNER="$user_login"
            return 0
        fi
    fi
    local origin_url
    origin_url=$(git remote get-url origin 2>/dev/null || true)
    REPO_OWNER=$(echo "$origin_url" | sed -E 's/.*[:/]([^/]+)\/[^/]+(\.git)?$/\1/' || true)
    if [[ -z "$REPO_OWNER" ]]; then
        REPO_OWNER="bvarnai"
    fi
}
resolve_owner
REPO_NAME="${REPO_NAME:-git-brx-sandbox}"

usage() {
    cat <<EOF
Usage: $0 [COMMAND] [OPTIONS]

Commands:
  --init-repo                 Check or create private GitHub repository (uses ADMIN token)
  --delete-repo               Delete GitHub sandbox repository (uses ADMIN token)
  --invite-collaborator <usr> Add collaborator and auto-accept if ALEX token set
  --accept-invitations        Accept pending invitations using ALEX token
  --seed-issues               Seed initial test issues (#1..#4)
  --create-issue <title>      Create an ad-hoc issue on GitHub
      [--type bug|story]      Issue type / label (default: story)
      [--body <description>]  Optional issue description
  --help                      Show this help message
EOF
}

api_request_with_token() {
    local token="$1"
    local method="$2"
    local endpoint="$3"
    shift 3
    local data="${1:-}"

    if [[ -n "$data" ]]; then
        curl -s -w "\n%{http_code}" \
            -X "$method" \
            -H "Authorization: Bearer ${token}" \
            -H "Accept: application/vnd.github+json" \
            "https://api.github.com/${endpoint}" \
            -d "$data"
    else
        curl -s -w "\n%{http_code}" \
            -X "$method" \
            -H "Authorization: Bearer ${token}" \
            -H "Accept: application/vnd.github+json" \
            "https://api.github.com/${endpoint}"
    fi
}

admin_api_request() {
    if [[ -z "$ADMIN_GITHUB_TOKEN" ]]; then
        echo "Error: ADMIN_GITHUB_TOKEN is required for this operation." >&2
        exit 1
    fi
    api_request_with_token "$ADMIN_GITHUB_TOKEN" "$@"
}

alex_api_request() {
    if [[ -z "$ALEX_GITHUB_TOKEN" ]]; then
        echo "Error: ALEX_GITHUB_TOKEN is required for this operation." >&2
        exit 1
    fi
    api_request_with_token "$ALEX_GITHUB_TOKEN" "$@"
}

api_request() {
    local token="${ME_GITHUB_TOKEN:-${ADMIN_GITHUB_TOKEN:-}}"
    if [[ -z "$token" ]]; then
        echo "Error: GITHUB_TOKEN is not set." >&2
        exit 1
    fi
    api_request_with_token "$token" "$@"
}

init_repo() {
    echo "==> [GitHub Sandbox] Checking if repository ${REPO_OWNER}/${REPO_NAME} exists..."
    local status_code
    status_code=$(curl -s -o /dev/null -w "%{http_code}" \
        -H "Authorization: Bearer ${ADMIN_GITHUB_TOKEN:-${ME_GITHUB_TOKEN:-}}" \
        -H "Accept: application/vnd.github+json" \
        "https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}")

    if [[ "$status_code" == "200" ]]; then
        echo "    Repository ${REPO_OWNER}/${REPO_NAME} exists on GitHub."
        return 0
    elif [[ "$status_code" == "404" ]]; then
        echo "    Repository not found. Creating private repository ${REPO_OWNER}/${REPO_NAME}..."
        local resp
        resp=$(admin_api_request "POST" "user/repos" "{\"name\":\"${REPO_NAME}\",\"private\":true,\"description\":\"Dedicated sandbox for git-brx multi-developer collaboration testing\",\"auto_init\":true}")
        local code
        code=$(echo "$resp" | tail -n1)
        if [[ "$code" != "201" ]]; then
            echo "Error: Failed to create repository (HTTP $code):" >&2
            echo "$resp" | head -n -1 >&2
            echo "" >&2
            echo "Tip: Ensure ADMIN_GITHUB_TOKEN has repository creation permissions," >&2
            echo "or create 'git-brx-sandbox' manually at https://github.com/new and re-run." >&2
            exit 1
        fi
        echo "    Repository ${REPO_OWNER}/${REPO_NAME} created successfully."
        sleep 2
    else
        echo "Error: Unexpected response from GitHub API (HTTP $status_code)." >&2
        exit 1
    fi
}

delete_repo() {
    echo "==> [GitHub Sandbox] Deleting repository ${REPO_OWNER}/${REPO_NAME}..."
    local resp
    resp=$(admin_api_request "DELETE" "repos/${REPO_OWNER}/${REPO_NAME}")
    local code
    code=$(echo "$resp" | tail -n1)
    if [[ "$code" == "204" ]]; then
        echo "    Repository ${REPO_OWNER}/${REPO_NAME} successfully deleted."
    elif [[ "$code" == "404" ]]; then
        echo "    Repository does not exist on GitHub."
    else
        echo "Error: Failed to delete repository (HTTP $code):" >&2
        echo "$resp" | head -n -1 >&2
        exit 1
    fi
}

invite_collaborator() {
    local username="$1"
    local permission="${2:-push}"
    echo "==> [GitHub Sandbox] Adding collaborator '${username}' (permission: ${permission})..."
    local resp
    resp=$(admin_api_request "PUT" "repos/${REPO_OWNER}/${REPO_NAME}/collaborators/${username}" "{\"permission\":\"${permission}\"}")
    local code
    code=$(echo "$resp" | tail -n1)

    if [[ "$code" == "201" ]]; then
        echo "    Invitation sent to ${username}."
        local inv_id
        inv_id=$(echo "$resp" | head -n -1 | jq -r '.id // empty' 2>/dev/null || true)
        if [[ -n "${ALEX_GITHUB_TOKEN:-}" && "$username" == "alexvarnai" ]]; then
            accept_invitation "$inv_id"
        else
            echo "    Alex can accept invitation at: https://github.com/${REPO_OWNER}/${REPO_NAME}/invitations"
        fi
    elif [[ "$code" == "204" ]]; then
        echo "    User ${username} is already a collaborator."
    else
        echo "Warning: Could not add collaborator (HTTP $code):" >&2
        echo "$resp" | head -n -1 >&2
    fi
}

accept_invitation() {
    local direct_id="${1:-}"
    if [[ -z "${ALEX_GITHUB_TOKEN:-}" ]]; then
        echo "    (ALEX_GITHUB_TOKEN not provided, skipping automatic invitation acceptance)"
        return 0
    fi
    echo "==> [GitHub Sandbox] Accepting repository invitation for Alex Varnai..."
    local inv_id="$direct_id"
    if [[ -z "$inv_id" ]]; then
        local list_resp
        list_resp=$(alex_api_request "GET" "user/repository_invitations")
        inv_id=$(echo "$list_resp" | head -n -1 | jq -r --arg repo "${REPO_NAME}" '.[] | select(.repository.name == $repo) | .id' 2>/dev/null || true)
    fi

    if [[ -n "$inv_id" && "$inv_id" != "null" ]]; then
        local accept_resp
        accept_resp=$(alex_api_request "PATCH" "user/repository_invitations/${inv_id}")
        local code
        code=$(echo "$accept_resp" | tail -n1)
        if [[ "$code" == "204" ]]; then
            echo "    Invitation #${inv_id} successfully accepted by Alex Varnai!"
            return 0
        fi
    fi
    echo "    Alex can accept invitation manually at: https://github.com/${REPO_OWNER}/${REPO_NAME}/invitations"
}

create_issue() {
    local title="$1"
    local body="${2:-}"
    local issue_type="${3:-story}"

    local label="enhancement"
    if [[ "$issue_type" == "bug" ]]; then
        label="bug"
    fi

    echo "==> Creating issue: '${title}' (type: ${issue_type}, label: ${label})..."
    local payload
    payload=$(jq -n \
        --arg t "$title" \
        --arg b "$body" \
        --arg l "$label" \
        '{title: $t, body: $b, labels: [$l]}')

    local resp
    resp=$(api_request "POST" "repos/${REPO_OWNER}/${REPO_NAME}/issues" "$payload")
    local code
    code=$(echo "$resp" | tail -n1)

    if [[ "$code" != "201" ]]; then
        echo "Error: Failed to create issue (HTTP $code):" >&2
        echo "$resp" | head -n -1 >&2
        exit 1
    fi

    local issue_url issue_num
    issue_num=$(echo "$resp" | head -n -1 | jq -r '.number')
    issue_url=$(echo "$resp" | head -n -1 | jq -r '.html_url')
    echo "    Created Issue #${issue_num}: ${issue_url}"
}

seed_issues() {
    echo "==> [GitHub Sandbox] Checking for existing issues in ${REPO_OWNER}/${REPO_NAME}..."
    local list_resp
    list_resp=$(api_request "GET" "repos/${REPO_OWNER}/${REPO_NAME}/issues?state=all&per_page=1")
    local code
    code=$(echo "$list_resp" | tail -n1)

    if [[ "$code" != "200" ]]; then
        echo "Error: Failed to query repository issues (HTTP $code)." >&2
        exit 1
    fi

    local issue_count
    issue_count=$(echo "$list_resp" | head -n -1 | jq 'length')

    if [[ "$issue_count" -gt 0 ]]; then
        echo "    Repository already has issues present. Skipping seed."
        return 0
    fi

    echo "    No issues found. Seeding initial collaborative scenario issues (#1..#4)..."
    create_issue "feat: Add subtract calculation function" \
        "Implement a Subtract function in calc.go for basic arithmetic calculations." "story"

    create_issue "feat: Add divide calculation function" \
        "Implement a Divide function in calc.go handling zero-division safety." "story"

    create_issue "fix: Resolve config retry timeout calculation" \
        "Fix retry timeout multiplier in config.json to prevent infinite retry loops." "bug"

    create_issue "feat: Implement shared pipeline stages" \
        "Add multi-step pipeline processing for calculation batch tasks." "story"

    echo "==> Seeding complete. Issues #1, #2, #3, and #4 created."
}

# CLI Argument Parsing
if [[ $# -eq 0 ]]; then
    usage
    exit 0
fi

while [[ $# -gt 0 ]]; do
    case "$1" in
        --init-repo)
            init_repo
            shift
            ;;
        --delete-repo)
            delete_repo
            shift
            ;;
        --invite-collaborator)
            if [[ $# -lt 2 ]]; then
                echo "Error: --invite-collaborator requires a username" >&2
                exit 1
            fi
            INVITE_USER="$2"
            INVITE_PERM="push"
            shift 2
            if [[ $# -gt 1 && "$1" == "--permission" ]]; then
                INVITE_PERM="$2"
                shift 2
            fi
            invite_collaborator "$INVITE_USER" "$INVITE_PERM"
            ;;
        --accept-invitations)
            accept_invitation
            shift
            ;;
        --seed-issues)
            seed_issues
            shift
            ;;
        --create-issue)
            if [[ $# -lt 2 ]]; then
                echo "Error: --create-issue requires a title argument" >&2
                exit 1
            fi
            TITLE="$2"
            TYPE="story"
            BODY=""
            shift 2
            while [[ $# -gt 0 ]]; do
                case "$1" in
                    --type)
                        TYPE="$2"
                        shift 2
                        ;;
                    --body)
                        BODY="$2"
                        shift 2
                        ;;
                    *)
                        break
                        ;;
                esac
            done
            create_issue "$TITLE" "$BODY" "$TYPE"
            ;;
        --help|-h)
            usage
            exit 0
            ;;
        *)
            echo "Unknown option: $1" >&2
            usage >&2
            exit 1
            ;;
    esac
done
