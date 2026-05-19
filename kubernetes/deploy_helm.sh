#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
NAMESPACE="mrfood"
TIMEOUT="5m"
SKIP_CONNECTORS=0
SKIP_RESTART=0

usage() {
  cat <<EOF
Usage: $(basename "$0") [--skip-connectors] [--skip-restart] [--timeout <dur>]

Runs Helm/Kubernetes commands from DEPLOYMENT.md ("Kubernetes — Helm" section).

Options:
  --skip-connectors   Skip registering CDC connectors
  --skip-restart      Skip running kubernetes/restart.sh (application services)
  --timeout <dur>     Rollout timeout for kubectl (default: ${TIMEOUT})
  -h, --help          Show this help
EOF
}

while [[ ${#} -gt 0 ]]; do
  case "$1" in
    --skip-connectors) SKIP_CONNECTORS=1; shift ;;
    --skip-restart) SKIP_RESTART=1; shift ;;
    --timeout) TIMEOUT="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown arg: $1"; usage; exit 2 ;;
  esac
done

check_cmds() {
  for c in helm kubectl bash; do
    if ! command -v "$c" >/dev/null 2>&1; then
      echo "Error: required command '$c' not found in PATH" >&2
      exit 3
    fi
  done
}

run() {
  echo "+ $*"
  "$@"
}

run_allow_err() {
  echo "+ $*"
  if ! "$@"; then
    echo "Warning: command failed: $*" >&2
    return 1
  fi
}

trap 'echo "ERROR: script failed at line $LINENO" >&2' ERR

check_cmds

echo "==> Applying namespace manifest"
run kubectl apply -f "$ROOT_DIR/kubernetes/namespace.yaml"

echo "==> Deploying observability stack (otel-collector, prometheus, loki, tempo, grafana)"
run helm upgrade --install otel-collector "$ROOT_DIR/kubernetes/helm/otel-collector" --namespace "$NAMESPACE"
run helm upgrade --install observability "$ROOT_DIR/kubernetes/helm/observability" --namespace "$NAMESPACE"

echo "==> Deploying Keycloak (required before auth)"
run helm upgrade --install keycloak "$ROOT_DIR/kubernetes/helm/keycloak" --namespace "$NAMESPACE"
echo "Waiting for Keycloak to be ready (deployment/keycloak)"
run kubectl rollout status deployment/keycloak -n "$NAMESPACE" --timeout "$TIMEOUT"

echo "==> Deploying search stack (Elasticsearch, Kafka)"
run helm upgrade --install elasticsearch "$ROOT_DIR/kubernetes/helm/elasticsearch" --namespace "$NAMESPACE"
run helm upgrade --install kafka "$ROOT_DIR/kubernetes/helm/kafka" --namespace "$NAMESPACE"

echo "Waiting for Zookeeper, Kafka and Elasticsearch to become ready"
run kubectl rollout status deployment/zookeeper -n "$NAMESPACE" --timeout "$TIMEOUT"
run kubectl rollout status deployment/kafka -n "$NAMESPACE" --timeout "$TIMEOUT"
run kubectl rollout status deployment/elasticsearch -n "$NAMESPACE" --timeout "$TIMEOUT"

echo "==> Deploying CDC (Kafka Connect)"
run helm upgrade --install cdc "$ROOT_DIR/kubernetes/helm/kafka-connect" -f "$ROOT_DIR/kubernetes/values/cdc.yaml" --namespace "$NAMESPACE"

if [[ $SKIP_CONNECTORS -eq 0 ]]; then
  echo "==> Registering CDC connectors (if not already present)"
  # Connector 1: restaurant-postgres-source
  run_allow_err kubectl exec -n "$NAMESPACE" deployment/cdc -- bash -c \
    "curl -sf http://localhost:8083/connectors | grep -q restaurant-postgres-source || \
      curl -sf -X POST http://localhost:8083/connectors -H 'Content-Type: application/json' -d @/connectors/restaurant-source.json"

  # Connector 2: restaurants-elasticsearch-sink
  run_allow_err kubectl exec -n "$NAMESPACE" deployment/cdc -- bash -c \
    "curl -sf http://localhost:8083/connectors | grep -q restaurants-elasticsearch-sink || \
      curl -sf -X POST http://localhost:8083/connectors -H 'Content-Type: application/json' -d @/connectors/restaurants-sink.json"
else
  echo "Skipping connector registration (--skip-connectors set)"
fi

if [[ $SKIP_RESTART -eq 0 ]]; then
  echo "==> Deploying application services via kubernetes/restart.sh"
  if [[ ! -x "$ROOT_DIR/kubernetes/restart.sh" ]]; then
    echo "Making restart.sh executable"
    run chmod +x "$ROOT_DIR/kubernetes/restart.sh"
  fi
  run bash "$ROOT_DIR/kubernetes/restart.sh"
else
  echo "Skipping application restart/install (--skip-restart set)"
fi

echo "==> Patching Kong configmap and restarting gateway"
run kubectl create configmap kong-config -n "$NAMESPACE" --from-file=kong.yml="$ROOT_DIR/services/gateway/kong/kong.yml" --dry-run=client -o yaml | kubectl apply -f -
run kubectl rollout restart deployment/gateway -n "$NAMESPACE"

echo "All Helm/Kubernetes tasks from DEPLOYMENT.md completed."

