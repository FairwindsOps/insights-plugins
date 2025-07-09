set -exo pipefail


# Loop for a fixed number of seconds, until new restarts are seen for the
# first container of all pods with the given label.
# Parameters: <label> <expected new restarts> [optional arguments to kubectl]
# <label> is a key=value form passed to the -l kubectl flag.
# <expected new restarts> is the number of NEW restarts to wait for.
# [optional arguments to kubectl] allows specifying other flags, such as a namespace.
wait_new_restarts_of_first_container() {
  local label="$1"
  local expected_new_restarts="$2"
  shift 2
  if [ "x${label}" == "x" ] ; then
    >&2 echo "get_restarts_of_first_container() called without a label parameter"
    return 1
  fi
  if [ "x${expected_new_restarts}" == "x" ] ; then
    >&2 echo "get_restarts_of_first_container() called without an expected_new_restarts parameter"
    return 1
  fi
  # Restarts from all pods are sumed, in case the ReplicaSet is healing.
  local initial_restarts=$(kubectl get po -l "${label}" $@ -o json \
    | jq '.items[].status.containerStatuses[0].restartCount' \
    | awk '{s+=$1} END {printf "%.0f", s}')
  >&2 echo "initial restarts for ${label} pods is $initial_restarts, waiting for ${expected_new_restarts} new restarts..."
  for n in `seq 1 20` ; do
    sleep 5
    # Restarts from all pods are sumed, in case the ReplicaSet is healing.
    local restarts=$(kubectl get po -l "${label}" $@ -o json \
      | jq '.items[].status.containerStatuses[0].restartCount' \
      | awk '{s+=$1} END {printf "%.0f", s}')
    >&2 echo "Restarts for ${label} pods is $restarts in loop $n"
    local new_restarts=$((${restarts} - ${initial_restarts}))
    if [ ${new_restarts} -ge ${expected_new_restarts} ] ; then
      break
    fi
  done
  if [ $new_restarts -lt $expected_new_restarts ] ; then
    >&2 echo "Expected ${expected_new_restarts} new restarts for pods with label ${label}, but got ${new_restarts} new restarts beyond ${initial_restarts} initial restarts"
echo "${new_restarts}"
    return 1
  fi
  echo "${new_restarts}"
  return 0
}

# Collect right-sizer related resources from the cluster, to a directory
# that will be collected as CI artifacts.
collect_rightsizer_debug() {
  local debug_dir="output/right-sizer-debug"
  mkdir "${debug_dir}"
  kubectl logs -l job-name=trigger-oomkill-right-sizer-test-workload -n insights-agent >"${debug_dir}/trigger-oomkill-right-sizer-test-workload.log"
  kubectl logs -l job-name=trigger-oomkill2-right-sizer-test-workload -n insights-agent >"${debug_dir}/trigger-oomkill2-right-sizer-test-workload.log"
  kubectl logs -l app=insights-agent,component=right-sizer -n insights-agent >"${debug_dir}/right-sizer-controller.log"
  kubectl describe pod -l app=right-sizer-test-workload -n insights-agent >"${debug_dir}/describe_pod_right-sizer-test-workload.log"
  kubectl get all -n insights-agent >"${debug_dir}/kubectl_get_all-insights_agent.txt"
  echo "Please see the ${debug_dir} directory in CI artifacts for additional logs."
}

cd /workspace
helm repo add fairwinds-incubator https://charts.fairwinds.com/incubator
helm repo add fairwinds-stable https://charts.fairwinds.com/stable
python3 -u test/plugins-e2e/testServer.py &> /workspace/py.log &
pyServer=$!

trap "cat /workspace/py.log && kill $pyServer" EXIT
sleep 5
insightsHost="http://$(awk '{if(/172/) print $1}' /etc/hosts):8080"
kubectl create namespace insights-agent

cat ./tags.sh
source ./tags.sh

# TODO: add some OPA checks

helm upgrade --install insights-agent fairwinds-stable/insights-agent \
  --namespace insights-agent \
  -f test/plugins-e2e/values.yaml \
  --set insights.host="$insightsHost" \
  --set insights.base64token="$(echo -n "Erehwon" | base64)" \
  --set workloads.image.tag="$workloads_tag" \
  --set rbac-reporter.image.tag="$rbacreporter_tag" \
  --set kube-bench.image.tag="$kubebench_tag" \
  --set trivy.image.tag="$trivy_tag" \
  --set opa.image.tag="$opa_tag" \
  --set right-sizer.image.tag="$rightsizer_tag" \
  --set uploader.image.tag="$uploader_tag"

sleep 5

echo "Applying right-sizer test workload and triggering the first OOM-kill."
kubectl apply -n insights-agent -f /workspace/plugins/right-sizer/e2e/testworkload.yaml
kubectl wait --for=condition=ready -l app=right-sizer-test-workload pod --timeout=60s --namespace insights-agent
# Be sure the right-sizer controller is available to see this OOM-kill.
kubectl wait --for=condition=ready -l app=insights-agent,component=right-sizer pod --timeout=60s --namespace insights-agent
kubectl create job trigger-oomkill-right-sizer-test-workload -n insights-agent --image=curlimages/curl -- curl http://right-sizer-test-workload:8080
# ifetch, 2023-02-23: disabling the kubectl wait because the OOMKill was happening before wait_new_restarts_of_first_container could run.
#kubectl wait --for=condition=complete job/trigger-oomkill-right-sizer-test-workload --timeout=40s --namespace insights-agent
# Verify the test workload has a new container restart.
rightsizer_workload_restarts=$(wait_new_restarts_of_first_container app=right-sizer-test-workload 1 -n insights-agent)
if [ ${rightsizer_workload_restarts} -ne 1 ] ; then
  echo "Got \"${rightsizer_workload_restarts}\" restarts (should be 1) after the first trigger of an OOM-kill."
  false # Fail the test.
fi

kubectl get all --namespace insights-agent
kubectl wait --for=condition=complete job/workloads --timeout=240s --namespace insights-agent
kubectl wait --for=condition=complete job/rbac-reporter --timeout=240s --namespace insights-agent
kubectl wait --for=condition=complete job/kube-bench --timeout=240s --namespace insights-agent
kubectl wait --for=condition=complete job/trivy --timeout=480s --namespace insights-agent
# TODO: enable OPA
# kubectl wait --for=condition=complete job/opa --timeout=480s --namespace insights-agent

kubectl get jobs --namespace insights-agent

echo "Testing kube-bench"
check-jsonschema --schemafile plugins/kube-bench/results.schema output/kube-bench.json || (cat output/kube-bench.json && exit 1)
echo "Testing trivy"
check-jsonschema --schemafile plugins/trivy/results.schema output/trivy.json || (cat output/trivy.json && exit 1)
echo "Testing rbac-reporter"
check-jsonschema --schemafile plugins/rbac-reporter/results.schema output/rbac-reporter.json || (cat output/rbac-reporter.json && exit 1)
echo "Testing Workloads"
check-jsonschema --schemafile plugins/workloads/results.schema output/workloads.json || (cat output/workloads.json && exit 1)
# The second right-sizer OOM-kill is triggered this late, to capitolize
# on the time it takes for other CronJob checks to complete.
# This allows the test workload to settle; avoid CrashLoopBackOff.
echo "Triggering the second OOM-kill for right-sizer test workload - memory limits will be updated by the controller."
kubectl wait --for=condition=ready -l app=right-sizer-test-workload pod --timeout=120s --namespace insights-agent
kubectl create job trigger-oomkill2-right-sizer-test-workload -n insights-agent --image=curlimages/curl -- curl http://right-sizer-test-workload:8080
kubectl wait --for=condition=complete job/trigger-oomkill2-right-sizer-test-workload --timeout=120s --namespace insights-agent
echo "Testing right-sizer"
kubectl wait --for=condition=ready -l app=right-sizer-test-workload pod --timeout=120s --namespace insights-agent
# Pull right-sizer data directly from the controller state ConfigMap,
# to obtain JSON for checking against the schema.
for n in `seq 1 18` ; do
  kubectl get configmap -n insights-agent insights-agent-right-sizer-controller-state -o jsonpath='{.data.report}' \
    > output/right-sizer.json && echo >>output/right-sizer.json
  # Wait for expected data in the right-sizer controller report.
  rightsizer_num_items=$(jq '.items |length' output/right-sizer.json)
  if [ $rightsizer_num_items -gt 0 ] ; then
    rightsizer_num_ooms=$(jq '.items[0].numOOMs' output/right-sizer.json)           
    if [ ${rightsizer_num_ooms} -eq 2 ] ; then
      echo "Got expected right-sizer report after checking $n times."
      break
    fi
  fi
  sleep 5
done
if [ $rightsizer_num_items -eq 0 ] ; then
  echo "The right-sizer controller has no report items after checking $n times."
  cat output/right-sizer.json
  collect_rightsizer_debug
  # right-sizer is a soft fail for now, see FWI-2806
  echo right-sizer is a temporary soft-fail for now...
  #false # Fail the test.
fi
#if [ $rightsizer_num_ooms -ne 2 ] ; then
#  echo "The right-sizer report item has \"${rightsizer_num_ooms}\" numOOMs instead of 2, after checking $n times."
#  cat output/right-sizer.json
#  collect_rightsizer_debug
#  false # Fail the test.
#fi
#jsonschema -i output/right-sizer.json plugins/right-sizer/results.schema || (cat output/right-sizer.json && exit 1)
# The jq select() avoids it creating the fields being replaced,
# in the top object of the resulting JSON.
#jq '(..|select(has("firstOOM"))?) += {firstOOM: "dummyvalue", lastOOM: "dummyvalue", "resourceGeneration": 0}' output/right-sizer.json >output/right-sizer_normalized.json
# Diffing plugin output with wanted; known JSON,
# allows matching whether fields like `endingMemory` have changed.
#diff plugins/right-sizer/e2e/want.json output/right-sizer_normalized.json
#if [ $? -gt 0 ] ; then
#  echo "The normalized right-sizer output failed to match wanted output (the diff is shown above)."
#  echo "** Expected Normalized Output **" && cat plugins/right-sizer/e2e/want.json
#  echo "** Got Normalized Output **" && cat output/right-sizer_normalized.json
#fi

echo "Showing content of output sub-directory:"
ls output
