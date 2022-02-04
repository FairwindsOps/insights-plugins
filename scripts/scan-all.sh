#! /bin/bash
set -eo pipefail

for d in ./plugins/*/ ; do
    if [[ $d == "_template" ]]; then
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
    trivy $name
done

