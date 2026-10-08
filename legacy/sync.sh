#!/bin/bash

# import common package
source "${BRANCH_HOME}/common.sh"

# help hook
function help()
{
  branch::common::show_help_and_exit "Sync changes from a branch ('master' by default)" "sync";
}

function main()
{
  # read repository status
  branch::common::read_repository

  # not allowed on shallow repository
  branch::common::precondition_shallow

  # pre-check branch types
  local branch_type
  branch_type=$(cut -d'/' -f1 <<<"${GIT_PROMPT_BRANCH}")

  local rebase
  local merge
  rebase=0
  merge=0
  # select default merge strategy based on branch type
  if [[ "${branch_type}" == "issue" ]]; then
    branch::common::log "Sync strategy 'rebase' selected for branch type '${branch_type}'"
    rebase=1
  elif [[ "${branch_type}" == "feature" || "${branch_type}" == "epic" ]]; then
    branch::common::log "Sync strategy 'merge' selected for branch type '${branch_type}'"
    merge=1
  else
    branch::common::err "You must be on 'issue', 'feature' or 'epic' branch"
    exit 1  
  fi

  # process arguments
  local params
  local autostash
  autostash=0
  local interactive
  interactive=0
  local force_merge
  force_merge=0
  local force_rebase
  force_rebase=0
  params=
  while (( "$#" )); do
    case "$1" in
      help) # print usage
        help
        ;;
      -a|--autostash)
        autostash=1
        shift
        ;;
       -i|--interactive)
        interactive=1
        shift
        ;;
      -m|--merge)
        force_merge=1
        shift
        ;;
      -r|--rebase)
        force_rebase=1
        shift
        ;;
      --) # end argument parsing
        shift
        break
        ;;
      -*|--*=) # unsupported options
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

  # overrides
  if [[ ${force_merge} == 1 && ${force_rebase} == 1 ]]; then
    branch::common::err "Multiple override sync strategies specified"
    exit 1
  fi

  if [[ ${rebase} == 1 && ${force_merge} == 1 ]]; then
    branch::common::log "Overriding sync strategy 'rebase' with 'merge'"
    rebase=0
    merge=1
  fi

  if [[ ${merge} == 1 && ${force_rebase} == 1 ]]; then
    branch::common::log "Overriding sync strategy 'merge' with 'rebase'"
    rebase=1
    merge=0
  fi

  # check rebase options
  if [[ ${merge} == 1 ]]; then
    if [[ ${autostash} == 1 || ${interactive} == 1 ]]; then
      branch::common::err "Options 'autostash/interactive' are rebase only"
      exit 1
    fi
  fi

  # we should be in a clean state, check for ongoing rebase or merge
  if [[ -n "${GIT_PROMPT_REBASE}" || -n "${GIT_PROMPT_MERGING}" ]]; then
    branch::common::err "You are in a middle of a rebase/merge"
    exit 1
  fi

  # check if branch name is specified, fallback to default
  local with_branch
  if [[ -z "$1" ]]; then
    with_branch="master"
    branch::common::log "Syncing '${with_branch}' branch by default"
  else
    with_branch="$1"
  fi

  # fetch all updates from remote
  if ! git fetch --all; then
    branch::common::err "Sync failed (gitish: 'git fetch --all' failed)"
    exit 1
  fi

  # check if branch to sync is up-to-date, fast-forward will be enforced
  branch::common::branch_delta "${with_branch}"
  branch::common::log "Branch '${BRANCH_DELTA_LOCAL}' (ahead ${BRANCH_DELTA_LEFT_AHEAD}) | (behind ${BRANCH_DELTA_RIGHT_AHEAD}) '${BRANCH_DELTA_REMOTE}'"
  # if any differences found, just prevent sync
  if [[ "${BRANCH_DELTA_LEFT_AHEAD}" != 0 || "${BRANCH_DELTA_RIGHT_AHEAD}" != 0 ]]; then
    branch::common::err "Sync branch '${with_branch}' is not up-to-date"
    branch::common::log "Hint: Switch to '${with_branch}' branch and use \"git branch-update\" to update all changes"
    branch::common::err "Sync failed"
    exit 1
  fi

  if [[ ${rebase} == 1 ]]; then
    # rebase merge strategy
    # add rebase options
    if [[ ${autostash} == 1 ]]; then
      rebase_options="--autostash"
    fi
    if [[ ${interactive} == 1 ]]; then
      rebase_options="$rebase_options --interactive"
    fi
    if [[ -z "${rebase_options}" ]]; then
      # rebase with options
      if ! git rebase "${with_branch}"; then
        branch::common::err "Sync failed (gitish: 'git rebase ${with_branch}' failed)"
        branch::common::log "Hint: Use \"git branch-resolve\" in case of merge conflicts"
        exit 1
      fi
    else
      # rebase
      if ! git rebase "${rebase_options}" "${with_branch}"; then
        branch::common::err "Sync failed (gitish: 'git rebase ${rebase_options} ${with_branch}' failed)"
        branch::common::log "Hint: Use \"git branch-resolve\" in case of merge conflicts"
        exit 1
      fi
    fi
  elif [[ ${merge} == 1 ]]; then
    # commit with message saved in .git/MERGE_MSG
    if ! git merge -m "Merge branch ${with_branch} into ${GIT_PROMPT_BRANCH}" "${with_branch}"; then
        branch::common::err "Sync failed (gitish: 'git merge -m \"Merge branch '${with_branch}' into ${GIT_PROMPT_BRANCH}\" ${with_branch}' failed)"
        branch::common::log "Hint: Use \"git branch-resolve\" in case of merge conflicts"
        exit 1
    fi
  fi
}

main "$@"
