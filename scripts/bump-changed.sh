#! /bin/bash
set -eo pipefail

# USAGE: ./scripts/bump-changed.sh "Message to add to the changelog"

for d in ./plugins/*/ ; do
    echo "$d"
    if ! git diff origin/main --exit-code --quiet $d; then
      version=$(cat $d/version.txt | awk -F. '{$NF = $NF + 1;} 1' | sed 's/ /./g')
      echo $version > $d/version.txt
      echo -e "# Changelog" > /tmp/CHANGELOG.md
      echo -e "\n## $version" >> /tmp/CHANGELOG.md
      echo -e "* $1" >> /tmp/CHANGELOG.md
      tail $d/CHANGELOG.md -n +2 >> /tmp/CHANGELOG.md
      mv /tmp/CHANGELOG.md $d/CHANGELOG.md
    fi
done

