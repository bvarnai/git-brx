#!/bin/bash

# import common package
source "${BRANCH_HOME}/common.sh"

# help hook
function help()
{
  branch::common::show_help_and_exit "Upload changes to remote" "publish";
}

function main()
{
  # read repository status
  branch::common::read_repository

  # not allowed on shallow repository
  branch::common::precondition_shallow

  # process arguments
  branch::common::no_args "$@"

  # push changes
  local tracking
  tracking=$(git for-each-ref --format='%(upstream:short)' "$(git symbolic-ref -q HEAD)")
  if [[ "${tracking}" == "origin/${GIT_PROMPT_BRANCH}" ]]; then
    if ! git push origin "${GIT_PROMPT_BRANCH}" --force-with-lease; then
      branch::common::err "Publish failed (gitish: 'git push origin ${GIT_PROMPT_BRANCH} --force-with-lease' failed)"
      exit 1
    fi
  else
    # set upstream if necessary
    if ! git push --set-upstream origin "${GIT_PROMPT_BRANCH}" --force-with-lease; then
      branch::common::err "Publish failed (gitish: 'git push --set-upstream origin ${GIT_PROMPT_BRANCH} --force-with-lease' failed)"
      exit 1
    fi
  fi
}

main "$@"
