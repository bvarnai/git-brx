#!/bin/bash

# import common packages
source "${ENV_ROOT}/env.sh"
source "${BRANCH_HOME}/common.sh"

# help hook
function help()
{
  branch::common::show_help_and_exit "Create a review request between branches (to 'master' by default)" "review";
}

function main()
{
  # read repository status
  branch::common::read_repository

    # process arguments
  local params
  params=""
  while (( "$#" )); do
    case "$1" in
      help) # print usage
        help
        ;;
      --) # end argument parsing
        shift
        break
        ;;
      -*|--*=) # unsupported options
        log "Unsupported option '$1'"
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

  # check if branch name is specified, fallback to default
  local target_branch
  if [[ -z "$1" ]]; then
    target_branch="master"
    branch::common::log "Selecting '${target_branch}' target branch by default"
  else
    target_branch="$1"
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
  groovy -cp "${groovyClasspath}" "${BRANCH_HOME}/review.groovy" "${GIT_PROMPT_BRANCH}" "${TOOLS_CREDENTIALS}" "${BRANCH_CONFIGURATION_PATH}" "${target_branch}"
}

main "$@"
