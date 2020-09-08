set -xeo pipefail
mkdir output
echo "export CI_BRANCH='$(echo "${CIRCLE_BRANCH:0:26}" | sed 's/[^a-zA-Z0-9]/-/g' | sed 's/-\+$//')'" >> env.sh
docker cp . e2e-command-runner:/workspace

# TODO: deduplicate this logic once rok8s moves from bash to eval
changed=()
for dir in `find ./plugins -maxdepth 1 -type d`; do
  if [ $dir == "./plugins" ]; then
    continue
  fi
  if git diff --name-only --exit-code --no-renames origin/master "$dir/" > /dev/null 2>&1 ; then
    continue
  fi
  echo "detected change in $dir"
  changed+=(${dir#"./plugins/"})
done

workloads_tag=$(cat ./plugins/workloads/version.txt)
rbacreporter_tag=$(cat ./plugins/rbac-reporter/version.txt)
kubesec_tag=$(cat ./plugins/kubesec/version.txt)
kubebench_tag=$(cat ./plugins/kube-bench/version.txt)
trivy_tag=$(cat ./plugins/trivy/version.txt)
opa_tag=$(cat ./plugins/opa/version.txt)
uploader_tag=$(cat ./plugins/uploader/version.txt)

echo "changed: $changed"
echo "changed arr: ${changed[@]}"

for plugin in "${changed[@]}"; do
  varname=$(echo $plugin | sed -e 's/-//g')
  export ${varname}_tag=$CI_SHA1
done

echo "export workloads_tag=$workloads_tag" >> tags.sh
echo "export rbacreporter_tag=$rbacreporter_tag" >> tags.sh
echo "export kubesec_tag=$kubesec_tag" >> tags.sh
echo "export kubebench_tag=$kubebench_tag" >> tags.sh
echo "export trivy_tag=$trivy_tag" >> tags.sh
echo "export opa_tag=$opa_tag" >> tags.sh
echo "export uploader_tag=$uploader_tag" >> tags.sh
docker cp tags.sh e2e-command-runner:/workspace/tags.sh

