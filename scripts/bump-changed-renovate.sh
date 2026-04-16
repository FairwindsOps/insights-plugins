#! /bin/bash
set -eo pipefail

# Bump version.txt / CHANGELOG.md for plugins changed on a Renovate PR branch.
# Intended for CI (e.g. .circleci/scripts/bump-renovate-pr.sh).

message=$1

if [[ -z $message ]]; then
  echo "Usage: ./scripts/bump-changed-renovate.sh 'Message to add to the changelog'"
  exit 1
fi

bump_one_plugin() {
  local d=$1
  local msg=$2
  version=$(cat "$d/version.txt" | awk -F. '{$NF = $NF + 1;} 1' | sed 's/ /./g')
  echo "$version" > "$d/version.txt"
  echo -e "# Changelog" > /tmp/CHANGELOG.md
  echo -e "\n## $version" >> /tmp/CHANGELOG.md
  echo -e "* $msg" >> /tmp/CHANGELOG.md
  tail -n+2 "$d/CHANGELOG.md" >> /tmp/CHANGELOG.md
  mv /tmp/CHANGELOG.md "$d/CHANGELOG.md"
}

should_bump_renovate_ci() {
  local d=$1
  local rel="${d#./}"

  if git diff --name-only --exit-code origin/main -- "$d" > /dev/null 2>&1; then
    return 1
  fi

  local main_ver head_ver last_ver_commit changed_since_ver
  main_ver=$(git show "origin/main:${rel}version.txt" 2>/dev/null | tr -d '\n\r' || true)
  head_ver=$(tr -d '\n\r' < "${d}version.txt")
  last_ver_commit=$(git log -1 --format=%H -- "${d}version.txt" 2>/dev/null || true)
  if [[ -z "$last_ver_commit" ]]; then
    return 1
  fi

  changed_since_ver=$(git diff --name-only "$last_ver_commit" HEAD -- "$d" | grep -vE 'version\.txt$|CHANGELOG\.md$' || true)
  if [[ -n "$changed_since_ver" ]]; then
    return 0
  fi
  if [[ "$main_ver" == "$head_ver" ]]; then
    return 0
  fi
  return 1
}

for d in ./plugins/*/ ; do
  if [[ "$d" == *"_template/" ]]; then
    continue
  fi
  if should_bump_renovate_ci "$d"; then
    echo "$d"
    bump_one_plugin "$d" "$message"
  fi
done
