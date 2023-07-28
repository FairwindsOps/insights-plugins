#! /bin/bash
# Additional information on PolicyReport,ClusterPolicyReport: https://kyverno.io/docs/policy-reports/
set -e
trap 'echo "Error on Line: $LINENO"' ERR
echo "Starting kyverno"
results_file=/output/kyverno.json

json='{"policyReports":[], "clusterPolicyReports":[]}'

# check for policy and clusterpolicy CRDs
kubectl get crd policies.kyverno.io >/dev/null || exit 1
kubectl get crd clusterpolicies.kyverno.io >/dev/null || exit 1

# collect policyreports, exit early if CRD is missing
KIND="policyreport"
kubectl get crd "$KIND"s.wgpolicyk8s.io >/dev/null || exit 1
echo "Retrieving namespaces"
namespaces=$(kubectl get namespaces -o name | sed 's/namespace\///g')
IFS=$'\n' namespaces=($namespaces)

declare -a policies
declare -a policy_titles
declare -a policy_descriptions

for namespace in "${namespaces[@]}"; do
  reports=$(kubectl get $KIND -n $namespace -o name | sed "s/$KIND\.wgpolicyk8s\.io\///g")
  IFS=$'\n' reports=($reports)

  count=${#reports[@]}
  echo "found $count $KIND for namespace $namespace"

  for report in "${reports[@]}"; do
    report_json=$(kubectl get $KIND $report -o json | jq '.results |= [.[-1]]')
    policy_name=$(echo $report_json | jq -r '.results[0].policy')

    # use lookup table to minimize control plane load
    if [[ ! "${policies[*]}" =~ "${policy_name}" ]]; then
      # determine if report refers to a policy or a clusterpolicy
      policy_type_expression="clusterpolicy"
      kubectl get $policy_type_expression $policy_name >/dev/null || policy_type_expression="policy -n $namespace"
      # retrieve necessary metadata from the associated policy
      policy_title=$(kubectl get $policy_type_expression $policy_name -o=jsonpath="{.metadata.annotations.policies\.kyverno\.io\/title}")
      policy_description=$(kubectl get $policy_type_expression $policy_name -o=jsonpath="{.metadata.annotations.policies\.kyverno\.io\/description}")
      # populate the policy title and name
      report_json="$(jq --arg title "$policy_title" --arg description "$policy_description" '. += {policyTitle: $title, policyDescription: $description}' <<< "$report_json")"

      policies+=($policy_name)
      policy_titles["$policy_name"]=$policy_title
      policy_descriptions["$policy_name"]=$policy_description
    else
      policy_title=${policy_titles[$policy_name]}
      policy_description=${policy_descriptions[$policy_name]}
      report_json="$(jq --arg title "$policy_title" --arg description "$policy_description" '. += {policyTitle: $title, policyDescription: $description}' <<< "$report_json")"
    fi

    # append the modified policyreport to the output file
    json="$(jq --argjson report_json "$report_json" '.policyReports += [$report_json]' <<< "$json")"
  done
done

# collect clusterpolicyreports, exit early if CRD is missing
KIND="clusterpolicyreport"
kubectl get crd "$KIND"s.wgpolicyk8s.io >/dev/null || exit 1
reports=$(kubectl get $KIND -n $namespace -o name | sed "s/$KIND\.wgpolicyk8s\.io\///g")
IFS=$'\n' reports=($reports)

count=${#reports[@]}
echo "found $count $KIND"

for report in "${reports[@]}"; do
  report_json=$(kubectl get $KIND $report -o json | jq '.results |= [.[-1]]')
  policy_name=$(echo $report_json | jq -r '.results[0].policy')

  if [[ ! "${policies[*]}" =~ "${policy_name}" ]]; then
    policy_type_expression="clusterpolicy"

    kubectl get $policy_type_expression $policy_name >/dev/null || policy_type_expression="policy -n $namespace"

    policy_title=$(kubectl get $policy_type_expression $policy_name -o=jsonpath="{.metadata.annotations.policies\.kyverno\.io\/title}")
    policy_description=$(kubectl get $policy_type_expression $policy_name -o=jsonpath="{.metadata.annotations.policies\.kyverno\.io\/description}")

    report_json="$(jq --arg title "$policy_title" --arg description "$policy_description" '. += {policyTitle: $title, policyDescription: $description}' <<< "$report_json")"

    policy_titles["$policy_name"]=$policy_title
    policy_descriptions["$policy_name"]=$policy_description
  else
    policy_title=${policy_titles[$policy_name]}
    policy_description=${policy_descriptions[$policy_name]}
    report_json="$(jq --arg title "$policy_title" --arg description "$policy_description" '. += {policyTitle: $title, policyDescription: $description}' <<< "$report_json")"
  fi

  json="$(jq --argjson report_json "$report_json" '.clusterPolicyReports += [$report_json]' <<< "$json")"
done

echo $json | jq > $results_file
