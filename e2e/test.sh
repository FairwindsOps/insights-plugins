set -eo pipefail
cd /workspace

helm repo add fairwinds-incubator https://charts.fairwinds.com/incubator
helm repo add fairwinds-stable https://charts.fairwinds.com/stable
helm repo add stable https://kubernetes-charts.storage.googleapis.com
python3 e2e/testServer.py &
pyServer=$!
insightsHost="http://$(awk 'END{print $1}' /etc/hosts)"
helm upgrade --install insights-agent fairwinds-stable/insights-agent \
  --namespace insights-agent \
  -f e2e/values.yaml \
  --set insights.host="$insightsHost" \
  --set insights.base64token="$(echo -n "Erehwon" | base64)" \
  --set workloads.image.tag="$CI_BRANCH" \
  --set kubesec.image.tag="$CI_BRANCH" \
  --set rbacreporter.image.tag="$CI_BRANCH" \
  --set kubebench.image.tag="$CI_BRANCH" \
  --set trivy.image.tag="$CI_BRANCH" \
  --set uploader.image.tag="$CI_BRANCH" 


kubectl wait --for=condition=complete job/workloads --timeout=120s --namespace insights-agent
kubectl wait --for=condition=complete job/kubesec --timeout=120s --namespace insights-agent
kubectl wait --for=condition=complete job/rbac-reporter --timeout=120s --namespace insights-agent
kubectl wait --for=condition=complete job/kube-bench --timeout=120s --namespace insights-agent
kubectl wait --for=condition=complete job/trivy --timeout=480s --namespace insights-agent

kubectl get jobs --namespace insights-agent

kill pyServer
ls output
