#!/usr/bin/env sh
# Wrap goreleaser by using envsubst on .goreleaser.yml,
# and creating a temporary git tag from an Insights plugin the version.txt file.

cleanup() {
  if [ "${CIRCLE_TAG}" == "" ] ; then
    echo "${this_script} deleting git tag ${temporary_git_tag} for goreleaser"
    unset GORELEASER_CURRENT_TAG
    git tag -d ${temporary_git_tag}
  fi
}
set -eE # errexit and errtrace
trap 'cleanup' ERR

this_script="$(basename $0)"
if [ "${CIRCLE_BRANCH}" == "" ] ; then
  echo "${this_script} requires the CIRCLE_BRANCH environment variable, which is not set"
  exit 1
fi

hash envsubst
hash goreleaser
echo "${this_script} will run goreleaser for $(basename $(pwd))"
if [ "${TMPDIR}" == "" ] ; then
  export TMPDIR="/tmp"
  echo "${this_script} temporarily set the TMPDIR environment variable to ${TMPDIR}, used by some .goreleaser.yml files"
fi
if [ ! -r version.txt ] ; then
  echo "This ${this_script} script expects to be run from within a sub-directory of an Insights plugin, which should contain a version.txt file."
  exit 1
fi
if [ "$(git config user.email)" == "" ] ; then
  # git will use this env var as its user.email.
  # git tag -m is used in case tags are manually pushed by accident,
  # however git tag -m requires an email.
  export EMAIL='goreleaser_ci@fairwinds.com'
  echo "${this_script} using ${EMAIL} temporarily as the git user.email"
fi

temporary_git_tag=$(cat version.txt)
echo "${this_script} creating git tag ${temporary_git_tag} for goreleaser"
# The -f is included to overwrite existing tags, perhaps from previous CI jobs.
git tag -f -m "temporary local tag for goreleaser" ${temporary_git_tag}
export GORELEASER_CURRENT_TAG=${temporary_git_tag}
export skip_feature_docker_tags=false
export skip_main_docker_tags=true
if [ "${CIRCLE_BRANCH}" == "main" ] ; then
  echo "${this_script} setting skip_main_docker_tags to false, and skip_feature_docker_tags to true,  because this is the main branch"
  export skip_feature_docker_tags=true
  export skip_main_docker_tags=false
else
  # Use an adjusted git branch name as an additional docker tag, for feature branches.
  export feature_docker_tag=$(echo "${CIRCLE_BRANCH:0:26}" | sed 's/[^a-zA-Z0-9]/-/g' | sed 's/-\+$//')
  echo "${this_script} also using docker tag ${feature_docker_tag} since ${CIRCLE_BRANCH} is a feature branch"
fi

git restore ../../go.work.sum # something on the releaser process is changing the go.work.sum file

cat .goreleaser.yml.envsubst |envsubst >.goreleaser.yml
goreleaser $@
if [ $? -eq 0 ] ; then
  echo "${this_script} removing the temporary .goreleaser.yml since goreleaser was successful"
  rm .goreleaser.yml # Keep git clean for additional goreleaser runs
  echo "${this_script} resetting the git repository so it is not in a dirty state for future goreleaser runs since goreleaser was successful"
  git checkout .
fi
cleanup
