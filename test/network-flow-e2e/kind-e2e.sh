#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

IG_VERSION="${IG_VERSION:-v0.52.0}"
KIND_CLUSTER="${KIND_CLUSTER:-network-flow}"
KIND_CONFIG="${KIND_CONFIG:-test/network-flow-e2e/deploy/e2e/kind-config.yaml}"
AGENT_IMAGE="${AGENT_IMAGE:-fw-network-flow:local}"
AGGREGATOR_IMAGE="${AGGREGATOR_IMAGE:-fw-network-flow-aggregator:local}"
DEPLOY_DIR="${DEPLOY_DIR:-test/network-flow-e2e/deploy/e2e}"
PF_PID=""

usage() {
  cat <<EOF
Usage: $(basename "$0") [up|down|load|deploy|traffic|verify|all]

  up       Create kind cluster (if missing)
  down     Delete kind cluster
  load     Build and load local images into kind nodes
  deploy   Apply e2e manifests
  traffic  Run demo-traffic job
  verify   Check servicemap for demo-client -> demo-server edge
  all      up + load + deploy + wait + traffic + verify
EOF
}

docker_build() {
  local arch tmpdir
  arch="$(go env GOARCH)"
  tmpdir="$(mktemp -d)"
  trap "rm -rf '${tmpdir}'" RETURN

  echo "Building network-flow binaries (${arch})..."
  CGO_ENABLED=0 GOOS=linux GOARCH="${arch}" go build -o "${tmpdir}/network-flow" ./plugins/network-flow/pkg
  CGO_ENABLED=0 GOOS=linux GOARCH="${arch}" go build -o "${tmpdir}/entrypoint" ./plugins/network-flow/cmd/entrypoint
  cp plugins/network-flow/Dockerfile "${tmpdir}/"

  echo "Building network-flow image (${IG_VERSION})..."
  docker build -f "${tmpdir}/Dockerfile" \
    --build-arg IG_VERSION="${IG_VERSION}" \
    -t "${AGENT_IMAGE}" \
    "${tmpdir}"

  echo "Building network-flow-aggregator binary (${arch})..."
  CGO_ENABLED=0 GOOS=linux GOARCH="${arch}" go build -o "${tmpdir}/network-flow-aggregator" ./plugins/network-flow-aggregator/pkg
  cp plugins/network-flow-aggregator/Dockerfile "${tmpdir}/"

  echo "Building network-flow-aggregator image..."
  docker build -f "${tmpdir}/Dockerfile" \
    -t "${AGGREGATOR_IMAGE}" \
    "${tmpdir}"
}

kind_up() {
  if kind get clusters 2>/dev/null | grep -qx "$KIND_CLUSTER"; then
    echo "kind cluster '$KIND_CLUSTER' already exists"
    return
  fi
  kind create cluster --name "$KIND_CLUSTER" --config "$KIND_CONFIG"
}

kind_down() {
  kind delete cluster --name "$KIND_CLUSTER"
}

kind_load() {
  docker_build
  kind load docker-image "$AGENT_IMAGE" "$AGGREGATOR_IMAGE" --name "$KIND_CLUSTER"
  echo "Loaded images into kind cluster '$KIND_CLUSTER':"
  echo "  $AGENT_IMAGE"
  echo "  $AGGREGATOR_IMAGE"
}

kind_deploy() {
  kubectl apply -k "$DEPLOY_DIR"
  kubectl -n insights rollout status deployment/network-flow-aggregator --timeout=120s
  kubectl -n insights rollout status daemonset/network-flow --timeout=180s
  kubectl -n insights rollout status deployment/demo-server --timeout=60s
  kubectl -n insights wait --for=condition=ready pod -l app.kubernetes.io/name=network-flow-aggregator --timeout=120s
}

kind_traffic() {
  kubectl -n insights delete job demo-traffic --ignore-not-found
  kubectl apply -k "$DEPLOY_DIR"
  kubectl -n insights wait --for=condition=complete job/demo-traffic --timeout=90s
  sleep 5
}

kind_verify() {
  local edges demo_edges
  kubectl -n insights port-forward svc/network-flow-aggregator 18080:8080 >/dev/null 2>&1 &
  PF_PID=$!
  trap '[[ -n "${PF_PID:-}" ]] && kill "${PF_PID}" 2>/dev/null || true' EXIT

  for _ in $(seq 1 30); do
    if curl -sf http://127.0.0.1:18080/healthz >/dev/null 2>&1; then
      break
    fi
    sleep 1
  done

  edges="$(curl -sf http://127.0.0.1:18080/api/v1/servicemap | jq -r '
    .edges[]?
    | . as $e
    | ($e.key.SrcNamespace + "/" + $e.key.SrcWorkloadKind + "/" + $e.key.SrcWorkloadName) as $src
    | (if ($e.key.DstKind // "") != "" then
         $e.key.DstNamespace + "/" + $e.key.DstKind + "/" + $e.key.DstName
       else
         ($e.dst_addr // $e.key.DstName)
       end) as $dst
    | "\($src) -> \($dst):\($e.key.DstPort)  count=\($e.count)  sent=\($e.bytes_sent)  rcvd=\($e.bytes_received)  pod=\($e.src_pod // "-")"
  ' || true)"

  demo_edges="$(curl -sf http://127.0.0.1:18080/api/v1/servicemap | jq -r '
    [.edges[]?
      | select(
          .key.SrcWorkloadName == "demo-traffic"
          or (.key.DstName == "demo-server" and (.key.DstKind // "") != "")
        )
      | . as $e
      | ($e.key.SrcNamespace + "/" + $e.key.SrcWorkloadKind + "/" + $e.key.SrcWorkloadName) as $src
      | (if ($e.key.DstKind // "") != "" then
           $e.key.DstNamespace + "/" + $e.key.DstKind + "/" + $e.key.DstName
         else
           ($e.dst_addr // $e.key.DstName)
         end) as $dst
      | {
          line: "\($src) -> \($dst):\($e.key.DstPort)  count=\($e.count)  sent=\($e.bytes_sent)  rcvd=\($e.bytes_received)  pod=\($e.src_pod // "-")",
          priority: (if $e.key.SrcWorkloadName == "demo-traffic" then 0 else 1 end)
        }
    ]
    | sort_by(.priority)
    | .[].line
  ' || true)"

  if [[ -n "$demo_edges" ]]; then
    echo "demo workload edges:"
    echo "$demo_edges"
  else
    echo "servicemap edges (all):"
    echo "$edges"
  fi

  if echo "$demo_edges" | grep -q 'insights/Job/demo-traffic.*insights/Service/demo-server'; then
    echo "e2e verify: found demo-traffic -> demo-server edge with top-controller enrichment"
    return 0
  fi

  if echo "$demo_edges" | grep -qi 'demo-server'; then
    echo "e2e verify: found demo-server edge(s), but missing Job/demo-traffic -> Service/demo-server"
    return 1
  fi

  echo "e2e verify: no demo edges yet; sample raw edges:"
  curl -sf http://127.0.0.1:18080/api/v1/servicemap | jq '[.edges[] | select(.key.SrcWorkloadName == "demo-traffic" or .key.DstName == "demo-server")][0:3] // .edges[0:3]'
  return 1
}

cmd="${1:-all}"
case "$cmd" in
  up) kind_up ;;
  down) kind_down ;;
  load) kind_load ;;
  deploy) kind_deploy ;;
  traffic) kind_traffic ;;
  verify) kind_verify ;;
  all)
    kind_up
    kind_load
    kind_deploy
    kind_traffic
    kind_verify
    ;;
  -h|--help|help) usage ;;
  *) echo "unknown command: $cmd" >&2; usage; exit 1 ;;
esac
