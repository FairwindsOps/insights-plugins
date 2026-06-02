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
   [[ "$(kubectl -n image-trust-smoke get deploy --no-headers 2>/dev/null | wc -l | tr -d ' ')" -lt 4 ]]; then
  echo "Smoke workloads not found or incomplete; running setup.sh..."
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

if [[ "${IMAGE_TRUST_MODES:-}" == *cosign-key* ]]; then
  if [[ -z "${IMAGE_TRUST_PUBLIC_KEY_REFS:-}" && -z "${IMAGE_TRUST_PUBLIC_KEY_PATHS:-}" && -z "${IMAGE_TRUST_PUBLIC_KEY_DIR:-}" ]]; then
    echo "cosign-key mode requires IMAGE_TRUST_PUBLIC_KEY_REFS, IMAGE_TRUST_PUBLIC_KEY_PATHS, or IMAGE_TRUST_PUBLIC_KEY_DIR" >&2
    exit 1
  fi
fi

KEY_DIR=""
if [[ -n "${IMAGE_TRUST_PUBLIC_KEY_DIR:-}" ]]; then
  KEY_DIR="$(cd "${IMAGE_TRUST_PUBLIC_KEY_DIR}" && pwd)"
  if [[ "${IMAGE_TRUST_MODES:-}" == *cosign-key* ]]; then
    if ! compgen -G "${KEY_DIR}/*.pub" >/dev/null && ! compgen -G "${KEY_DIR}/*.pem" >/dev/null; then
      echo "cosign-key mode requires a .pub or .pem file in ${KEY_DIR}" >&2
      exit 1
    fi
  fi
  export IMAGE_TRUST_PUBLIC_KEY_DIR=/etc/image-trust/keys
fi

DOCKER_ENV=(
  -e KUBECONFIG=/kube/config
  -e IMAGE_TRUST_MODES
  -e IMAGE_TRUST_TRUSTED_ISSUERS
  -e IMAGE_TRUST_TRUSTED_SUBJECTS
  -e IMAGE_TRUST_TRUSTED_SUBJECT_REGEXPS
  -e IMAGE_TRUST_PUBLIC_KEY_REFS
  -e IMAGE_TRUST_PUBLIC_KEY_PATHS
  -e IMAGE_TRUST_PUBLIC_KEY_DIR
  -e IMAGE_TRUST_IGNORE_TLOG
  -e IMAGE_TRUST_NAMESPACE_ALLOWLIST
  -e MAX_CONCURRENT_SCANS
  -e IMAGE_VERIFY_TIMEOUT_SECONDS
)
DOCKER_MOUNTS=(
  -v "${KUBECONFIG_PATH}:/kube/config:ro"
  -v "${OUTPUT_DIR}:/output"
)
if [[ -n "${KEY_DIR}" ]]; then
  DOCKER_MOUNTS+=(-v "${KEY_DIR}:/etc/image-trust/keys:ro")
fi

echo "Running plugin (report -> ${OUTPUT_DIR}/image-trust.json)..."
docker run --rm --network host \
  --user 0:0 \
  --entrypoint /usr/local/bin/image-trust \
  "${DOCKER_ENV[@]}" \
  "${DOCKER_MOUNTS[@]}" \
  "${IMAGE_NAME}"

echo "Done. Report: ${OUTPUT_DIR}/image-trust.json"
