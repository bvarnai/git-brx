#!/bin/bash

 # import common package
source "${BRANCH_HOME}/common.sh"

# help hook
function help()
{
  branch::common::show_help_and_exit "Get the generic help and command list" "help";
}

function main()
{
  # read repository status
  branch::common::read_repository

  # process arguments
  branch::common::no_args "$@"

  # print usage
  echo "Usage: git branch-<command> [options]... [arguments]..."
  echo "branch automates parts of the development workflow. branch isn't meant to replace Git, only to make it easier to work with Git in our development context."
  echo ""
  echo "Available commands:"
  echo " branch-name"
  echo " branch-history"
  echo " branch-update"
  echo " branch-select"
  echo " branch-sync"
  echo " branch-reset"
  echo " branch-resolve"
  echo " branch-create"
  echo " branch-publish"
  echo " branch-delete"
  echo " branch-review"
  echo " branch-help * this command"
  echo ""
  echo "For command specific help use"
  echo " git branch-<command> help"
  echo ""
  echo "Full documentation <http://ies-iesd-conf.ies.mentorg.com:8090/x/uxObCQ>"
}

main "$@"
