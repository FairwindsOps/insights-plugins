set -eo pipefail
cd /workspace

helm repo add fairwinds-incubator https://charts.fairwinds.com/incubator
helm repo add fairwinds-stable https://charts.fairwinds.com/stable
python3 -u e2e/testServer.py &> /workspace/py.log &
pyServer=$!

trap "cat /workspace/py.log && kill $pyServer" EXIT
sleep 5
insightsHost="http://$(awk 'END{print $1}' /etc/hosts):8080"
kubectl create namespace insights-agent

cat ./tags.sh
source ./tags.sh

# TODO: add some OPA checks

helm upgrade --install insights-agent fairwinds-stable/insights-agent \
  --namespace insights-agent \
  -f e2e/values.yaml \
  --set insights.host="$insightsHost" \
  --set insights.base64token="$(echo -n "Erehwon" | base64)" \
  --set workloads.image.tag="$workloads_tag" \
  --set rbacreporter.image.tag="$rbacreporter_tag" \
  --set kubesec.image.tag="$kubesec_tag" \
  --set kubebench.image.tag="$kubebench_tag" \
  --set trivy.image.tag="$trivy_tag" \
  --set opa.image.tag="$opa_tag" \
  --set uploader.image.tag="$uploader_tag"

function rerunWorkloads {
  kubectl -n insights-agent delete job workloads
  kubectl -n insights-agent create job workloads --from=cronjob/workloads
  sleep 5
  kubectl wait --for=condition=complete job/workloads --timeout=120s --namespace insights-agent
}

sleep 5
kubectl get all --namespace insights-agent
kubectl wait --for=condition=complete job/rbac-reporter --timeout=120s --namespace insights-agent
kubectl wait --for=condition=complete job/kube-bench --timeout=120s --namespace insights-agent
kubectl wait --for=condition=complete job/trivy --timeout=480s --namespace insights-agent
# TODO: enable OPA
# kubectl wait --for=condition=complete job/opa --timeout=480s --namespace insights-agent
kubectl wait --for=condition=complete job/kubesec --timeout=480s --namespace insights-agent
kubectl wait --for=condition=complete job/workloads --timeout=120s --namespace insights-agent

kubectl get jobs --namespace insights-agent
echo "Testing kube-bench"
jsonschema -i output/kube-bench.json plugins/kube-bench/results.schema
echo "Testing trivy"
jsonschema -i output/trivy.json plugins/trivy/results.schema
echo "Testing rbac-reporter"
jsonschema -i output/rbac-reporter.json plugins/rbac-reporter/results.schema
echo "Testing Workloads"
workloadsfailed=0
jsonschema -i output/workloads.json plugins/workloads/results.schema || workloadsfailed=1
if [[ "$workloadsfailed" == "1" ]]
then
  cat output/workloads.json
  rerunWorkloads  
  jsonschema -i output/workloads.json plugins/workloads/results.schema || workloadsfailed=1
fi
echo "Testing Kubesec"
jsonschema -i output/kubesec.json plugins/kubesec/results.schema
ls output
