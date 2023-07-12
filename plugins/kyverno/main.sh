#! /bin/bash
set -e
trap 'echo "Error on Line: $LINENO"' ERR
echo "Starting kyverno"
results_file=/output/kyverno.json

json='{"policyReports":[], "clusterPolicyReports":[]}'

KIND="policyreport"

echo "Retrieving namespaces"
namespaces=$(kubectl get namespaces -o name)
IFS=$'\n' namespaces=($namespaces)
for ns_idx in "${!namespaces[@]}"; do
  namespace=${namespaces[$ns_idx]#namespace\/}
  count=$(kubectl get $KIND -n $namespace -o name | wc -l)

  echo "found $count $KIND for namespace $namespace"

  json="$(jq --argjson list "$(kubectl get $KIND -n $namespace -o json)" '.policyReports += [$list.items]' <<< "$json")"
done

KIND="clusterpolicyreport"

count=$(kubectl get $KIND -o name | wc -l)
echo "found $count $KIND"
json="$(jq --argjson list "$(kubectl get $KIND -o json)" '.clusterPolicyReports += [$list.items]' <<< "$json")"

echo $json | jq > $results_file
