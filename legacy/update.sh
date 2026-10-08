#!/bin/bash

# import common package
source "${BRANCH_HOME}/common.sh"

# help hook
function help()
{
  branch::common::show_help_and_exit "Update the local branch using rebase" "update";
}

function main()
{
  # read repository status
  branch::common::read_repository

  # process arguments
  branch::common::no_args "$@"

  # preconditions
  branch::common::precondition_origin
  branch::common::precondition_detached

  # pull changes
  if ! git pull --rebase origin; then
     branch::common::err "Update failed (gitish: 'git pull --rebase origin' failed)"
    exit 1
  fi
}

main "$@"
