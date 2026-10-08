#!/bin/bash

# import common package
source "${BRANCH_HOME}/common.sh"

# help hook
function help()
{
  branch::common::show_help_and_exit "Resolve conflicts using a mergetool and continue with rebase/merge if necessary" "resolve";
}

function main()
{
  # read repository status
  branch::common::read_repository

  # process arguments
  branch::common::no_args "$@"

  # preconditions
  # check for ongoing merge/rebase
  if [[ -z "${GIT_PROMPT_REBASE}" && -z "${GIT_PROMPT_MERGING}" ]]; then
    branch::common::log "No merge/rebase (sync) is ongoing"
    exit 0
  fi

  # run merge conflict resolution tools to resolve merge conflicts
  git mergetool --no-prompt
  # add or stage changes
  git add .

  if [[ -n "$GIT_PROMPT_MERGING" ]]; then
    # merge flow
    branch::common::log "You are middle of a merge (sync), continuing"
    # commit staged changes (closes merge)
    if ! git diff --cached --exit-code; then
      local line
      line=$(head -n 1 .git/MERGE_MSG)
      if git commit -m "${line}"; then
        branch::common::log "Merge (sync) completed"
      fi
    else
      if git merge --continue; then
        branch::common::log "Merge (sync) completed"
      fi
    fi
  elif [[ -n "$GIT_PROMPT_REBASE" ]]; then
    # rebase flow (note rebase is commit by commit)
    branch::common::log "You are middle of a rebase (sync), continuing"
    # we need to loop here until all commits are rebased ???
    while ! git rebase --continue; do
      # run merge conflict resolution tools to resolve merge conflicts
      git mergetool --no-prompt
      # add or stage changes
      git add .
    done
    branch::common::log "Rebase (sync) completed"
  fi
}

main "$@"
