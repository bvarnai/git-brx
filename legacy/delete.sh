#!/bin/bash

 # import common package
source "${BRANCH_HOME}/common.sh"

# help hook
function help()
{
  branch::common::show_help_and_exit "Delete the current branch" "delete";
}

function main()
{
  # read repository status
  branch::common::read_repository

  # process arguments
  branch::common::no_args "$@"

  # preconditions
  branch::common::precondition_origin

  # allow only branch types 'issue' and 'feature'
  local branch_type
  branch_type=$(cut -d'/' -f1 <<<"${GIT_PROMPT_BRANCH}")
  if [[ "${branch_type}" != "feature" ]] && [[ "${branch_type}" != "issue" ]] && [[ "${branch_type}" != "epic" ]]; then
      branch::common::err "You must be on 'issue', 'feature' or 'epic' branch"
      exit 1
  fi

  branch::common::log "Checking for existing branches (remote)"
  local remote_branch
  if ! remote_branch=$(git ls-remote origin "${GIT_PROMPT_BRANCH}"); then
    branch::common::err "Unable to reach remote, are you offline? (gitish: 'git ls-remote origin -D ${GIT_PROMPT_BRANCH}' failed)"
    exit 1
  fi

  if [[ -z "${remote_branch}" ]]; then
    branch::common::log "No branch '${GIT_PROMPT_BRANCH}' found (remote)"
  else
    branch::common::log "Branch '${GIT_PROMPT_BRANCH}' found (remote)"
    branch::common::err "Branch must be deleted on remote first"
    exit 1
  fi

  # switch to master first
  branch::common::log "Switching to 'master' branch"
  if ! git branch-select master; then
    branch::common::err "Unable to switch to 'master' branch (gitish: 'git branch-select master' failed)"
  fi

  if ! git branch -D "${GIT_PROMPT_BRANCH}"; then
    branch::common::err "Delete failed (gitish: 'git branch -D ${GIT_PROMPT_BRANCH}' failed)"
    exit 1
  fi

  if ! git remote prune origin; then
    branch::common::err "Prune failed (gitish: 'git remote prune origin' failed)"
    exit 1
  fi
  branch::common::log "Hint: Your branch may not been up-to-date. Use \"git branch-update\" to update a branch"
}

main "$@"
