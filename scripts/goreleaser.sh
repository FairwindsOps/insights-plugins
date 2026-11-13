#!/usr/bin/env sh
# Wrap goreleaser by using envsubst on .goreleaser.yml,
# and creating a temporary git tag from an Insights plugin the version.txt file.

# prepare_goreleaser_config STRIP_AMD64 STRIP_ARM64 SKIP_COMMIT_MANIFEST
# Reads a goreleaser config on stdin and writes the config to run on stdout.
# GAR commit tags are immutable. A retry must not include a tag Buildx would
# push again, but Quay shares that image_templates list, so drop only the GAR
# arch lines under dockers. Manifests still reference the tags already in GAR.
# --provenance=false keeps each arch tag a single image so `docker manifest
# create tag@digest` can see it on Quay.
prepare_goreleaser_config() {
  awk -v strip_amd64="$1" -v strip_arm64="$2" -v skip_commit_manifest="$3" '
    BEGIN { in_dockers = 0 }
    /^dockers:/ { in_dockers = 1 }
    /^docker_manifests:/ { in_dockers = 0 }
    in_dockers && strip_amd64 == "true" && /us-docker\.pkg\.dev/ && /\.FullCommit }}-amd64/ { next }
    in_dockers && strip_arm64 == "true" && /us-docker\.pkg\.dev/ && /\.FullCommit }}-arm64/ { next }
    skip_commit_manifest == "true" && /name_template:.*us-docker\.pkg\.dev/ && /\.FullCommit }}[[:space:]]*$/ {
      print
      print "  skip_push: true"
      next
    }
    {
      print
      if (in_dockers && $0 ~ /^[[:space:]]*- "--platform=/) {
        match($0, /^[[:space:]]*/)
        indent = substr($0, 1, RLENGTH)
        print indent "- \"--provenance=false\""
        print indent "- \"--sbom=false\""
      }
    }
  '
}

if [ "${1}" = "--prepare-config" ] ; then
  prepare_goreleaser_config "$2" "$3" "$4"
  exit $?
fi

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

gar_image=$(grep -o 'us-docker\.pkg\.dev/[^:]*' .goreleaser.yml.envsubst | head -n 1)
full_commit=$(git rev-parse HEAD)
strip_amd64=false
strip_arm64=false
skip_gar_commit_tag=false
export skip_gar_version_tag=true
if [ "${gar_image}" != "" ] ; then
  if docker buildx imagetools inspect "${gar_image}:${full_commit}-amd64" >/dev/null 2>&1 ; then
    strip_amd64=true
    echo "${this_script} not pushing ${gar_image}:${full_commit}-amd64, which is already published and immutable"
  fi
  if docker buildx imagetools inspect "${gar_image}:${full_commit}-arm64" >/dev/null 2>&1 ; then
    strip_arm64=true
    echo "${this_script} not pushing ${gar_image}:${full_commit}-arm64, which is already published and immutable"
  fi
  if docker buildx imagetools inspect "${gar_image}:${full_commit}" >/dev/null 2>&1 ; then
    skip_gar_commit_tag=true
    echo "${this_script} not pushing ${gar_image}:${full_commit}, which is already published and immutable"
  fi
  if [ "${skip_main_docker_tags}" == "false" ] ; then
    if docker buildx imagetools inspect "${gar_image}:${temporary_git_tag}" >/dev/null 2>&1 ; then
      echo "${this_script} not pushing ${gar_image}:${temporary_git_tag}, which is already published and immutable"
    else
      export skip_gar_version_tag=false
    fi
  fi
fi

git restore ../../go.work.sum # something on the releaser process is changing the go.work.sum file

if [ ! -f go.mod ] ; then
  export GOWORK=off
  echo "${this_script} set GOWORK=off because this plugin has no go.mod"
elif grep -q 'github.com/inspektor-gadget/inspektor-gadget' go.mod ; then
  # Resolve before goreleaser runs. GOWORK=off so this lookup reads the plugin go.mod
  # and does not rewrite the workspace go.work.sum; goreleaser fails its dirty check if that
  # file changes. Do not rely on go list alone: it can fail when the local Go/GOTOOLCHAIN is
  # older than the plugin, while goreleaser itself later builds with a newer toolchain from
  # .goreleaser.yml — leaving IG_VERSION empty and breaking Docker FROM.
  IG_VERSION="$(GOWORK=off go list -m -f '{{.Version}}' github.com/inspektor-gadget/inspektor-gadget 2>/dev/null || true)"
  if [ -z "${IG_VERSION}" ] ; then
    IG_VERSION="$(awk '/github\.com\/inspektor-gadget\/inspektor-gadget($|[[:space:]])/ {print $2; exit}' go.mod)"
  fi
  if [ -z "${IG_VERSION}" ] ; then
    echo "${this_script} failed to resolve IG_VERSION from go.mod for github.com/inspektor-gadget/inspektor-gadget" >&2
    exit 1
  fi
  export IG_VERSION
  echo "${this_script} using IG_VERSION=${IG_VERSION} from go.mod"
fi

if grep -q 'IG_VERSION' .goreleaser.yml.envsubst && [ -z "${IG_VERSION}" ] ; then
  echo "${this_script} .goreleaser.yml.envsubst references IG_VERSION but it is unset" >&2
  exit 1
fi

envsubst < .goreleaser.yml.envsubst > "${TMPDIR}/goreleaser.yml.unprepared"
prepare_goreleaser_config "${strip_amd64}" "${strip_arm64}" "${skip_gar_commit_tag}" < "${TMPDIR}/goreleaser.yml.unprepared" > .goreleaser.yml
rm -f "${TMPDIR}/goreleaser.yml.unprepared"
goreleaser $@
if [ $? -eq 0 ] ; then
  echo "${this_script} removing the temporary .goreleaser.yml since goreleaser was successful"
  rm .goreleaser.yml # Keep git clean for additional goreleaser runs
  echo "${this_script} resetting the git repository so it is not in a dirty state for future goreleaser runs since goreleaser was successful"
  git checkout .
fi
cleanup
