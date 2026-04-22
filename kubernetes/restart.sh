#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
NAMESPACE="mrfood"
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

for values_file in "${value_files[@]}"; do
  service="$(basename "$values_file" .yaml)"

  echo "[${service}] Uninstalling release from namespace ${NAMESPACE}..."
  helm uninstall "$service" -n "$NAMESPACE" >/dev/null 2>&1 || true

  if [[ "$service" == "gateway" ]]; then
    echo "[${service}] Installing release with helm/kong and values/$(basename "$values_file")..."
    helm install "$service" "$GATEWAY_CHART_DIR" -f "$values_file" -n "$NAMESPACE"
  else
    echo "[${service}] Installing release with values/$(basename "$values_file")..."
    helm install "$service" "$CHART_DIR" -f "$values_file" -n "$NAMESPACE"
  fi

done

echo "All services processed successfully."
