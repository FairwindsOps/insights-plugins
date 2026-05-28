#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PLUGIN_ROOT="$(cd "${ROOT}/.." && pwd)"
OUTPUT_DIR="${PLUGIN_ROOT}/output"
IMAGE_NAME="${IMAGE_NAME:-fw-image-trust:smoke}"

if [[ -f "${ROOT}/env" ]]; then
  # shellcheck source=/dev/null
  source "${ROOT}/env"
elif [[ -f "${ROOT}/env.example" ]]; then
  # shellcheck source=/dev/null
  source "${ROOT}/env.example"
else
  echo "missing ${ROOT}/env or env.example" >&2
  exit 1
fi

: "${IMAGE_TRUST_TRUSTED_ISSUERS:?IMAGE_TRUST_TRUSTED_ISSUERS is required}"
: "${IMAGE_TRUST_TRUSTED_SUBJECTS:?IMAGE_TRUST_TRUSTED_SUBJECTS is required}"

if ! kubectl get namespace image-trust-smoke >/dev/null 2>&1 || \
   [[ "$(kubectl -n image-trust-smoke get deploy --no-headers 2>/dev/null | wc -l | tr -d ' ')" == "0" ]]; then
  echo "Smoke workloads not found; running setup.sh..."
  "${ROOT}/setup.sh"
fi

mkdir -p "${OUTPUT_DIR}"

echo "Building image-trust binary..."
(
  cd "${PLUGIN_ROOT}"
  CGO_ENABLED=0 GOOS=linux GOARCH="$(go env GOARCH)" go build -o image-trust ./cmd
)

echo "Building Docker image ${IMAGE_NAME}..."
docker build -t "${IMAGE_NAME}" -f "${PLUGIN_ROOT}/Dockerfile" "${PLUGIN_ROOT}"

KUBECONFIG_PATH="${KUBECONFIG:-${HOME}/.kube/config}"
if [[ ! -f "${KUBECONFIG_PATH}" ]]; then
  echo "kubeconfig not found at ${KUBECONFIG_PATH}" >&2
  exit 1
fi

echo "Running plugin (report -> ${OUTPUT_DIR}/image-trust.json)..."
docker run --rm --network host \
  --user 0:0 \
  --entrypoint /usr/local/bin/image-trust \
  -e KUBECONFIG=/kube/config \
  -e IMAGE_TRUST_TRUSTED_ISSUERS \
  -e IMAGE_TRUST_TRUSTED_SUBJECTS \
  -e IMAGE_TRUST_TRUSTED_SUBJECT_REGEXPS \
  -e NAMESPACE_ALLOWLIST \
  -e MAX_CONCURRENT_SCANS \
  -e IMAGE_VERIFY_TIMEOUT_SECONDS \
  -v "${KUBECONFIG_PATH}:/kube/config:ro" \
  -v "${OUTPUT_DIR}:/output" \
  "${IMAGE_NAME}"

echo "Done. Report: ${OUTPUT_DIR}/image-trust.json"
