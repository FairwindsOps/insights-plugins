#! /bin/bash
set -xeo pipefail

for plugin in ./plugins/*; do
  if [ -f $plugin ] || [ $plugin == "./plugins" ] ; then
    continue
  fi

  id=$(echo $plugin | sed -e 's/\.\/plugins\///g')
  varname=$(echo $id | sed -e 's/-//g')
  tag=$(cat "$plugin/version.txt")

  for changed_id in "${CHANGED[@]}"; do
    if [ $id == $changed_id ]; then
      tag=$CI_SHA1
    fi
    export ${varname}_tag=$CI_SHA1
  done

  echo "export ${varname}_tag=$tag" >> tags.sh
done

