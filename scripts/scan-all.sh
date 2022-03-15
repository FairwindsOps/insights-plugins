#! /bin/bash
set -eo pipefail

have_vulns=()

for d in ./plugins/*/ ; do
    echo $d
    if [[ $d == *"_template"* ]]; then
      continue
    fi
    if [[ ! -f $d/build.config ]]; then
      continue
    fi
    version=$(cat $d/version.txt)
    . $d/build.config
    name="quay.io/$REPOSITORY_NAME:$version"
    echo "scanning $name"
    docker pull $name

    set +e
    trivy i --exit-code 123 $name
    if [[ $? -eq 123 ]]; then
      have_vulns+=($name)
    fi
    set -e
done

if [[ -n $BASH_ENV ]]; then
  echo "export VULNERABLE_IMAGES=''" >> ${BASH_ENV}
fi

if (( ${#have_vulns[@]} != 0 )); then
    echo "The following images have vulnerabilities:"
    for image in "${have_vulns[@]}"; do
      if [[ -n $BASH_ENV ]]; then
        echo "VULNERABLE_IMAGES+='* ${image}\n'">> ${BASH_ENV}
      fi
      echo $image
    done
    exit 1
fi
