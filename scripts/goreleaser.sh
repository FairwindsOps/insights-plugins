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
goreleaser
git tag -d ${temporary_git_tag}

