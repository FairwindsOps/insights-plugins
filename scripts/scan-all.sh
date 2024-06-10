#! /bin/bash
set -eo pipefail

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

echo "scanning all images"
for name in "${images[@]}"; do
    if [[ $SKIP_TRIVY == "true" ]]; then
      break
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
