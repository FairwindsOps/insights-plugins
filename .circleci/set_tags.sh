#! /bin/bash
set -xeo pipefail

# Feature branch: a plugin that differs from main is pulled by commit SHA.
# A plugin that does not differ is pulled by its version.txt tag.
sha="${CI_SHA1:-$CIRCLE_SHA1}"

git fetch --no-tags origin +refs/heads/main:refs/remotes/origin/main 2>/dev/null || \
  git fetch origin main --depth=200 || true

changed=()
plugins=()
go_pkgs=()

for dir in `find ./plugins -maxdepth 1 -type d`; do
  name=${dir#"./plugins/"}
  if [ ! -f "$dir/Dockerfile" ]; then
    continue
  fi
  plugins+=($name)

  if [ -f "$dir/go.mod" ]; then
    go_pkgs+=($name)
  fi

  if git diff --name-only --exit-code --no-renames origin/main "$dir/" > /dev/null 2>&1 && [ "$CIRCLE_BRANCH" != "main" ] ; then
    continue
  fi

  echo "detected change in $dir"
  changed+=($name)
done

echo "export PLUGINS=(${plugins[*]})" >> ${BASH_ENV}
echo "export CHANGED=(${changed[*]})" >> ${BASH_ENV}
echo "export GO_PKGS=(${go_pkgs[*]})" >> ${BASH_ENV}

for plugin in ./plugins/*; do
  if [ -f $plugin ] || [ $plugin == "./plugins" ] ; then
    continue
  fi

  id=$(echo $plugin | sed -e 's/\.\/plugins\///g')
  varname=$(echo $id | sed -e 's/-//g')
  tag=$(cat "$plugin/version.txt")

  if [ "$CIRCLE_BRANCH" != "main" ]; then
    for changed_id in "${changed[@]}"; do
      if [ "$id" = "$changed_id" ]; then
        tag=$sha
        break
      fi
    done
  fi

  echo "export ${varname}_tag=$tag" >> tags.sh
done
cat tags.sh >> ${BASH_ENV}
