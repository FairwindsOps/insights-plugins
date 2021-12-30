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
kubectl wait --for=condition=ready -l app=right-sizer-test-workload pod --timeout=60s --namespace insights-agent
kubectl create job trigger-oomkill-right-sizer-test-workload -n insights-agent --image=curlimages/curl -- curl http://right-sizer-test-workload:8080

kubectl get all --namespace insights-agent
kubectl wait --for=condition=complete job/workloads --timeout=120s --namespace insights-agent
kubectl wait --for=condition=complete job/rbac-reporter --timeout=120s --namespace insights-agent
kubectl wait --for=condition=complete job/kube-bench --timeout=120s --namespace insights-agent
kubectl wait --for=condition=complete job/trivy --timeout=480s --namespace insights-agent
# TODO: enable OPA
# kubectl wait --for=condition=complete job/opa --timeout=480s --namespace insights-agent
kubectl wait --for=condition=complete job/kubesec --timeout=480s --namespace insights-agent
kubectl wait --for=condition=complete job/trigger-oomkill-right-sizer-test-workload --timeout=10s --namespace insights-agent

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
# Make sure the test workload has a container restart.
for n in `seq 1 20` ; do
  # Restarts from all Deployment pods are sumed, in case the ReplicaSet is healing.
  rightsizer_workload_restarts=$(kubectl get po -n insights-agent -l app=right-sizer-test-workload -o json \
    | jq '.items[].status.containerStatuses[0].restartCount' \
    | awk '{s+=$1} END {printf "%.0f", s}')
  if [ ${rightsizer_workload_restarts} -gt 0 ] ; then
    break
  fi
  sleep 3
done
if [ $rightsizer_workload_restarts -eq 0 ] ; then
  echo There were no right-sizer test workload restarts after checking $n times.
  false # Fail the test.
fi
# Pull right-sizer data directly from the controller state ConfigMap,
# to obtain JSON for checking against the schema.
for n in `seq 1 10` ; do
  kubectl get configmap -n insights-agent insights-agent-right-sizer-controller-state -o jsonpath='{.data.report}' \
    > output/right-sizer.json
  rightsizer_num_items=$(jq '.items |length' output/right-sizer.json)
  if [ $rightsizer_num_items -gt 0 ] ; then
    break
  fi
  sleep 3
done
if [ $rightsizer_num_items -eq 0 ] ; then
  echo The right-sizer controller has no report items.
  cat output/right-sizer.json
  false # Fail the test.
fi
jsonschema -i output/right-sizer.json plugins/right-sizer/results.schema || (cat output/right-sizer.json && exit 1)

ls output
