#! /bin/bash
set -xeo pipefail

changed=()
for dir in `find ./plugins -maxdepth 1 -type d`; do
  if [ ! -f "$dir/Dockerfile" ]; then
    continue
  fi
  if git diff --name-only --exit-code --no-renames origin/main "$dir/" > /dev/null 2>&1 && [ "$CIRCLE_BRANCH" != "main" ] ; then
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

  if [ "$CIRCLE_BRANCH" != "main" ]; then
    for changed_id in "${changed[@]}"; do
      if [ $id == $changed_id ]; then
        tag=$CI_SHA1
      fi
      export ${varname}_tag=$CI_SHA1
    done
  fi

  echo "export ${varname}_tag=$tag" >> tags.sh
done
cat tags.sh >> ${BASH_ENV}

