#!/bin/bash

# import common package
source "${BRANCH_HOME}/common.sh"

# help hook
function help()
{
  branch::common::show_help_and_exit "Discard the current working changes, restoring the latest state from origin" "reset";
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

  # cancel any onging merge/rebase
  if [[ -n "$GIT_PROMPT_MERGING" ]]; then
    branch::common::log "You are middle of a merge (sync), aborting"
    if ! git merge --abort; then
      branch::common::err "Reset failed (gitish: 'git merge --abort' failed)"
      exit 1
    fi
  fi

  if [[ -n "$GIT_PROMPT_REBASE" ]]; then
    branch::common::log "You are middle of a rebase (sync), aborting"
    if ! git rebase --abort; then
      branch::common::err "Reset failed (gitish: 'git rebase --abort' failed)"
      exit 1
    fi
  fi

  # reset branch
  local BRANCH_NAME
  BRANCH_NAME=$(git branch-name)
  
  # check return value
  if ! git reset --hard origin/"${BRANCH_NAME}"; then
    branch::common::err "Reset failed (gitish: 'git reset --hard origin/${BRANCH_NAME}' failed)"
    branch::common::log "Hint: Your branch may have not been published yet. Use \"git branch-publish\" to publish a branch"
    exit 1
  fi
}

main "$@"
