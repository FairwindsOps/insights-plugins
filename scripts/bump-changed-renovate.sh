#! /bin/bash
set -eo pipefail

# Bump version.txt / CHANGELOG.md for plugins changed on a Renovate PR branch.
# Intended for CI (e.g. .circleci/scripts/bump-renovate-pr.sh).

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)

message=$1

if [[ -z $message ]]; then
  echo "Usage: ./scripts/bump-changed-renovate.sh 'Message to add to the changelog'"
  exit 1
fi

bump_one_plugin() {
  local d=$1
  local fallback_msg=$2
  local rel="${d#./}go.mod"
  local bullets_tmp
  bullets_tmp=$(mktemp)

  if [[ -f "$rel" ]]; then
    local py_out
    py_out=$(python3 "${SCRIPT_DIR}/gomod-diff-changelog.py" "$rel" 2>/dev/null) || py_out=""
    if [[ -n "$py_out" ]]; then
      printf '%s\n' "$py_out" > "$bullets_tmp"
    else
      printf '%s\n' "$fallback_msg" > "$bullets_tmp"
    fi
  else
    printf '%s\n' "$fallback_msg" > "$bullets_tmp"
  fi

  local version
  version=$(cat "$d/version.txt" | awk -F. '{$NF = $NF + 1;} 1' | sed 's/ /./g')
  echo "$version" > "$d/version.txt"
  echo -e "# Changelog" > /tmp/CHANGELOG.md
  echo -e "\n## $version" >> /tmp/CHANGELOG.md
  while IFS= read -r line || [[ -n "$line" ]]; do
    [[ -z "$line" ]] && continue
    echo "* $line" >> /tmp/CHANGELOG.md
  done < "$bullets_tmp"
  rm -f "$bullets_tmp"
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
