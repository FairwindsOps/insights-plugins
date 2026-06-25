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
AGENT_ONLY_DEPLOY_DIR="${AGENT_ONLY_DEPLOY_DIR:-test/network-flow-e2e/deploy/e2e/agent-only}"
DEMO_NAMESPACES="${DEMO_NAMESPACES:-insights shop payments analytics}"
INSIGHTS_GRPC_ADDR="${INSIGHTS_GRPC_ADDR:-}"
INSIGHTS_GRPC_PORT="${INSIGHTS_GRPC_PORT:-4318}"
INSIGHTS_HOST="${INSIGHTS_HOST:-}"
ORGANIZATION="${ORGANIZATION:-}"
CLUSTER="${CLUSTER:-}"
AUTH_TOKEN="${AUTH_TOKEN:-}"
PF_PID=""
WATCH_INTERVAL="${WATCH_INTERVAL:-3}"

demo_namespace_list() {
  echo "$DEMO_NAMESPACES"
}

for_each_demo_namespace() {
  local ns
  for ns in $DEMO_NAMESPACES; do
    "$@" "$ns"
  done
}

delete_demo_traffic() {
  local ns="$1"
  kubectl -n "$ns" delete deployment demo-traffic --ignore-not-found
  kubectl -n "$ns" delete job demo-traffic --ignore-not-found
}

usage() {
  cat <<EOF
Usage: $(basename "$0") [options] [up|down|load|deploy|deploy-agent-only|traffic|traffic-continuous|verify|watch|stream|agent-only|all]

  up       Create kind cluster (if missing)
  down     Delete kind cluster
  load     Build and load local images into kind nodes
  deploy   Apply e2e manifests
  deploy-agent-only  Apply agent-only overlay (no aggregator; collector blocked)
  traffic  Run demo-traffic job (one-shot)
  traffic-continuous  Run demo-traffic deployment (loops until deleted)
  verify   Check flows API for demo-traffic connect + traffic events
  watch    up + load + deploy + continuous traffic + stream flows until Ctrl+C
  stream   Port-forward and stream flows until Ctrl+C (cluster must already be up)
  agent-only  up + load + deploy-agent-only + continuous traffic (queue backpressure test)
  all      up + load + deploy + continuous traffic + stream flows until Ctrl+C

  WATCH_INTERVAL     Poll interval in seconds for watch mode (default: 3)
  DEMO_NAMESPACES    Space-separated demo namespaces (default: insights shop payments analytics)

Required environment variables (Insights upstream is always enabled for deploy/all/watch):
  AUTH_TOKEN         Insights cluster auth token
  INSIGHTS_GRPC_ADDR Host API gRPC address (e.g. host.docker.internal:4318)
  ORGANIZATION       Insights organization slug
  CLUSTER            Insights cluster name

Optional:
  INSIGHTS_GRPC_PORT Host API gRPC port when addr omits port (default: 4318)
  INSIGHTS_HOST      Hostname/IP reachable from kind pods (auto-detected when unset)

Example (Insights API on host: NETWORK_FLOW_GRPC_ADDR=:4318 go run cmd/api/main.go):
  AUTH_TOKEN=<cluster-token> \
  INSIGHTS_GRPC_ADDR=host.docker.internal:4318 \
  ORGANIZATION=ci-co \
  CLUSTER=k8test \
  ./test/network-flow-e2e/kind-e2e.sh all
EOF
}

require_insights_upstream_env() {
  local missing=()
  [[ -z "$AUTH_TOKEN" ]] && missing+=(AUTH_TOKEN)
  [[ -z "$INSIGHTS_GRPC_ADDR" ]] && missing+=(INSIGHTS_GRPC_ADDR)
  [[ -z "$ORGANIZATION" ]] && missing+=(ORGANIZATION)
  [[ -z "$CLUSTER" ]] && missing+=(CLUSTER)
  if ((${#missing[@]} > 0)); then
    echo "required environment variable(s) not set: ${missing[*]}" >&2
    exit 1
  fi
}

normalize_insights_grpc_addr() {
  local addr="${1:-}"
  addr="${addr#http://}"
  addr="${addr#https://}"
  echo "$addr"
}

prepare_insights_upstream_config() {
  local normalized host_part port_part host
  normalized="$(normalize_insights_grpc_addr "$INSIGHTS_GRPC_ADDR")"

  if [[ -n "$normalized" && "$normalized" == *:* ]]; then
    host_part="${normalized%:*}"
    port_part="${normalized##*:}"
    if [[ "$port_part" =~ ^[0-9]+$ ]]; then
      INSIGHTS_GRPC_PORT="$port_part"
    fi
    if [[ -n "$host_part" && "$host_part" != "insights-api" ]]; then
      INSIGHTS_HOST="${INSIGHTS_HOST:-$host_part}"
    fi
  fi

  host="$(resolve_insights_host)"
  # Aggregator runs in kind; Insights API runs on the host.
  INSIGHTS_GRPC_ADDR="${host}:${INSIGHTS_GRPC_PORT}"
}

resolve_insights_host() {
  if [[ -n "$INSIGHTS_HOST" ]]; then
    echo "$INSIGHTS_HOST"
    return
  fi
  if [[ "$(uname -s)" == "Darwin" ]]; then
    echo "host.docker.internal"
    return
  fi
  docker network inspect kind -f '{{(index .IPAM.Config 0).Gateway}}' 2>/dev/null || true
}

apply_insights_upstream() {
  prepare_insights_upstream_config

  local aggregator_patch
  echo "Configuring Insights upstream (host API -> in-cluster aggregator):"
  echo "  INSIGHTS_GRPC_ADDR=${INSIGHTS_GRPC_ADDR}"
  echo "  ORGANIZATION=${ORGANIZATION}"
  echo "  CLUSTER=${CLUSTER}"

  kubectl -n insights create secret generic network-flow-insights-upstream \
    --from-literal=auth-token="$AUTH_TOKEN" \
    --dry-run=client -o yaml | kubectl apply -f -

  kubectl -n insights patch deployment network-flow-aggregator --type=strategic --patch "$(cat <<EOF
spec:
  template:
    spec:
      containers:
        - name: network-flow-aggregator
          env:
            - name: INSIGHTS_GRPC_ADDR
              value: "${INSIGHTS_GRPC_ADDR}"
            - name: ORGANIZATION
              value: "${ORGANIZATION}"
            - name: CLUSTER
              value: "${CLUSTER}"
            - name: AUTH_TOKEN
              valueFrom:
                secretKeyRef:
                  name: network-flow-insights-upstream
                  key: auth-token
EOF
)"

  kubectl -n insights rollout restart deployment/network-flow-aggregator
  kubectl -n insights rollout status deployment/network-flow-aggregator --timeout=120s
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

ensure_kubectl_context() {
  if ! kind get clusters 2>/dev/null | grep -qx "$KIND_CLUSTER"; then
    echo "kind cluster '$KIND_CLUSTER' does not exist; run '$(basename "$0") up' first" >&2
    exit 1
  fi
  kind export kubeconfig --name "$KIND_CLUSTER"
}

kind_up() {
  if kind get clusters 2>/dev/null | grep -qx "$KIND_CLUSTER"; then
    echo "kind cluster '$KIND_CLUSTER' already exists"
  else
    kind create cluster --name "$KIND_CLUSTER" --config "$KIND_CONFIG"
  fi
  ensure_kubectl_context
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

wait_demo_server() {
  kubectl -n "$1" rollout status deployment/demo-server --timeout=60s
}

wait_demo_traffic_complete() {
  kubectl -n "$1" wait --for=condition=complete job/demo-traffic --timeout=120s
}

wait_demo_traffic_rollout() {
  kubectl -n "$1" rollout status deployment/demo-traffic --timeout=60s
}

apply_demo_namespaces() {
  kubectl apply -f "${DEPLOY_DIR}/demo-namespaces.yaml"
}

apply_demo_workloads() {
  kubectl apply -f "${DEPLOY_DIR}/demo-workloads-multi.yaml"
}

kind_deploy() {
  ensure_kubectl_context
  apply_demo_namespaces
  apply_demo_workloads
  kubectl apply -k "$DEPLOY_DIR"
  apply_insights_upstream
  kubectl -n insights rollout status deployment/network-flow-aggregator --timeout=120s
  kubectl -n insights rollout status daemonset/network-flow --timeout=180s
  for_each_demo_namespace wait_demo_server
  kubectl -n insights wait --for=condition=ready pod -l app.kubernetes.io/name=network-flow-aggregator --timeout=120s
}

kind_deploy_agent_only() {
  ensure_kubectl_context
  kubectl -n insights delete deployment,service network-flow-aggregator --ignore-not-found
  apply_demo_namespaces
  apply_demo_workloads
  kubectl apply -k "$AGENT_ONLY_DEPLOY_DIR"
  kubectl apply -f "${DEPLOY_DIR}/demo-traffic-continuous.yaml"
  kubectl -n insights rollout status daemonset/network-flow --timeout=180s
  for_each_demo_namespace wait_demo_server
  for_each_demo_namespace wait_demo_traffic_rollout
  echo
  echo "Agent-only mode: collector blocked at 127.0.0.1:1, MAX_PENDING_EVENTS=1000"
  echo "  logs: kubectl -n insights logs -l app.kubernetes.io/name=network-flow -f"
  echo "  look for: pending flow events dropped by retention"
}

kind_traffic() {
  ensure_kubectl_context
  for_each_demo_namespace delete_demo_traffic
  apply_demo_workloads
  kubectl apply -k "$DEPLOY_DIR"
  for_each_demo_namespace wait_demo_traffic_complete
  sleep 10
}

kind_traffic_continuous() {
  ensure_kubectl_context
  for_each_demo_namespace delete_demo_traffic
  kubectl apply -f "${DEPLOY_DIR}/demo-traffic-continuous.yaml"
  for_each_demo_namespace wait_demo_traffic_rollout
}

format_flow_event() {
  jq -r '
    .events[]?
    | . as $e
    | ($e.src.namespace + "/" + ($e.srcWorkload.kind // "Pod") + "/" + ($e.srcWorkload.name // $e.src.pod)) as $src
    | (if ($e.dstRef.kind // "") != "" then
         $e.dstRef.namespace + "/" + $e.dstRef.kind + "/" + $e.dstRef.name
       else
         ($e.dst.addr // "?")
       end) as $dst
    | (if $e.eventKind == "FLOW_EVENT_KIND_TRAFFIC" then
         " sent=\($e.bytesSent // 0) rcvd=\($e.bytesReceived // 0)"
       else
         ""
       end) as $bytes
    | "\($e.eventKind) \($src) -> \($dst):\($e.dst.port)\($bytes)"
  '
}

wait_for_port_forward() {
  for _ in $(seq 1 30); do
    if curl -sf http://127.0.0.1:18080/healthz >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "e2e watch: port-forward to aggregator did not become ready" >&2
  return 1
}

int_gt() {
  awk -v a="$1" -v b="$2" 'BEGIN { exit (a + 0 > b + 0) ? 0 : 1 }'
}

count_namespace_demo_events() {
  local ns="$1"
  local event_kind="$2"
  local require_bytes="${3:-0}"
  curl -sf "http://127.0.0.1:18080/api/v1/flows?namespace=${ns}&event_kind=${event_kind}" | jq --arg ns "$ns" --argjson require_bytes "$require_bytes" '
    [.events[]?
      | select(
          .src.namespace == $ns
          and (.srcWorkload.name // "") == "demo-traffic"
          and .dstRef.namespace == $ns
          and (.dstRef.name // "") == "demo-server"
          and (
            $require_bytes == 0
            or ((.bytesSent // 0) > 0 or (.bytesReceived // 0) > 0)
          )
        )
    ] | length
  '
}

verify_dns_external() {
  local dns_responses external_connects
  dns_responses="$(curl -sf 'http://127.0.0.1:18080/api/v1/flows?namespace=insights&event_kind=DNS_RESPONSE' | jq '
    [.events[]?
      | select(
          (.srcWorkload.name // "") == "demo-traffic"
          and (.dns.name // "") == "example.com"
        )
    ] | length
  ')"
  external_connects="$(curl -sf 'http://127.0.0.1:18080/api/v1/flows?namespace=insights&event_kind=CONNECT' | jq '
    [.events[]?
      | select(
          (.srcWorkload.name // "") == "demo-traffic"
          and (.dstRef.kind // "") == "ExternalHostname"
          and (.dstRef.name // "") == "example.com"
        )
    ] | length
  ')"
  if [[ "$dns_responses" -ge 1 && "$external_connects" -ge 1 ]]; then
    echo "e2e verify: insights: DNS_RESPONSE and CONNECT with ExternalHostname example.com"
    return 0
  fi
  echo "e2e verify: insights: missing DNS external hostname (dns=$dns_responses connect=$external_connects)" >&2
  return 1
}

verify_demo_namespaces() {
  local ns failed=0
  for ns in $DEMO_NAMESPACES; do
    local has_connect has_traffic
    has_connect="$(count_namespace_demo_events "$ns" CONNECT)"
    has_traffic="$(count_namespace_demo_events "$ns" TRAFFIC 1)"
    if [[ "$has_connect" -ge 1 && "$has_traffic" -ge 1 ]]; then
      echo "e2e verify: ${ns}: CONNECT and TRAFFIC for demo-traffic -> demo-server"
    else
      echo "e2e verify: ${ns}: missing CONNECT ($has_connect) or TRAFFIC with bytes ($has_traffic)" >&2
      failed=1
    fi
  done
  if ! verify_dns_external; then
    failed=1
  fi
  return "$failed"
}

kind_watch() {
  ensure_kubectl_context
  local interval="$WATCH_INTERVAL"
  local since=0
  local verified_namespaces=""

  echo "Watching aggregator flows (Ctrl+C to stop)..."
  echo "  flows:  http://127.0.0.1:18080/api/v1/flows"
  echo "  health: http://127.0.0.1:18080/healthz"
  echo "  namespaces: $DEMO_NAMESPACES"
  echo "  poll interval: ${interval}s"
  echo

  kubectl -n insights port-forward svc/network-flow-aggregator 18080:8080 >/dev/null 2>&1 &
  PF_PID=$!
  trap '[[ -n "${PF_PID:-}" ]] && kill "${PF_PID}" 2>/dev/null || true; exit 0' INT TERM EXIT

  wait_for_port_forward

  while true; do
    local resp count new_since ns
    resp="$(curl -sf "http://127.0.0.1:18080/api/v1/flows?since=${since}" || echo '{"events":[],"count":0}')"
    count="$(echo "$resp" | jq '.count // 0')"

    if [[ "$count" -gt 0 ]]; then
      echo "--- $(date -u +%H:%M:%S) +${count} event(s) ---"
      echo "$resp" | format_flow_event
      new_since="$(echo "$resp" | jq -r '[.events[]?.timestampUnixNano // 0] | if length > 0 then max else 0 end')"
      if int_gt "$new_since" "$since"; then
        since="$new_since"
      fi
    fi

    for ns in $DEMO_NAMESPACES; do
      if [[ " ${verified_namespaces} " == *" ${ns} "* ]]; then
        continue
      fi
      local has_connect has_traffic
      has_connect="$(count_namespace_demo_events "$ns" CONNECT)"
      has_traffic="$(count_namespace_demo_events "$ns" TRAFFIC 1)"
      if [[ "$has_connect" -ge 1 && "$has_traffic" -ge 1 ]]; then
        echo "e2e verify: ${ns}: CONNECT and TRAFFIC for demo-traffic -> demo-server"
        verified_namespaces="${verified_namespaces} ${ns}"
      fi
    done

    sleep "$interval"
  done
}

kind_verify() {
  ensure_kubectl_context
  local flows demo_flows
  kubectl -n insights port-forward svc/network-flow-aggregator 18080:8080 >/dev/null 2>&1 &
  PF_PID=$!
  trap '[[ -n "${PF_PID:-}" ]] && kill "${PF_PID}" 2>/dev/null || true' EXIT

  for _ in $(seq 1 30); do
    if curl -sf http://127.0.0.1:18080/healthz >/dev/null 2>&1; then
      break
    fi
    sleep 1
  done

  flows="$(curl -sf http://127.0.0.1:18080/api/v1/flows | format_flow_event || true)"

  demo_flows="$(curl -sf 'http://127.0.0.1:18080/api/v1/flows' | jq -r --arg nslist "$DEMO_NAMESPACES" '
    ($nslist | split(" ")) as $namespaces
    | [.events[]?
        | select(
            ((.srcWorkload.name // "") == "demo-traffic" or (.dstRef.name // "") == "demo-server")
            and (
              (.src.namespace // "") as $src_ns
              | (.dstRef.namespace // "") as $dst_ns
              | ($namespaces | index($src_ns)) or ($namespaces | index($dst_ns))
            )
          )
        | . as $e
        | ($e.src.namespace + "/" + ($e.srcWorkload.kind // "Pod") + "/" + ($e.srcWorkload.name // $e.src.pod)) as $src
        | (if ($e.dstRef.kind // "") != "" then
             $e.dstRef.namespace + "/" + $e.dstRef.kind + "/" + $e.dstRef.name
           else
             ($e.dst.addr // "?")
           end) as $dst
        | {
            line: (
              (if $e.eventKind == "FLOW_EVENT_KIND_TRAFFIC" then
                 " sent=\($e.bytesSent // 0) rcvd=\($e.bytesReceived // 0)"
               else
                 ""
               end) as $bytes
              | "\($e.eventKind) \($src) -> \($dst):\($e.dst.port)\($bytes)"
            ),
            kind: ($e.eventKind // ""),
            has_bytes: (
              $e.eventKind == "FLOW_EVENT_KIND_TRAFFIC"
              and (($e.bytesSent // 0) > 0 or ($e.bytesReceived // 0) > 0)
            )
          }
      ]
      | sort_by(if .has_bytes then 0 elif .kind == "FLOW_EVENT_KIND_TRAFFIC" then 1 else 2 end)
      | .[].line
  ' || true)"

  if [[ -n "$demo_flows" ]]; then
    echo "demo workload events:"
    echo "$demo_flows"
  else
    echo "flow events (all):"
    echo "$flows"
  fi

  verify_demo_namespaces
  local verify_status=$?
  if [[ "$verify_status" -ne 0 && -z "$demo_flows" ]]; then
    echo "e2e verify: no demo events yet; sample raw events:"
    curl -sf http://127.0.0.1:18080/api/v1/flows | jq '[.events[]? | select(.srcWorkload.name == "demo-traffic" or .dstRef.name == "demo-server")][0:5] // .events[0:5]'
  fi
  return "$verify_status"
}

cmd="${1:-all}"
shift || true
while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help|help) usage; exit 0 ;;
    *) echo "unknown option: $1" >&2; usage; exit 1 ;;
  esac
  shift
done

case "$cmd" in
  deploy|all|watch) require_insights_upstream_env ;;
esac

case "$cmd" in
  up) kind_up ;;
  down) kind_down ;;
  load) kind_load ;;
  deploy) kind_deploy ;;
  deploy-agent-only) kind_deploy_agent_only ;;
  traffic) kind_traffic ;;
  traffic-continuous) kind_traffic_continuous ;;
  verify) kind_verify ;;
  stream) kind_watch ;;
  watch)
    kind_up
    kind_load
    kind_deploy
    kind_traffic_continuous
    kind_watch
    ;;
  agent-only)
    kind_up
    kind_load
    kind_deploy_agent_only
    ;;
  all)
    kind_up
    kind_load
    kind_deploy
    kind_traffic_continuous
    kind_watch
    ;;
  -h|--help|help) usage ;;
  *) echo "unknown command: $cmd" >&2; usage; exit 1 ;;
esac
