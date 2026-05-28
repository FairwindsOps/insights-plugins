#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
K8S="${ROOT}/k8s"

echo "Applying smoke fixtures to kind..."
kubectl apply -f "${K8S}/namespace.yaml"
kubectl apply -f "${K8S}/verified.yaml"
kubectl apply -f "${K8S}/untrusted.yaml"
kubectl apply -f "${K8S}/unsigned.yaml"

echo "Waiting for deployments..."
kubectl -n image-trust-smoke rollout status deployment/verified --timeout=180s
kubectl -n image-trust-smoke rollout status deployment/untrusted --timeout=180s
kubectl -n image-trust-smoke rollout status deployment/unsigned --timeout=180s

echo ""
echo "Runtime image IDs (expect @sha256: for verification):"
kubectl -n image-trust-smoke get pods -o jsonpath='{range .items[*]}{.metadata.labels.app}{"\t"}{.status.containerStatuses[0].imageID}{"\n"}{end}'
