#! /bin/bash
# Feature branch: a plugin that differs from main uses the commit SHA.
# A plugin that does not differ uses version.txt.
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
export GIT_AUTHOR_NAME=test GIT_AUTHOR_EMAIL=test@example.com
export GIT_COMMITTER_NAME=test GIT_COMMITTER_EMAIL=test@example.com

fail() { echo "set-tags-published-version: $*" >&2; exit 1; }

init_repo() {
  local dir="$1"
  mkdir -p "${dir}/plugins/demo" "${dir}/.circleci"
  cp "${root}/.circleci/set_tags.sh" "${dir}/.circleci/set_tags.sh"
  cat >"${dir}/plugins/demo/Dockerfile" <<'EOF'
FROM scratch
EOF
  git -C "$dir" init -q
  git -C "$dir" checkout -q -b main
}

commit_version() {
  local dir="$1" version="$2"
  printf '%s\n' "$version" >"${dir}/plugins/demo/version.txt"
  git -C "$dir" add plugins/demo
  git -C "$dir" commit -q -m "version ${version}"
}

run_set_tags() {
  local dir="$1"
  local bash_env="${dir}/bash_env"
  : >"$bash_env"
  (
    cd "$dir"
    export BASH_ENV="$bash_env"
    export CIRCLE_BRANCH=feature
    export CI_SHA1=abc123
    bash ./.circleci/set_tags.sh
  )
}

# Same tree as main: pull version.txt, not an older tag and not the SHA.
unchanged=$(mktemp -d)
init_repo "$unchanged"
commit_version "$unchanged" 1.0.0
commit_version "$unchanged" 9.9.9
git -C "$unchanged" update-ref refs/remotes/origin/main HEAD
git -C "$unchanged" checkout -q -b feature
run_set_tags "$unchanged"
grep -qx 'export demo_tag=9.9.9' "${unchanged}/tags.sh" || fail "unchanged plugin should use version.txt, got: $(cat "${unchanged}/tags.sh")"

# Differs from main: pull the commit SHA.
changed=$(mktemp -d)
init_repo "$changed"
commit_version "$changed" 1.0.0
git -C "$changed" update-ref refs/remotes/origin/main HEAD
git -C "$changed" checkout -q -b feature
commit_version "$changed" 9.9.9
run_set_tags "$changed"
grep -qx 'export demo_tag=abc123' "${changed}/tags.sh" || fail "changed plugin should use CI_SHA1, got: $(cat "${changed}/tags.sh")"

echo "set-tags-published-version: ok"
