#! /bin/bash
set -eo pipefail

# On feature branches, changed plugins are tagged with the git commit SHA on GAR
# (no floating/feature tags). Pass CIRCLE_SHA1 as $1 when scanning PR builds.
declare override_tag=$1
declare -a changed_plugins=($2)

novaVersion=v3.12.0
plutoVersion=v5.24.0
goldilocksVersion=v4.15.0
polarisVersion=v10.2.0

# Hard-coding four external images we own. Versions taken from insights-agent. OSS images live on Artifact Registry
images=(us-docker.pkg.dev/fairwinds-ops/oss/polaris:${polarisVersion} us-docker.pkg.dev/fairwinds-ops/oss/nova:${novaVersion} us-docker.pkg.dev/fairwinds-ops/oss/pluto:${plutoVersion} us-docker.pkg.dev/fairwinds-ops/oss/goldilocks:${goldilocksVersion})
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
    # Prefer GAR (same short name as Quay). Pin semver from version.txt — GAR has no latest/major/major.minor.
    repo=$(grep -oE 'us-docker\.pkg\.dev/fairwinds-ops/oss/[^:"[:space:]]+' "$d/.goreleaser.yml.envsubst" | head -1)
    name="$repo:$version"
    images+=($name)
done

echo "regenerating image list in fairwinds-insights.yaml"
tmp=$(mktemp)
awk '/^images:/{exit} {print}' fairwinds-insights.yaml > "$tmp"
{
  echo "images:"
  echo "  docker:"
  for name in "${images[@]}"; do
    echo "    - $name"
  done
} >> "$tmp"
mv "$tmp" fairwinds-insights.yaml

declare -A changed_plugins_map
for plugin in "${changed_plugins[@]}"; do
  changed_plugins_map[$plugin]=1
done

# Match images in images array to the plugin name (GAR paths).
declare -A plugin_map
plugin_map["us-docker.pkg.dev/fairwinds-ops/oss/insights-admission-controller"]="admission"
plugin_map["us-docker.pkg.dev/fairwinds-ops/oss/insights-ci"]="ci"
plugin_map["us-docker.pkg.dev/fairwinds-ops/oss/cloud-costs"]="cloud-costs"
plugin_map["us-docker.pkg.dev/fairwinds-ops/oss/falco-agent"]="falco-agent"
plugin_map["us-docker.pkg.dev/fairwinds-ops/oss/fw-kube-bench-aggregator"]="kube-bench-aggregator"
plugin_map["us-docker.pkg.dev/fairwinds-ops/oss/fw-kube-bench"]="kube-bench"
plugin_map["us-docker.pkg.dev/fairwinds-ops/oss/network-flow-aggregator"]="network-flow-aggregator"
plugin_map["us-docker.pkg.dev/fairwinds-ops/oss/network-flow"]="network-flow"
plugin_map["us-docker.pkg.dev/fairwinds-ops/oss/kubectl"]="kubectl"
plugin_map["us-docker.pkg.dev/fairwinds-ops/oss/kyverno"]="kyverno"
plugin_map["us-docker.pkg.dev/fairwinds-ops/oss/fw-opa"]="opa"
plugin_map["us-docker.pkg.dev/fairwinds-ops/oss/prometheus-collector"]="prometheus"
plugin_map["us-docker.pkg.dev/fairwinds-ops/oss/rbac-reporter"]="rbac-reporter"
plugin_map["us-docker.pkg.dev/fairwinds-ops/oss/right-sizer"]="right-sizer"
plugin_map["us-docker.pkg.dev/fairwinds-ops/oss/fw-trivy"]="trivy"
plugin_map["us-docker.pkg.dev/fairwinds-ops/oss/image-trust"]="image-trust"
plugin_map["us-docker.pkg.dev/fairwinds-ops/oss/insights-uploader"]="uploader"
plugin_map["us-docker.pkg.dev/fairwinds-ops/oss/insights-utils"]="utils"
plugin_map["us-docker.pkg.dev/fairwinds-ops/oss/workloads"]="workloads"
plugin_map["us-docker.pkg.dev/fairwinds-ops/oss/on-demand-job-runner"]="on-demand-job-runner"
plugin_map["us-docker.pkg.dev/fairwinds-ops/oss/kyverno-policy-sync"]="kyverno-policy-sync"
plugin_map["us-docker.pkg.dev/fairwinds-ops/oss/insights-event-watcher"]="event-watcher"

echo "scanning all images"
for name in "${images[@]}"; do
    if [[ $SKIP_TRIVY == "true" ]]; then
      break
    fi

    name_without_tag=$(echo $name | sed "s/:.*//")
    if [[ $name_without_tag == "us-docker.pkg.dev/fairwinds-ops/oss/postgres-partman" ]]; then
      echo "skipping postgres-partman"
      continue
    fi
    if [[ -n ${plugin_map[$name_without_tag]} ]]; then
      if [[ -n ${changed_plugins_map[${plugin_map[$name_without_tag]}]} ]] && [[ -n $override_tag ]]; then
        name="${name_without_tag}:${override_tag}"
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
