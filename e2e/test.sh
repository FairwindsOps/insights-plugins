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
  --set rightsizer.image.tag="$rightsizer_tag" \
  --set uploader.image.tag="$uploader_tag"

sleep 5

echo Applying right-sizer test workload and triggering first OOM-kill.
kubectl apply -n insights-agent -f /workspace/plugins/right-sizer/e2e/testworkload.yaml
kubectl create job trigger-oomkill-testworkload -n insights-agent --image=curlimages/curl -- curl http://testworkload:8080

kubectl get all --namespace insights-agent
kubectl wait --for=condition=complete job/workloads --timeout=120s --namespace insights-agent
kubectl wait --for=condition=complete job/rbac-reporter --timeout=120s --namespace insights-agent
kubectl wait --for=condition=complete job/kube-bench --timeout=120s --namespace insights-agent
kubectl wait --for=condition=complete job/trivy --timeout=480s --namespace insights-agent
# TODO: enable OPA
# kubectl wait --for=condition=complete job/opa --timeout=480s --namespace insights-agent
kubectl wait --for=condition=complete job/kubesec --timeout=480s --namespace insights-agent
kubectl wait --for=condition=complete job/trigger-oomkill-testworkload --timeout=10s --namespace insights-agent

kubectl get jobs --namespace insights-agent

echo "Testing kube-bench"
jsonschema -i output/kube-bench.json plugins/kube-bench/results.schema || (cat output/kube-bench.json && exit 1)
echo "Testing trivy"
jsonschema -i output/trivy.json plugins/trivy/results.schema || (cat output/trivy.json && exit 1)
echo "Testing rbac-reporter"
jsonschema -i output/rbac-reporter.json plugins/rbac-reporter/results.schema || (cat output/rbac-reporter.json && exit 1)
echo "Testing Workloads"
jsonschema -i output/workloads.json plugins/workloads/results.schema || (cat output/workloads.json && exit 1)
echo "Testing Kubesec"
jsonschema -i output/kubesec.json plugins/kubesec/results.schema || (cat output/kubesec.json && exit 1)
echo "Testing right-sizer"
# For now, right-sizer data will be pulled directly from its state ConfigMap,
# instead of using a collector CronJob which would need to be delayed.
# parse ConfigMap JSON, and replace dynamic values.
# The below jq expression looks for `firstOOM` to avoid jq duplicating replaced
# keys unnecessarily to the top-level JSON object.
kubectl get configmap -n insights-agent insights-agent-right-sizer-controller-state -o jsonpath='{.data.report}' \
  | jq '(..|select(has("firstOOM"))?) += {firstOOM: "dummyvalue", lastOOM: "dummyvalue", "resourceGeneration": 0}' \
    > output/right-sizer.json
jsonschema -i output/right-sizer.json plugins/right-sizer/results.schema || (cat output/right-sizer.json && exit 1)

ls output
