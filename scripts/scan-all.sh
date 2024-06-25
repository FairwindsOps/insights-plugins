#! /bin/bash
set -eo pipefail

declare branch_name=$1
declare -a changed_plugins=($2)

branch_name=$(echo "${branch_name:0:26}" | sed 's/[^a-zA-Z0-9]/-/g' | sed 's/-\+$//')

# Hard-coding four external images we own. Versions taken from insights-agent. Need to find a better solution here.
images=(quay.io/fairwinds/polaris:9.0 quay.io/fairwinds/nova:v3.9 us-docker.pkg.dev/fairwinds-ops/oss/pluto:v5.19 us-docker.pkg.dev/fairwinds-ops/oss/goldilocks:v4.11)
have_vulns=()

for d in ./plugins/*/ ; do
    echo $d
    if [[ $d == *"_template"* ]]; then
      continue
    fi
    if [[ ! -f "$d/.goreleaser.yml.envsubst" ]]; then
      continue
    fi
    version=$(cat $d/version.txt)
    repo=$(cat "$d/.goreleaser.yml.envsubst" | grep "quay.io" | head -1 | sed s/:.*// | sed 's/^  - "//')
    name="$repo:$version"
    images+=($name)
done

echo "regenerating image list in fairwinds-insights.yaml"
sed -i -n '/images:/q;p' fairwinds-insights.yaml
echo -e "images:" >> ./fairwinds-insights.yaml
echo -e "  docker:" >> ./fairwinds-insights.yaml
for name in "${images[@]}"; do
  echo -e "    - $name" >> ./fairwinds-insights.yaml
done

declare -A changed_plugins_map
for plugin in "${changed_plugins[@]}"; do
  changed_plugins_map[$plugin]=1
done

# create a map to match images in images array to the plugin name
declare -A plugin_map
plugin_map["quay.io/fairwinds/insights-admission-controller"]="admission"
plugin_map["quay.io/fairwinds/aws-costs"]="aws-costs"
plugin_map["quay.io/fairwinds/insights-ci"]="ci"
plugin_map["quay.io/fairwinds/cloud-costs"]="cloud-costs"
plugin_map["quay.io/fairwinds/falco-agent"]="falco-agent"
plugin_map["quay.io/fairwinds/fw-kube-bench-aggregator"]="kube-bench-aggregator"
plugin_map["quay.io/fairwinds/fw-kube-bench"]="kube-bench"
plugin_map["quay.io/fairwinds/kubectl"]="kubectl"
plugin_map["quay.io/fairwinds/fw-kubesec"]="kubesec"
plugin_map["quay.io/fairwinds/kyverno"]="kyverno"
plugin_map["quay.io/fairwinds/fw-opa"]="opa"
plugin_map["quay.io/fairwinds/postgres-partman"]="postgres-partman"
plugin_map["quay.io/fairwinds/prometheus-collector"]="prometheus"
plugin_map["quay.io/fairwinds/rbac-reporter"]="rbac-reporter"
plugin_map["quay.io/fairwinds/right-sizer"]="right-sizer"
plugin_map["quay.io/fairwinds/fw-trivy"]="trivy"
plugin_map["quay.io/fairwinds/insights-uploader"]="uploader"
plugin_map["quay.io/fairwinds/insights-utils"]="utils"
plugin_map["quay.io/fairwinds/workloads"]="workloads"

echo "scanning all images"
for name in "${images[@]}"; do
    if [[ $SKIP_TRIVY == "true" ]]; then
      break
    fi

    name_without_tag=$(echo $name | sed "s/:.*//") 
    if [[ -n ${plugin_map[$name_without_tag]} ]]; then
      if [[ -n ${changed_plugins_map[${plugin_map[$name_without_tag]}]} ]]; then
        name=$(echo $name_without_tag:$branch_name)
      fi
    fi

    echo "scanning $name"
    docker pull $name

    set +e
    trivy i --exit-code 123 --severity CRITICAL,HIGH $name
    if [[ $? -eq 123 ]]; then
      have_vulns+=($name)
    fi
    set -e
    echo "done with scan!"
done

if [[ -n $BASH_ENV ]]; then
  echo "export VULNERABLE_IMAGES_LIST=''" >> ${BASH_ENV}
fi

if (( ${#have_vulns[@]} != 0 )); then
    echo "The following images have vulnerabilities:"
    for image in "${have_vulns[@]}"; do
      if [[ -n $BASH_ENV ]]; then
        echo "VULNERABLE_IMAGES_LIST+='- ${image}\n'">> ${BASH_ENV}
      fi
      echo $image
    done
    exit 1
fi
