#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONNECT_URL="${CONNECT_URL:-http://localhost:8083}"
CONNECT_TIMEOUT_SECONDS="${CONNECT_TIMEOUT_SECONDS:-120}"

# Load config.env from project root (adjust path if needed)
CONFIG_FILE="${SCRIPT_DIR}/../config.env"

if [[ -f "$CONFIG_FILE" ]]; then
  set -a  
  source "$CONFIG_FILE"
  set +a
else
  echo "config.env not found at $CONFIG_FILE" >&2
  exit 1
fi

wait_for_connect() {
  local deadline=$((SECONDS + CONNECT_TIMEOUT_SECONDS))
  echo "Waiting for Kafka Connect at $CONNECT_URL ..."
  while (( SECONDS < deadline )); do
    if curl -fsS "$CONNECT_URL/connector-plugins" >/dev/null 2>&1; then
      echo "Kafka Connect is ready."
      return 0
    fi
    sleep 2
  done

  echo "Kafka Connect did not become ready within ${CONNECT_TIMEOUT_SECONDS}s" >&2
  return 1
}

parse_name() {
  local connector_file="$1"
  python3 - <<'PY' "$connector_file"
import json, sys
with open(sys.argv[1], 'r', encoding='utf-8') as f:
    obj = json.load(f)
print(obj.get('name', ''))
PY
}

upsert_connector() {
  local connector_file="$1"
  local name
  name="$(parse_name "$connector_file")"

  if [[ -z "$name" ]]; then
    echo "Could not parse connector name from: $connector_file" >&2
    exit 1
  fi

  local status
  status="$(curl -s -o /dev/null -w '%{http_code}' "$CONNECT_URL/connectors/$name")"

  if [[ "$status" == "200" ]]; then
    echo "Updating connector: $name"
    local update_body
    update_body="$(python3 - <<'PY' "$connector_file"
import json, sys
p = sys.argv[1]
with open(p, 'r', encoding='utf-8') as f:
    obj = json.load(f)
print(json.dumps(obj['config']))
PY
)"

    local update_response
    update_response="$(mktemp)"
    local update_code
    update_code="$(curl -sS -o "$update_response" -w '%{http_code}' -X PUT "$CONNECT_URL/connectors/$name/config" \
      -H 'Content-Type: application/json' \
      --data "$update_body")"

    if [[ "$update_code" != "200" ]]; then
      echo "Failed updating connector '$name' (HTTP $update_code):" >&2
      cat "$update_response" >&2
      rm -f "$update_response"
      exit 1
    fi
    rm -f "$update_response"
  else
    echo "Creating connector: $name"
    local create_response
    create_response="$(mktemp)"
    local create_code
    create_code="$(curl -sS -o "$create_response" -w '%{http_code}' -X POST "$CONNECT_URL/connectors" \
      -H 'Content-Type: application/json' \
      --data @"$connector_file")"

    if [[ "$create_code" != "201" ]]; then
      echo "Failed creating connector '$name' (HTTP $create_code):" >&2
      cat "$create_response" >&2
      rm -f "$create_response"
      exit 1
    fi
    rm -f "$create_response"
  fi
}

wait_for_connect
upsert_connector "$SCRIPT_DIR/connectors/restaurant-source.json"
upsert_connector "$SCRIPT_DIR/connectors/restaurants-sink.json"

echo "Connectors are registered."
