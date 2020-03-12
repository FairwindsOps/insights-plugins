set -eo pipefail
cd /workspace

helm repo add fairwinds-incubator https://charts.fairwinds.com/incubator
helm repo add fairwinds-stable https://charts.fairwinds.com/stable
helm repo add stable https://kubernetes-charts.storage.googleapis.com
python3 -u e2e/testServer.py &> /workspace/py.log &
pyServer=$!

trap "cat /workspace/py.log && kill $pyServer" EXIT
sleep 5
insightsHost="http://$(awk 'END{print $1}' /etc/hosts):8080"
kubectl create namespace insights-agent
helm upgrade --install insights-agent fairwinds-stable/insights-agent \
  --namespace insights-agent \
  -f e2e/values.yaml \
  --set insights.host="$insightsHost" \
  --set insights.base64token="$(echo -n "Erehwon" | base64)" \
  --set workloads.image.tag="$CI_BRANCH" \
  --set rbacreporter.image.tag="$CI_BRANCH" \
  --set kubebench.image.tag="$CI_BRANCH" \
  --set trivy.image.tag="$CI_BRANCH" \
  --set uploader.image.tag="$CI_BRANCH" 
sleep 5
kubectl get all --namespace insights-agent
#kubectl wait --for=condition=complete job/workloads --timeout=120s --namespace insights-agent
kubectl wait --for=condition=complete job/rbac-reporter --timeout=120s --namespace insights-agent
kubectl wait --for=condition=complete job/kube-bench --timeout=120s --namespace insights-agent
kubectl wait --for=condition=complete job/trivy --timeout=480s --namespace insights-agent

kubectl get jobs --namespace insights-agent
jsonschema -i output/kube-bench.json kube-bench/results.schema
jsonschema -i output/trivy.json trivy/results.schema
jsonschema -i output/rbac-reporter.json rbac-reporter/results.schema
ls output
