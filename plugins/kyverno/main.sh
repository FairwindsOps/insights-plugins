#! /bin/bash
set -e
trap 'echo "Error on Line: $LINENO"' ERR
echo "Starting kyverno"
results_file=/output/kyverno.json

json='{"policyReports":[], "clusterPolicyReports":[]}'

# collect policyreports, exit early if CRD is missing
KIND="policyreport"
kubectl get crd "$KIND"s.wgpolicyk8s.io >/dev/null || exit 1
echo "Retrieving namespaces"
namespaces=$(kubectl get namespaces -o name | sed 's/namespace\///g')
IFS=$'\n' namespaces=($namespaces)
for namespace in "${namespaces[@]}"; do
  reports=$(kubectl get $KIND -n $namespace -o name | sed "s/$KIND\.wgpolicyk8s\.io\///g")
  IFS=$'\n' reports=($reports)

  count=${#reports[@]}
  echo "found $count $KIND for namespace $namespace"

  for report in "${reports[@]}"; do
    # to avoid overhead, collect only the last (most recent result) from the policyreport
    report_json=$(kubectl get $KIND $report -n $namespace -o json | jq '.results |= [.[-1]]')
    json="$(jq --argjson report_json "$report_json" '.policyReports += [$report_json]' <<< "$json")"
  done
done

# collect clusterpolicyreports, exit early if CRD is missing
KIND="clusterpolicyreport"
kubectl get crd "$KIND"s.wgpolicyk8s.io >/dev/null || exit 1
reports=$(kubectl get $KIND -n $namespace -o name | sed "s/$KIND\.wgpolicyk8s\.io\///g")
IFS=$'\n' reports=($reports)

count=${#reports[@]}
echo "found $count $KIND for namespace $namespace"

for report in "${reports[@]}"; do
  report_json=$(kubectl get $KIND $report -n $namespace -o json | jq '.results |= [.[-1]]')
  json="$(jq --argjson report_json "$report_json" '.clusterPolicyReports += [$report_json]' <<< "$json")"
done

echo $json | jq > $results_file
