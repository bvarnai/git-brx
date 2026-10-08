#!/bin/bash

# import common package
source "${BRANCH_HOME}/common.sh"

# help hook
function help()
{
  branch::common::show_help_and_exit "Select and switch to an existing branch ('master' by default)" "select";
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
  local branch
  if [[ -z "$1" ]]; then
    branch="master"
    branch::common::log "Selecting '${branch}' branch by default"
  else
    branch="$1"
  fi

  # get all changes
  if ! git fetch --all; then
    branch::common::err "Select failed (gitish: 'git fetch --all' failed)"
    exit 1
  fi

  # checkout branch
  if ! git checkout "${branch}"; then
    branch::common::err "Select failed (gitish: 'git checkout ${branch}' failed)"
    exit 1
  fi

}

main "$@"
