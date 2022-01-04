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

echo "Applying right-sizer test workload and triggering first OOM-kill."
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
rightsizer_workload_restarts=$(get_restarts_of_first_container app=right-sizer-test-workload 1 -n insights-agent)
    echo "Got ${rightsizer_workload_restarts} after the first trigger of an OOM-kill."
echo "Triggering second OOM-kill for right-sizer test workload."
kubectl create job trigger-oomkill2-right-sizer-test-workload -n insights-agent --image=curlimages/curl -- curl http://right-sizer-test-workload:8080
kubectl wait --for=condition=complete job/trigger-oomkill2-right-sizer-test-workload --timeout=10s --namespace insights-agent
rightsizer_workload_restarts=$(get_restarts_of_first_container app=right-sizer-test-workload 2 -n insights-agent)
    echo "Got ${rightsizer_workload_restarts} after the second trigger of an OOM-kill."
# Pull right-sizer data directly from the controller state ConfigMap,
# to obtain JSON for checking against the schema.
for n in `seq 1 10` ; do
  kubectl get configmap -n insights-agent insights-agent-right-sizer-controller-state -o jsonpath='{.data.report}' \
    > output/right-sizer.json
  # Wait for the right-sizer controller to process the test workload OOM-kill,
  # before processing right-sizer output.
  rightsizer_num_items=$(jq '.items |length' output/right-sizer.json)
  if [ $rightsizer_num_items -gt 0 ] ; then
    break
  fi
  sleep 3
done
if [ $rightsizer_num_items -eq 0 ] ; then
  echo "The right-sizer controller has no report items after checking $n times."
  cat output/right-sizer.json
  false # Fail the test.
fi
jsonschema -i output/right-sizer.json plugins/right-sizer/results.schema || (cat output/right-sizer.json && exit 1)
echo right-sizer output is: && cat output/right-sizer.json
ls output

# Get the number of restarts for the first container of all pods with the given label.
# Parameters: <label> <expected restarts> [optional arguments to kubectl]
# <label> is a key=value form passed to the -l kubectl flag.
# <expected restarts> is the number of restarts expected, after which the function returns.
# [optional arguments to kubectl] allows specifying I.E. a namespace or other flags.
get_restarts_of_first_container() {
  local label="$1"
  local expected_restarts="$2"
  shift 2
  if [ "x${label}" == "x" ] ; then
    >&2 echo "get_restarts_of_first_container() called without a label parameter"
    return 1
  fi
  if [ "x${expected_restarts}" == "x" ] ; then
    >&2 echo "get_restarts_of_first_container() called without an expected_restarts parameter"
    return 1
  fi
  for n in `seq 1 20` ; do
    # Restarts from all pods are sumed, in case the ReplicaSet is healing.
    local restarts=$(kubectl get po -l "${label}" $@ -o json \
      | jq '.items[].status.containerStatuses[0].restartCount' \
      | awk '{s+=$1} END {printf "%.0f", s}')
    if [ ${restarts} -ge ${expected_restarts} ] ; then
      break
    fi
    sleep 3
  done
  if [ $restarts -lt $expected_restarts ] ; then
    >&2 echo "Expected there to be %{expected_restarts} restarts for pods with label ${label}, but got ${restarts}"
    return 1
  fi
  return 0
}

