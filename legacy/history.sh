#!/bin/bash

 # import common package
source "${BRANCH_HOME}/common.sh"

# help hook
function help()
{
  branch::common::show_help_and_exit "Display a simplified commit graph" "history";
}

function main()
{
  # process arguments
  branch::common::no_args "$@"

  # display log
  if ! git log --pretty=format:"%h %ad | %s%d [%an]" --graph --decorate --date=short; then
    branch::common::err "Getting history failed (gitish: 'git log --pretty=format:\"%h %ad | %s%d [%an]\" --graph --decorate --date=short' failed)"
    exit 1
  fi
}

main "$@"
