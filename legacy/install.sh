#!/bin/bash

echo "Installing branch command aliases"
# Remove commands (do not delete entires from here to always get a clean state)
git config --unset alias.branch-name
git config --unset alias.branch-history
git config --unset alias.branch-update
git config --unset alias.branch-select
git config --unset alias.branch-sync
git config --unset alias.branch-reset
git config --unset alias.branch-resolve
git config --unset alias.branch-create
git config --unset alias.branch-publish
git config --unset alias.branch-delete
git config --unset alias.branch-pr
git config --unset alias.branch-review
git config --unset alias.branch-help

# Install commands (don't forget to update config-example)
echo "Installed branch-name"
git config --add alias.branch-name "!f() { ( \$BRANCH_HOME/name.sh \$@ ); }; f"
echo "Installed branch-history"
git config --add alias.branch-history "!f() { ( \$BRANCH_HOME/history.sh \$@ ); }; f"
echo "Installed branch-update"
git config --add alias.branch-update "!f() { ( \$BRANCH_HOME/update.sh \$@ ); }; f"
echo "Installed branch-select"
git config --add alias.branch-select "!f() { ( \$BRANCH_HOME/select.sh \$@ ); }; f"
echo "Installed branch-sync"
git config --add alias.branch-sync "!f() { ( \$BRANCH_HOME/sync.sh \$@ ); }; f"
echo "Installed branch-reset"
git config --add alias.branch-reset "!f() { ( \$BRANCH_HOME/reset.sh \$@ ); }; f"
echo "Installed branch-resolve"
git config --add alias.branch-resolve "!f() { ( \$BRANCH_HOME/resolve.sh \$@ ); }; f"
echo "Installed branch-create"
git config --add alias.branch-create "!f() { ( \$BRANCH_HOME/create.sh \$@ ); }; f"
echo "Installed branch-publish"
git config --add alias.branch-publish "!f() { ( \$BRANCH_HOME/publish.sh \$@  ); }; f"
echo "Installed branch-delete"
git config --add alias.branch-delete "!f() { ( \$BRANCH_HOME/delete.sh \$@ ); }; f"
echo "Installed branch-review"
git config --add alias.branch-review "!f() { ( \$BRANCH_HOME/review.sh \$@  ); }; f"
echo "Installed branch-help"
git config --add alias.branch-help "!f() { ( \$BRANCH_HOME/help.sh \$@  ); }; f"
