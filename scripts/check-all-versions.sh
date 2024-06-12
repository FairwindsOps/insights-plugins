#! /bin/bash
set -eo pipefail

RED='\033[0;31m'
NO_COLOR='\033[0m'

if mkdir tmp_repos; then
  cd tmp_repos
  git clone https://github.com/FairwindsOps/charts
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
  used_versions[$proj]=$(yq e ".$proj.image.tag" $values_file)
done

plugin_projects=( aws-costs falco-agent kube-bench kube-bench-aggregator opa prometheus rbac-reporter right-sizer trivy uploader workloads )
declare -A rewrites=()
for proj in ${plugin_projects[@]}; do
  rewrites[$proj]=$proj
done
rewrites["aws-costs"]="awscosts"
rewrites["falco-agent"]="falco"
rewrites["kube-bench-aggregator"]="kube-bench.aggregator"
rewrites["prometheus"]="prometheus-metrics"
rewrites["right-sizer"]="insights-right-sizer"

for proj in ${plugin_projects[@]}; do
  latest_versions[$proj]=$(cat ../plugins/$proj/version.txt)
  value_name=${rewrites[$proj]}
  used_versions[$proj]=$(yq e ".$value_name.image.tag" $values_file)
done

latest_versions["admission"]=$(cat ../plugins/admission/version.txt)
used_versions["admission"]=$(yq e ".appVersion" ./charts/stable/insights-admission/Chart.yaml)

latest_versions["admission-chart"]=$(yq e ".version" ./charts/stable/insights-admission/Chart.yaml)
used_versions["admission-chart"]=$(yq e ".dependencies[] | select(.name == \"insights-admission\").version" ./charts/stable/insights-agent/requirements.yaml)

all_projects=( admission admission-chart )
all_projects+=(${cloned_projects[@]})
all_projects+=(${plugin_projects[@]})

echo "latest versions: ${latest_versions[@]}"
echo "used versions: ${used_versions[@]}"

need_update=0

if [[ -n $BASH_ENV ]]; then
  echo "export OUTDATED_VERSIONS_LIST=''" >> ${BASH_ENV}
fi

for proj in ${all_projects[@]}; do
  echo -e "\n"
  latest=${latest_versions[$proj]}
  used=${used_versions[$proj]}
  echo "$proj:"
  echo "  latest:  $latest"
  echo "  used:    $used"
  if [[ $latest != $used* ]]; then
    echo -e "$RED$proj needs update from $used to $latest$NO_COLOR"
    need_update=1
    if [[ -n $BASH_ENV ]]; then
      echo "OUTDATED_VERSIONS_LIST+='- ${proj}: ${used} should be updated to ${latest}\n'">> ${BASH_ENV}
    fi
  fi
done

cd ..

rm -rf tmp_repos


if [[ $need_update -eq 1 ]]; then
  exit 1
fi

