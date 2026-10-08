#!/bin/bash

# References, acknowledgements
# 1. https://google.github.io/styleguide/shell.xml
# 2. https://github.com/git/git/blob/master/contrib/completion/git-prompt.sh
# 3. https://medium.com/@Drew_Stokes/bash-argument-parsing-54f3b81a6a8f
# 4. https://gist.github.com/cjus/1047794

# Constants
declare -r BRANCH_DOC_URL="http://ies-iesd-conf.ies.mentorg.com:8090/x/uxObCQ#GitwithBitbucketServerUser'sGuide(VSB)"
declare -r LOG_PREFIX="[branch]"
declare -r TOOLS_CREDENTIALS_ADDRESS="https://tools@ies-vsb-ub.ies.mentorg.com"

# Project
PROJECT=$(basename "$(pwd)" | cut -d _ -f 1)
BRANCH_CONFIGURATION_PATH="${ENV_ROOT}/../conf/${PROJECT}"

source "${BRANCH_HOME}/git-prompt.sh"

#######################################
# Read repository information using git-prompt.sh.
# Globals:
#  GIT_PROMPT - stdout of the _git_ps1
#  GIT_PROMPT_BRANCH - branch name
#  GIT_PROMPT_REBASE - '1' if middle of rebase
#  GIT_PROMPT_MERGING - '1' if middle of merge
#  GIT_PROMPT_STEP - step count if middle of rebase
#  GIT_PROMPT_TOTAL - total steps if middle of rebase
#  GIT_PROMPT_DETACHED - '1' if we are in detached HEAD state
# Arguments:
#   None
#######################################
function branch::common::read_repository()
{
  # read repository information
  GIT_PROMPT=
  GIT_PROMPT_BRANCH=
  GIT_PROMPT_REBASE=
  GIT_PROMPT_MERGING=
  GIT_PROMPT_STEP=
  GIT_PROMPT_TOTAL=
  GIT_PROMPT_DETACHED=

  __git_ps1

  # sanity check
  if [[ -z "${GIT_PROMPT}" ]]; then
    branch::common::err "Awh! This is not a git repository"
    exit 1
  fi
}

#######################################
# Check upstream branch delta status in ahead/behind style.
# Based on https://stackoverflow.com/questions/7773939/show-git-ahead-and-behind-info-for-all-branches-including-remotes/20499690#20499690
# Globals:
#  BRANCH_DELTA_LOCAL - local branch name
#  BRANCH_DELTA_LEFT_AHEAD - number of commits the local branch is ahead of remote
#  BRANCH_DELTA_REMOTE - remote branch name
#  BRANCH_DELTA_RIGHT_AHEAD - number of commits the local branch is behind of remote
# Arguments:
#  $1 - branch to check
#######################################
function branch::common::branch_delta()
{
  BRANCH_DELTA_LOCAL="$1"
  BRANCH_DELTA_LEFT_AHEAD=0
  BRANCH_DELTA_REMOTE=
  BRANCH_DELTA_RIGHT_AHEAD=0

  git for-each-ref --format="%(refname:short) %(upstream:short)" refs/heads | \
  while read -r local_branch remote_branch
  do
    [ -z "${remote_branch}" ] && continue
    git rev-list --left-right "${local_branch}...${remote_branch}" -- 2>/dev/null >/tmp/git_upstream_status_delta || continue
    LEFT_AHEAD=$(grep -c '^<' /tmp/git_upstream_status_delta)
    RIGHT_AHEAD=$(grep -c '^>' /tmp/git_upstream_status_delta)
    if [[ "${local_branch}" == "$1" ]]; then
      #log "$local_branch (ahead $LEFT_AHEAD) | (behind $RIGHT_AHEAD) $remote_branch"
      echo "${local_branch}|${LEFT_AHEAD}|${remote_branch}|${RIGHT_AHEAD}" 2>/dev/null >/tmp/branch_delta
      break
    fi
  done

  if [[ -f /tmp/branch_delta ]]; then
    # the command above runs in a sub-shell, variables passed in tmp files
    BRANCH_DELTA_LOCAL=$(cut -d'|' -f1 /tmp/branch_delta)
    BRANCH_DELTA_LEFT_AHEAD=$(cut -d'|' -f2 /tmp/branch_delta)
    BRANCH_DELTA_REMOTE=$(cut -d'|' -f3 /tmp/branch_delta)
    BRANCH_DELTA_RIGHT_AHEAD=$(cut -d'|' -f4 /tmp/branch_delta)

    # delete tmp file
    rm /tmp/branch_delta 2>/dev/null
  fi
}

#######################################
# Logs a message to stdout.
# Arguments:
#   $@ - anything to log
# Returns:
#   None
#######################################
function branch::common::log()
{
  echo "${LOG_PREFIX} $*"
}

#######################################
# Logs a message to stderr.
# Arguments:
#   $@ - anything to log
# Returns:
#   None
#######################################
function branch::common::err()
{
  if [[ -z "$VSB_CI" ]]; then
    echo -e "${LOG_PREFIX} \e[31m! $*\e[0m" >&2
  else
    echo -e "${LOG_PREFIX} ! $*" >&2
  fi
}

#######################################
# Shows help message with link to the command reference, than exits.
# Arguments:
#  $1 - help messsage
#  $2 - command name to anchor the reference documentation
# Returns:
#  Funtion doesn't return
#######################################
function branch::common::show_help_and_exit()
{
  branch::common::log "$1"; \
  branch::common::log "Documentation <${BRANCH_DOC_URL}-$2>" 1>&2; exit 1;
}

#######################################
# Helper function for command with no arguments just help.
# Arguments:
#  $@ - all arguments
# Returns:
#  None
#######################################
function branch::common::no_args()
{
  # parse command parameters
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

  # no arguments are expected, just discard them
  if [[ "$#" != 0 ]]; then
    branch::common::log "This command takes no argument(s), input '$*' is discarded"
  fi
}

#######################################
# Checks if origin is set.
# Arguments:
#  None
# Returns:
#  Exits with return value 1 if no origin
#######################################
function branch::common::precondition_origin()
{
  local result
  result=$(git remote get-url origin)
  if [[ -z "${result}" ]]; then
    branch::common::err "Your repository has no remote origin (gitish: 'git remote get-url origin' failed)"
    exit 1
  fi
}

#######################################
# Checks if in detached HEAD state.
# Arguments:
#  None
# Returns:
#  Exits with return value 1 if detached HEAD state detected
#######################################
function branch::common::precondition_detached()
{
  if [[ "${GIT_PROMPT_DETACHED}" == "1" ]]; then
    branch::common::err "You are in detached HEAD state, this means you are not on any branch"
    branch::common::log "Hint: Select a branch first. Use \"git branch-select\" to select a branch"
    exit 1
  fi
}

#######################################
# Checks if repository is shallow.
# Arguments:
#  None
# Returns:
#  Exits with return value 1 if shallow
#######################################
function branch::common::precondition_shallow()
{
  if $(git rev-parse --is-shallow-repository); then
    branch::common::err "You are in a shallow repository, this means some git commands might not work properly"
    branch::common::log "Hint: Make a full clone of the repository and try again"
    exit 1
  fi
}

#######################################
# Read stored creadentials if not found as for user input.
# Globals:
#  TOOLS_CREDENTIALS - credentials (base64 encoded)
# Arguments:
#  None
# Returns:
#  Exits with return value 1 if credentials cannot be saved
function branch::common::get_credentials()
{
  if ! TOOLS_CREDENTIALS=$(WinCreds get --base64 "${TOOLS_CREDENTIALS_ADDRESS}"); then
    branch::common::log "Authentication required. Sign in to your MGC account"

    # get user credentials from stdin
    echo -n Username:
    read -r username
    echo -n Password:
    read -r -s password
    echo ""

    local save
    save=0
    while true; do
      read -r -p "Do you wish to save your credentials? (y/n)" yn
      case $yn in
          [Yy]* ) save=1; break;;
          [Nn]* ) break;;
          * ) echo "Please answer Yy or Nn.";;
      esac
    done
    if [[ ${save} == 1 ]]; then
      if ! WinCreds set "${TOOLS_CREDENTIALS_ADDRESS}" "${username}" "${password}"; then
        branch::common::err "Unable to access credentials storage"
        exit 1
      fi
      # read in base64 format
      TOOLS_CREDENTIALS=$(WinCreds get --base64 "${TOOLS_CREDENTIALS_ADDRESS}")
    else
      # just create the base64 formatted credentials without storing it
      TOOLS_CREDENTIALS=$(echo -n "${username}:${password}" | base64)
    fi
  else
    branch::common::log "Authenticating using stored credentials"
  fi
}
