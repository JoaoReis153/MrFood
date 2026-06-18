#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../gcp.env"
source "${SCRIPT_DIR}/../secrets.env"
NAMESPACE="${K8S_NAMESPACE}"
CHART_DIR="$SCRIPT_DIR/helm/mrfood-service"
GATEWAY_CHART_DIR="$SCRIPT_DIR/helm/kong"
VALUES_DIR="$SCRIPT_DIR/values"

if ! command -v helm >/dev/null 2>&1; then
  echo "Error: helm is not installed or not in PATH."
  exit 1
fi

if [[ ! -d "$CHART_DIR" ]]; then
  echo "Error: chart directory not found: $CHART_DIR"
  exit 1
fi

if [[ ! -d "$VALUES_DIR" ]]; then
  echo "Error: values directory not found: $VALUES_DIR"
  exit 1
fi

if [[ ! -d "$GATEWAY_CHART_DIR" ]]; then
  echo "Error: gateway chart directory not found: $GATEWAY_CHART_DIR"
  exit 1
fi

shopt -s nullglob
value_files=("$VALUES_DIR"/*.yaml)
shopt -u nullglob

if [[ ${#value_files[@]} -eq 0 ]]; then
  echo "Error: no values files found in $VALUES_DIR"
  exit 1
fi

REDIS_HOST_NOTIFICATION=""
if command -v terraform >/dev/null 2>&1 && [[ -d "$SCRIPT_DIR/../terraform" ]]; then
  REDIS_HOST_NOTIFICATION=$(terraform -chdir="$SCRIPT_DIR/../terraform" output -json service_redis_hosts 2>/dev/null | python3 -c "import json,sys; print(json.load(sys.stdin).get('notification',''))" 2>/dev/null || true)
fi

for values_file in "${value_files[@]}"; do
  service="$(basename "$values_file" .yaml)"

  if [[ "$service" == "cdc" ]]; then
    echo "[${service}] Skipping — deployed via kafka-connect chart."
    continue
  fi

  echo "[${service}] Uninstalling release from namespace ${NAMESPACE}..."
  helm uninstall "$service" -n "$NAMESPACE" >/dev/null 2>&1 || true

  extra_args=()
  if [[ "$service" == "notification" && -n "${REDIS_HOST_NOTIFICATION:-}" ]]; then
    extra_args+=(--set "env.config.NOTIFICATION_REDIS_HOST=${REDIS_HOST_NOTIFICATION}")
  fi
  if [[ "$service" == "payment" ]]; then
    if [[ -z "${STRIPE_SECRET_KEY:-}" ]]; then
      echo "Error: STRIPE_SECRET_KEY is not set. Add it to gcp.env before deploying."
      exit 1
    fi
    extra_args+=(--set "env.secrets.STRIPE_SECRET_KEY=${STRIPE_SECRET_KEY}")
  fi

  if [[ "$service" == "gateway" ]]; then
    echo "[${service}] Installing release with helm/kong and values/$(basename "$values_file")..."
    helm install "$service" "$GATEWAY_CHART_DIR" -f "$values_file" -n "$NAMESPACE" --create-namespace "${extra_args[@]+"${extra_args[@]}"}"
  else
    echo "[${service}] Installing release with values/$(basename "$values_file")..."
    helm install "$service" "$CHART_DIR" -f "$values_file" -n "$NAMESPACE" --create-namespace \
      --set "gcpProjectId=${GCP_PROJECT_ID}" \
      "${extra_args[@]+"${extra_args[@]}"}"
  fi

done

echo "All services processed successfully."
