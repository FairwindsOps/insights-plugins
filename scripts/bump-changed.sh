#! /bin/bash
set -eo pipefail

message=$1
force=$2

if [[ -z $1 ]]; then
  echo "Usage: ./scripts/bump-changed.sh 'Message to add to the changelog' [--force]"
  exit 1
fi

for d in ./plugins/*/ ; do
    if [[ $d == "_template" ]]; then
      continue
    fi
    echo "$d"
    if [[ -z $force ]]; then
      if git diff origin/main --exit-code --quiet $d; then
        continue
      fi
    fi
    version=$(cat $d/version.txt | awk -F. '{$NF = $NF + 1;} 1' | sed 's/ /./g')
    echo $version > $d/version.txt
    echo -e "# Changelog" > /tmp/CHANGELOG.md
    echo -e "\n## $version" >> /tmp/CHANGELOG.md
    echo -e "* $message" >> /tmp/CHANGELOG.md
    tail $d/CHANGELOG.md -n +2 >> /tmp/CHANGELOG.md
    mv /tmp/CHANGELOG.md $d/CHANGELOG.md
done

