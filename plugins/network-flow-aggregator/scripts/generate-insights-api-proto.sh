#! /usr/bin/env bash
set -eo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

PROTOC_GEN_GO_VERSION=v1.36.11
PROTOC_GEN_GO_GRPC_VERSION=v1.6.2
GO_MODULE="github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator"
GO_PACKAGE="${GO_MODULE}/pkg/insights/v1;insightsv1"

FAIRWINDS_INSIGHTS_PATH=""

usage() {
  cat <<EOF
Usage: $(basename "$0") --fairwinds-insights-path <path>

Copy the Insights network-flow API protos from a local fairwinds-insights checkout
and regenerate Go stubs under pkg/insights/v1.

Example:
  $(basename "$0") --fairwinds-insights-path ../../../Insights
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --fairwinds-insights-path)
      FAIRWINDS_INSIGHTS_PATH="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ -z "${FAIRWINDS_INSIGHTS_PATH}" ]]; then
  echo "error: --fairwinds-insights-path is required" >&2
  usage >&2
  exit 1
fi

FAIRWINDS_INSIGHTS_PATH="$(cd "${FAIRWINDS_INSIGHTS_PATH}" && pwd)"
SOURCE_PROTO_DIR="${FAIRWINDS_INSIGHTS_PATH}/api/proto/api/v1"
DEST_PROTO_DIR="${ROOT}/proto/insights/api/v1"

if [[ ! -d "${SOURCE_PROTO_DIR}" ]]; then
  echo "error: expected protos at ${SOURCE_PROTO_DIR}" >&2
  exit 1
fi

if ! command -v protoc >/dev/null 2>&1; then
  echo "protoc is required; install Protocol Buffers (e.g. brew install protobuf)" >&2
  exit 1
fi

go install google.golang.org/protobuf/cmd/protoc-gen-go@"${PROTOC_GEN_GO_VERSION}"
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@"${PROTOC_GEN_GO_GRPC_VERSION}"

protoc --version
protoc-gen-go --version
protoc-gen-go-grpc --version

mkdir -p "${DEST_PROTO_DIR}"

for proto in types.proto ingest.proto; do
  src="${SOURCE_PROTO_DIR}/${proto}"
  if [[ ! -f "${src}" ]]; then
    echo "error: missing ${src}" >&2
    exit 1
  fi
  dest="${DEST_PROTO_DIR}/${proto}"
  cp "${src}" "${dest}"
  sed -i.bak "s|^option go_package = .*|option go_package = \"${GO_PACKAGE}\";|" "${dest}"
  rm -f "${dest}.bak"
done

protoc \
  --go_out=. \
  --go_opt=module="${GO_MODULE}" \
  --go-grpc_out=. \
  --go-grpc_opt=module="${GO_MODULE}" \
  -I proto/insights \
  proto/insights/api/v1/types.proto \
  proto/insights/api/v1/ingest.proto

gofmt -w ./pkg/insights/v1/*.go

HAS_CHANGE=$(git status -s ./pkg/insights/v1 ./proto/insights)
if [ -n "${HAS_CHANGE}" ]; then
  echo "Insights API proto generation changes detected:"
  echo "${HAS_CHANGE}"
  git --no-pager diff ./pkg/insights/v1 ./proto/insights
  if [[ -n "${CIRCLECI}" ]]; then
    echo "Regenerate from Insights API protos (./scripts/generate-insights-api-proto.sh), commit pkg/insights/v1 and proto/insights, and push."
    exit 1
  fi
else
  echo "Insights API proto generation is up to date"
fi
