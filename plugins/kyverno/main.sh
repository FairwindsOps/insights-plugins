#! /bin/bash
# Additional information on PolicyReport,ClusterPolicyReport: https://kyverno.io/docs/policy-reports/
set -e
trap 'echo "Error on Line: $LINENO"' ERR
echo "Starting kyverno"
results_file=/output/kyverno.json
report_file=/tmp/report.json
list_file=/tmp/results.json
echo '{"policyReports":[], "clusterPolicyReports":[]}' > $list_file

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
    # remove passed or skipped results to minimize payload and trigger a 'fixed' status for action items
    report_json_full=$(kubectl -n $namespace get $KIND $report -o json)
    echo $report_json_full | jq '. | del( ."results"[]? | select(."result" == "pass" or ."result" == "skip") )' > $report_file
    policy_name=$(cat $report_file | jq -r '.results[0]?.policy')

    # use lookup table to minimize control plane load
    if [[ "$policy_name" != "null" && (! "${policies[*]}" =~ "${policy_name}") ]]; then
      # determine if report refers to a policy or a clusterpolicy
      policy_type="clusterpolicy"
      kubectl get $policy_type $policy_name >/dev/null || policy_type="policy"
      # retrieve necessary metadata from the associated policy
      policy_title=$(kubectl -n $namespace get $policy_type $policy_name -o=jsonpath="{.metadata.annotations.policies\.kyverno\.io\/title}")
      policy_description=$(kubectl -n $namespace get $policy_type $policy_name -o=jsonpath="{.metadata.annotations.policies\.kyverno\.io\/description}")
      # populate the policy title and name
      jq --arg title "$policy_title" --arg description "$policy_description" '. += {policyTitle: $title, policyDescription: $description}' < $report_file | sponge $report_file

      policies+=($policy_name)
      policy_titles["$policy_name"]=$policy_title
      policy_descriptions["$policy_name"]=$policy_description
    else
      policy_title=${policy_titles[$policy_name]}
      policy_description=${policy_descriptions[$policy_name]}
      jq --arg title "$policy_title" --arg description "$policy_description" '. += {policyTitle: $title, policyDescription: $description}' < $report_file | sponge $report_file
    fi

    # append the modified policyreport to the output file
    jq --slurpfile report_json "$report_file" '.policyReports += $report_json' < $list_file | sponge $list_file
  done
done

# collect clusterpolicyreports, exit early if CRD is missing
KIND="clusterpolicyreport"
kubectl get crd "$KIND"s.wgpolicyk8s.io >/dev/null || exit 1
cpol_reports=$(kubectl get $KIND -n $namespace -o name | sed "s/$KIND\.wgpolicyk8s\.io\///g")
IFS=$'\n' cpol_reports=($cpol_reports)

count=${#cpol_reports[@]}
echo "found $count $KIND"

for report in "${cpol_reports[@]}"; do
  # remove passed or skipped results to minimize payload and trigger a 'fixed' status for action items
  report_json_full=$(kubectl -n $namespace get $KIND $report -o json)
  echo $report_json_full | jq '. | del( ."results"[]? | select(."result" == "pass" or ."result" == "skip") )' > $report_file
  policy_name=$(cat $report_file | jq -r '.results[0]?.policy')

  if [[ "$policy_name" != "null" && (! "${policies[*]}" =~ "${policy_name}") ]]; then
    policy_type="clusterpolicy"

    kubectl get $policy_type $policy_name >/dev/null || policy_type="policy -n $namespace"

    policy_title=$(kubectl get $policy_type $policy_name -o=jsonpath="{.metadata.annotations.policies\.kyverno\.io\/title}")
    policy_description=$(kubectl get $policy_type $policy_name -o=jsonpath="{.metadata.annotations.policies\.kyverno\.io\/description}")

    jq --arg title "$policy_title" --arg description "$policy_description" '. += {policyTitle: $title, policyDescription: $description}' < $report_file | sponge $report_file

    policy_titles["$policy_name"]=$policy_title
    policy_descriptions["$policy_name"]=$policy_description
  else
    policy_title=${policy_titles[$policy_name]}
    policy_description=${policy_descriptions[$policy_name]}
    jq --arg title "$policy_title" --arg description "$policy_description" '. += {policyTitle: $title, policyDescription: $description}' < $report_file | sponge $report_file
  fi

  jq --slurpfile report_json "$report_file" '.clusterPolicyReports += $report_json' < $list_file | sponge $list_file
done

cat $list_file | jq > $results_file
rm $report_file
rm $list_file
