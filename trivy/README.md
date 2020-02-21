## Running locally
You can run the trivy plugin against e.g. a KIND cluster by running:
```
docker build -t fw-trivy .
docker run --network host --privileged \
  -e "KUBECONFIG=/root/.kubeconfig" \
  -e "MAX_CONCURRENT_SCANS=5" \
  -v `pwd`/output:/output \
  -v $HOME/.kube/kind-config-kind:/root/.kubeconfig \
  fw-trivy
```
