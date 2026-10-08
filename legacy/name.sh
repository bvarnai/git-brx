#!/bin/bash

# import common package
source "${BRANCH_HOME}/common.sh"

# help hook
function help()
{
  branch::common::show_help_and_exit "Get the current (local) branch name" "name";
}

function main()
{
  # read repository status
  branch::common::read_repository

  # process arguments
  branch::common::no_args "$@"

  # diplay branch name
  echo "${GIT_PROMPT_BRANCH}"
}

main "$@"
