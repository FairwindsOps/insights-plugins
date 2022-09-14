#!/usr/bin/env sh
# Wrap goreleaser by creating a temporary git tag from the version.txt file.
set -e
this_script="$(basename $0)"
hash envsubst
hash goreleaser
if [ ! -r version.txt ] ; then
  echo "This ${this_script} script expects to be run from within a sub-directory of an Insights plugin, which should contain a version.txt file."
  exit 1
fi
temporary_git_tag=$(cat version.txt)
echo "${this_script} creating git tag ${temporary_git_tag} for goreleaser"
# The -f is included to overwrite existing tags, perhaps from previous CI jobs.
git tag -f -m "temporary local tag for goreleaser" ${temporary_git_tag}
export GORELEASER_CURRENT_TAG=${temporary_git_tag}
export skip_main_docker_tags=true
if [ "${CIRCLE_BRANCH}" == "main" ] ; then
  echo "${this_script} setting skip_main_docker_tags to false because this is the main branch"
export skip_main_docker_tags=false
  fi
cat .goreleaser.yml.envsubst |envsubst >.goreleaser.yml
goreleaser $@
if [ $? -eq 0 ] ; then
  echo "${this_script} removing the temporary .goreleaser.yml since goreleaser was successful"
  rm .goreleaser.yml # Keep git clean for additional goreleaser runs
fi
echo "${this_script} deleting git tag ${temporary_git_tag} for goreleaser"
unset GORELEASER_CURRENT_TAG
git tag -d ${temporary_git_tag}
