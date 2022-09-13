#!/usr/bin/env sh
# Wrap goreleaser by creating a temporary git tag from the version.txt file.
set -e
hash goreleaser
if [ ! -r version.txt ] ; then
  echo "This $0 script expects to be run from within a sub-directory of an Insights plugin, which should contain a version.txt file."
  exit 1
fi
temporary_git_tag=$(cat version.txt)
echo "$(basename $0) creating local tag ${temporary_git_tag} for goreleaser"
# The -f is included to overwrite existing tags, perhaps from previous CI jobs.
git tag -f -m "temporary local tag for goreleaser" ${temporary_git_tag}
export GORELEASER_CURRENT_TAG=${temporary_git_tag}
export skip_main_docker_tags=true
if [ "${CIRCLE_BRANCH}" == "main" ] ; then
  echo 'Setting skip_main_docker_tags to false because this is the main branch'
export skip_main_docker_tags=false
  fi
hash envsubst
cat .goreleaser.yml.envsubst |envsubst >.goreleaser.yml
goreleaser $@
if [ $? -eq 0 ] ; then
  rm .goreleaser.yml # Keep git clean
fi
unset GORELEASER_CURRENT_TAG
git tag -d ${temporary_git_tag}
