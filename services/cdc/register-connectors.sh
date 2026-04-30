#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONNECT_URL="http://localhost:${CDC_CONNECT_PORT:-8083}"
CONNECT_TIMEOUT_SECONDS="${CONNECT_TIMEOUT_SECONDS:-120}"

log()  { echo "[register] $*"; }
fail() { echo "[register][error] $*" >&2; exit 1; }

wait_for_connect() {
  local deadline=$((SECONDS + CONNECT_TIMEOUT_SECONDS))
  local elapsed=0
  log "Waiting for Kafka Connect at $CONNECT_URL (this takes ~60s on first run while plugins install)..."
  while (( SECONDS < deadline )); do
    if curl -fsS "$CONNECT_URL/connector-plugins" >/dev/null 2>&1; then
      log "✔ Kafka Connect ready (${elapsed}s)"
      return 0
    fi
    printf "  [%ds] still waiting...\r" "$elapsed"
    sleep 5
    elapsed=$((elapsed + 5))
  done
  fail "Kafka Connect did not become ready within ${CONNECT_TIMEOUT_SECONDS}s"
}

parse_name() {
  python3 -c "import json,sys; print(json.load(open(sys.argv[1]))['name'])" "$1"
}

upsert_connector() {
  local file="$1"
  local name
  name="$(parse_name "$file")"
  [[ -n "$name" ]] || fail "Could not parse connector name from: $file"

  local http_code
  http_code="$(curl -s -o /dev/null -w '%{http_code}' "$CONNECT_URL/connectors/$name")"

  local response
  response="$(mktemp)"
  trap "rm -f $response" RETURN

  if [[ "$http_code" == "200" ]]; then
    log "Updating connector: $name"
    local config
    config="$(python3 -c "import json,sys; print(json.dumps(json.load(open(sys.argv[1]))['config']))" "$file")"
    local code
    code="$(curl -sS -o "$response" -w '%{http_code}' \
      -X PUT "$CONNECT_URL/connectors/$name/config" \
      -H 'Content-Type: application/json' \
      --data "$config")"
    [[ "$code" == "200" ]] || { cat "$response" >&2; fail "Update failed for '$name' (HTTP $code)"; }
  else
    log "Creating connector: $name"
    local code
    code="$(curl -sS -o "$response" -w '%{http_code}' \
      -X POST "$CONNECT_URL/connectors" \
      -H 'Content-Type: application/json' \
      --data @"$file")"
    [[ "$code" == "201" ]] || { cat "$response" >&2; fail "Create failed for '$name' (HTTP $code)"; }
  fi

  log "✔ Connector '$name' registered"
}

wait_for_connect
upsert_connector "$SCRIPT_DIR/connectors/restaurant-source.json"
upsert_connector "$SCRIPT_DIR/connectors/restaurants-sink.json"
log "All connectors registered"