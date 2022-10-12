#! /bin/bash
set -eo pipefail

RED='\033[0;31m'
NO_COLOR='\033[0m'

if mkdir tmp_repos; then
  cd tmp_repos
  git clone https://github.com/FairwindsOps/charts
  git clone https://github.com/FairwindsOps/insights-plugins
  git clone https://github.com/FairwindsOps/polaris
  git clone https://github.com/FairwindsOps/goldilocks
  git clone https://github.com/FairwindsOps/pluto
  git clone https://github.com/FairwindsOps/nova
  cd ..
fi
cd tmp_repos

values_file="./charts/stable/insights-agent/values.yaml"

declare -A latest_versions=()
declare -A used_versions=()

cloned_projects=( polaris goldilocks nova pluto )

for proj in ${cloned_projects[@]}; do
  cd $proj
  latest_versions[$proj]=$(git describe --tags --abbrev=0)
  cd ..
  used_versions[$proj]=$(yq r $values_file "$proj.image.tag")
done

echo "latest versions: ${latest_versions[@]}"
echo "used versions: ${used_versions[@]}"

need_update=0

for proj in ${cloned_projects[@]}; do
  echo -e "\n"
  latest=${latest_versions[$proj]}
  used=${used_versions[$proj]}
  echo "$proj:"
  echo "  latest:  $latest"
  echo "  used:    $used"
  if [[ $latest != $used* ]]; then
    echo -e "$RED$proj needs update from $used to $latest$NO_COLOR"
    need_update=1
  fi
done

cd ..
#rm -rf tmp_repos

if [[ $need_update -eq 1 ]]; then
  exit 1
fi

