#! /bin/bash
set -eo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

PROTOC_GEN_GO_VERSION=v1.36.11
PROTOC_GEN_GO_GRPC_VERSION=v1.6.2

if ! command -v protoc >/dev/null 2>&1; then
  echo "protoc is required; install Protocol Buffers (e.g. brew install protobuf)" >&2
  exit 1
fi

go install google.golang.org/protobuf/cmd/protoc-gen-go@"${PROTOC_GEN_GO_VERSION}"
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@"${PROTOC_GEN_GO_GRPC_VERSION}"

protoc --version
protoc-gen-go --version
protoc-gen-go-grpc --version

protoc \
  --go_out=. \
  --go_opt=module=github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator \
  --go-grpc_out=. \
  --go-grpc_opt=module=github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator \
  -I proto \
  proto/aggregator/v1/types.proto \
  proto/aggregator/v1/ingest.proto

gofmt -w ./pkg/aggregator/v1/*.go

HAS_CHANGE=$(git status -s ./pkg/aggregator/v1 ./proto)
if [ -n "${HAS_CHANGE}" ]; then
  echo "Proto generation changes detected:"
  echo "${HAS_CHANGE}"
  git --no-pager diff ./pkg/aggregator/v1 ./proto
  if [[ -n "${CIRCLECI}" ]]; then
    echo "Regenerate from proto (./scripts/generate-proto.sh), commit pkg/aggregator/v1, and push."
    exit 1
  fi
else
  echo "Proto generation is up to date"
fi
