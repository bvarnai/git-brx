#!/bin/bash

# import common packages
source "${ENV_ROOT}/env.sh"
source "${BRANCH_HOME}/common.sh"

# help hook
function help()
{
  branch::common::show_help_and_exit  "Create a new development branch" "create";
}

function main()
{
  # process arguments
  local params
  local offline
  params=""
  while (( "$#" )); do
    case "$1" in
      help) # print usage
        help
        ;;
      -o|--offline)
        offline=true
        shift
        ;;
      --) # end argument parsing
        shift
        break
        ;;
      -*|--*=) # unsupported flags
        branch::common::err "Unsupported option '$1'"
        exit 1
        ;;
      *) # preserve positional arguments
        params="${params} $1"
        shift
        ;;
    esac
  done
  # set positional arguments in their proper place
  eval set -- "${params}"

  # preconditions
  # check if branch name argument is specified
  if [[ -z "$1" ]]; then
    branch::common::err "No branch name specified"
    exit 1
  fi

  # check if branch is already avaiable on local
  branch::common::log "Checking for existing branches (local)"
  local local_branch
  local_branch=$(git for-each-ref --format='%(refname:short)' refs/heads/"$1")
  if [[ -z "${local_branch}" ]]; then
    branch::common::log "No branch '$1' found (local)"
  else
    branch::common::log "Branch '${local_branch}' found (local)"
    branch::common::log "Hint: To select that branch, use \"git branch-select ${local_branch}\" instead"
    exit 0
  fi

  # check if branch is already avaiable on remote
  if [[ -z "${offline}" ]]; then
    # check if remote already contains this branch
    branch::common::log "Checking for existing branches (remote)"
    # git ls-remote is one unique command allowing you to query a remote repo without having to clone/fetch it first
    local remote_branch
    remote_branch=$(git ls-remote origin "$1")
    # check return value
    if [[ $? != 0 ]]; then
      branch::common::err "Unable to reach remote"
      branch::common::log "Hint: Try --offline if working without network access"
      exit 1
    fi

    if [[ -z "${remote_branch}" ]]; then
      branch::common::log "No branch '$1' found (remote)"
    else
      branch::common::log "Branch '$1' found (remote)"
      branch::common::log "To select this branch, use \"git branch-select ${1}\" instead"
      exit 0
    fi
  else
    branch::common::log "Offline mode selected (git remote checks skipped...)"
  fi

  # get user credetials
  branch::common::get_credentials

  # call groovy script
  branch::common::log "Launching groovy environment"
  local groovyClasspath
  if [[ $(uname -s) == "Linux" ]]; then
    groovyClasspath="${BRANCH_HOME}"
  else
    groovyClasspath="$(cygpath -u $BRANCH_HOME)"
  fi
  groovy -cp "${groovyClasspath}" "${BRANCH_HOME}/create.groovy" "$1" "${offline}" "${TOOLS_CREDENTIALS}" "${BRANCH_CONFIGURATION_PATH}"
}

main "$@"
