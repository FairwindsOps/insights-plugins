#! /bin/bash
# Additional information on PolicyReport,ClusterPolicyReport: https://kyverno.io/docs/policy-reports/
# NOTE: this may only work for Kyverno versions v1.11 and up
# See https://github.com/kyverno/kyverno/pull/8426 for more information
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

echo "Retrieving "
namespaces=$(kubectl get namespaces -o name | sed 's/namespace\///g')
IFS=$'\n' namespaces=("$namespaces")

declare -a policies
declare -a policy_titles
declare -a policy_descriptions

# if there are no policyreports whatsoever, exit
reports=$(kubectl get policyreport,clusterpolicyreport --all-namespaces)
if [[ -z "$reports" ]]; then
	echo "no policyreport,clusterpolicyreport found in cluster. quitting..."
  cat $list_file | jq > $results_file
  rm $list_file
	exit 0
fi

# get policyreport totals for diagnostics
for namespace in "${namespaces[@]}"; do
	reports_for_count=$(kubectl get policyreport,clusterpolicyreport -n "$namespace" -o name)
	IFS=$'\n' reports_for_count=("$reports_for_count")
	count=${#reports_for_count[@]}
	echo "found $count policyreport,clusterpolicyreport for namespace $namespace"
done

echo "" > $report_file
# retrieve all policyreports in all namespaces, only include those with nonzero numbers of:
# 'fail', 'warn', or 'error'
reports=$(kubectl get policyreport,clusterpolicyreport -A -ojson | jq '.items | map(select(.summary.fail > 0 or .summary.warn > 0 or .summary.error > 0))')
echo "$reports" | jq -c '.[]' | while read -r r; do
	# remove 'pass' or 'skip' results from the report, these are not needed for generating
	# Insights action items, write to the report file
	echo "$r" | jq '. | del( ."results"[]? | select(."result" == "pass" or ."result" == "skip") )'  > $report_file
	policy_name=$(cat $report_file | jq -r '.results[0]?.policy')
	namespace=$(cat $report_file | jq -r '.metadata.namespace')
	kind=$(cat $report_file | jq -r '.kind')

  policy_name_hash= echo "$(echo $policy_name | md5sum)"
	# use lookup table to minimize control plane load
  if [[ "$policy_name" != "null" && (! "${policies[*]}" =~ ${policy_name}) ]]; then
    # determine if report refers to a policy or a clusterpolicy
    policy_type="clusterpolicy"
    kubectl get $policy_type "$policy_name" >/dev/null || policy_type="policy"
    # retrieve necessary metadata from the associated policy
    policy_title=$(kubectl -n "$namespace" get "$policy_type" "$policy_name" -o=jsonpath="{.metadata.annotations.policies\.kyverno\.io\/title}")
    policy_description=$(kubectl -n "$namespace" get "$policy_type" "$policy_name" -o=jsonpath="{.metadata.annotations.policies\.kyverno\.io\/description}")
    # populate the policy title and name
    jq --arg title "$policy_title" --arg description "$policy_description" '. += {policyTitle: $title, policyDescription: $description}' < $report_file | sponge $report_file

    policies+=("$policy_name")
    policy_titles["$policy_name_hash"]=$policy_title
    policy_descriptions["$policy_name_hash"]=$policy_description
  else
    policy_title=${policy_titles["$policy_name_hash"]}
    policy_description=${policy_descriptions["$policy_name_hash"]}
    jq --arg title "$policy_title" --arg description "$policy_description" '. += {policyTitle: $title, policyDescription: $description}' < $report_file | sponge $report_file
  fi

  # append the modified policyreport to the output file
  if [[ "$kind" == "PolicyReport" ]]; then
    	jq --slurpfile report_json "$report_file" '.policyReports += $report_json' < $list_file | sponge $list_file
	fi

	if [[ "$kind" == "ClusterPolicyReport" ]]; then
    	jq --slurpfile report_json "$report_file" '.clusterPolicyReports += $report_json' < $list_file | sponge $list_file
	fi
done

cat $list_file | jq > $results_file
rm $report_file
rm $list_file
