#! /bin/bash
# file: insights_collect.sh
# this is a simple bash script that collects job and pod data in the insights-agent namespace,
# to assist customers with debugging of insights
set -e
# redirect output to a file for uploading later
exec >> insights_collect_$(date +%s).log
exec 2>&1
NAMESPACE=insights-agent
trap 'echo "Error on Line: $LINENO"' ERR

echo "Collecting insights diagnostic information in namespace ${NAMESPACE}"

kubectl -n ${NAMESPACE} get pods
kubectl -n ${NAMESPACE} get jobs

pods=$(kubectl -n ${NAMESPACE} get pods | tail -n +2 | awk '{print $1}')
jobs=$(kubectl -n ${NAMESPACE} get jobs | tail -n +2 | awk '{print $1}')

echo "found ${#pods[@]} pods in ${NAMESPACE}"
echo "found ${#jobs[@]} jobs in ${NAMESPACE}"

for pod_name in $pods; do
    echo
    echo "========================"
    echo "pod_name=${pod_name}"
    echo "========================"
    echo
    kubectl -n ${NAMESPACE} describe pod ${pod_name}
    echo
    kubectl -n ${NAMESPACE} logs -f ${pod_name} --all-containers=true
done

for job_name in $jobs; do
    echo
    echo "========================"
    echo "job_name=${job_name}"
    echo "========================"
    echo
    kubectl -n ${NAMESPACE} describe job ${job_name}
    echo
done

