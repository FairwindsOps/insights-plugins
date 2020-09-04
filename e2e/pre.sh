set -xeo pipefail
mkdir output
echo "export CI_BRANCH='$(echo "${CIRCLE_BRANCH:0:26}" | sed 's/[^a-zA-Z0-9]/-/g' | sed 's/-\+$//')'" >> env.sh
docker cp . e2e-command-runner:/workspace

workloads_tag=$(cat ./plugins/workloads/version.txt)
rbacreporter_tag=$(cat ./plugins/rbac-reporter/version.txt)
kubesec_tag=$(cat ./plugins/kubesec/version.txt)
kubebench_tag=$(cat ./plugins/kube-bench/version.txt)
trivy_tag=$(cat ./plugins/trivy/version.txt)
opa_tag=$(cat ./plugins/opa/version.txt)
uploader_tag=$(cat ./plugins/uploader/version.txt)

for plugin in "${CHANGED[@]}"; do
  varname=$(echo $plugin | sed -e 's/-//g')
  export $varname_tag=$CI_SHA1
done

echo "export workloads_tag=$workloads_tag >> tags.sh"
echo "export rbacreporter_tag=$rbacreporter_tag >> tags.sh"
echo "export kubesec_tag=$kubesec_tag >> tags.sh"
echo "export kubebench_tag=$kubebench_tag >> tags.sh"
echo "export trivy_tag=$trivy_tag >> tags.sh"
echo "export opa_tag=$opa_tag >> tags.sh"
echo "export uploader_tag=$uploader_tag >> tags.sh"
docker cp tags.sh e2e-command-runner:/tags.sh

