#! /bin/bash
set -xeo pipefail

changed=()
for dir in `find ./plugins -maxdepth 1 -type d`; do
  if [ $dir == "./plugins" ]; then
    continue
  fi
  if git diff --name-only --exit-code --no-renames origin/master "$dir/" > /dev/null 2>&1 && [ "$CIRCLE_BRANCH" != "master" ] ; then
    continue
  fi
  echo "detected change in $dir"
  changed+=(${dir#"./plugins/"})
done
echo "export CHANGED=(${changed[*]})" >> ${BASH_ENV}

for plugin in ./plugins/*; do
  if [ -f $plugin ] || [ $plugin == "./plugins" ] ; then
    continue
  fi

  id=$(echo $plugin | sed -e 's/\.\/plugins\///g')
  varname=$(echo $id | sed -e 's/-//g')
  tag=$(cat "$plugin/version.txt")

  if [ "$CIRCLE_BRANCH" != "master" ]; then
    for changed_id in "${CHANGED[@]}"; do
      if [ $id == $changed_id ]; then
        tag=$CI_SHA1
      fi
      export ${varname}_tag=$CI_SHA1
    done
  fi

  echo "export ${varname}_tag=$tag" >> tags.sh
done
cat tags.sh >> ${BASH_ENV}

