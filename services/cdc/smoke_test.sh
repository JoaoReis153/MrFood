#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICES_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
COMPOSE_FILE="${COMPOSE_FILE:-$SERVICES_DIR/docker-compose.cdc.yml}"
RESTAURANT_COMPOSE_FILE="${RESTAURANT_COMPOSE_FILE:-$SERVICES_DIR/docker-compose.restaurant.yml}"
BASE_COMPOSE_FILE="${BASE_COMPOSE_FILE:-$SERVICES_DIR/docker-compose.yml}"
DOCKER_COMPOSE_PROJECT="${DOCKER_COMPOSE_PROJECT:-mrfood}"

CONNECT_URL="${CONNECT_URL:-http://localhost:8083}"
ELASTIC_URL="${ELASTIC_URL:-http://localhost:9200}"
ELASTIC_INDEX="${ELASTIC_INDEX:-_all}"

DB_USER="${DB_USER:-restaurant}"
DB_NAME="${DB_NAME:-mrfood_restaurant}"

CONNECT_TIMEOUT_SECONDS="${CONNECT_TIMEOUT_SECONDS:-120}"
ELASTIC_TIMEOUT_SECONDS="${ELASTIC_TIMEOUT_SECONDS:-120}"
INDEX_TIMEOUT_SECONDS="${INDEX_TIMEOUT_SECONDS:-60}"

SOURCE_CONNECTOR="${SOURCE_CONNECTOR:-restaurant-postgres-source}"
SINK_CONNECTOR="${SINK_CONNECTOR:-restaurants-elasticsearch-sink}"

log() {
  echo "[smoke] $*"
}

fail() {
  echo "[smoke][error] $*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "Missing required command: $1"
}

wait_http_ok() {
  local url="$1"
  local timeout="$2"
  local label="$3"
  local deadline=$((SECONDS + timeout))

  while (( SECONDS < deadline )); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      log "$label is ready"
      return 0
    fi
    sleep 2
  done

  fail "$label did not become ready within ${timeout}s ($url)"
}

assert_connector_running() {
  local name="$1"
  local tmp_file
  tmp_file="$(mktemp)"

  if ! curl -fsS "$CONNECT_URL/connectors/$name/status" >"$tmp_file"; then
    rm -f "$tmp_file"
    fail "Could not get connector status for '$name'"
  fi

  if ! python3 - <<'PY' "$tmp_file" "$name"
import json
import sys

status_path = sys.argv[1]
connector_name = sys.argv[2]

with open(status_path, 'r', encoding='utf-8') as f:
    data = json.load(f)

connector_state = data.get('connector', {}).get('state')
task_states = [task.get('state') for task in data.get('tasks', [])]

ok = connector_state == 'RUNNING' and task_states and all(state == 'RUNNING' for state in task_states)
if not ok:
    print(f"Connector '{connector_name}' not fully RUNNING.")
    print(json.dumps(data, indent=2))
    sys.exit(1)
PY
  then
    rm -f "$tmp_file"
    fail "Connector '$name' is not healthy"
  fi

  rm -f "$tmp_file"
  log "Connector '$name' is RUNNING"
}

require_cmd docker
require_cmd curl
require_cmd python3

log "Checking CDC stack is up"
if ! docker compose -p "$DOCKER_COMPOSE_PROJECT" -f "$COMPOSE_FILE" ps >/dev/null 2>&1; then
  fail "Could not query docker compose status using $COMPOSE_FILE"
fi

wait_http_ok "$CONNECT_URL/connector-plugins" "$CONNECT_TIMEOUT_SECONDS" "Kafka Connect"
wait_http_ok "$ELASTIC_URL" "$ELASTIC_TIMEOUT_SECONDS" "Elasticsearch"

if [[ "${SKIP_REGISTER_CONNECTORS:-0}" != "1" ]]; then
  log "Registering connectors"
  bash "$SCRIPT_DIR/register-connectors.sh"
fi

assert_connector_running "$SOURCE_CONNECTOR"
assert_connector_running "$SINK_CONNECTOR"

unique_suffix="$(date +%s)"
restaurant_name="smoke-cdc-${unique_suffix}"
restaurant_id="$((1700000000 + unique_suffix))"

log "Inserting test restaurant into Postgres: $restaurant_name"
docker compose -p "$DOCKER_COMPOSE_PROJECT" -f "$BASE_COMPOSE_FILE" -f "$RESTAURANT_COMPOSE_FILE" up -d restaurant_db >/dev/null
db_ready=0
for _ in $(seq 1 30); do
  if docker compose -p "$DOCKER_COMPOSE_PROJECT" -f "$BASE_COMPOSE_FILE" -f "$RESTAURANT_COMPOSE_FILE" exec -T restaurant_db pg_isready -U "$DB_USER" -d "$DB_NAME" >/dev/null 2>&1; then
    db_ready=1
    break
  fi
  sleep 2
done
[[ "$db_ready" -eq 1 ]] || fail "restaurant_db did not become ready in time"
insert_output="$({
  docker compose -p "$DOCKER_COMPOSE_PROJECT" -f "$BASE_COMPOSE_FILE" -f "$RESTAURANT_COMPOSE_FILE" exec -T restaurant_db psql -U "$DB_USER" -d "$DB_NAME" -t -A -c "INSERT INTO restaurants (id, name, latitude, longitude, address, opening_time, closing_time, max_slots, owner_id, owner_name, sponsor_tier) VALUES ($restaurant_id, '$restaurant_name', 41.1579, -8.6291, 'Porto', '09:00:00', '22:00:00', 20, 9999, 'smoke-test', 0) RETURNING id;"
} | cat)"

restaurant_id="$(printf '%s\n' "$insert_output" | sed -n '1p' | tr -dc '0-9')"

[[ -n "$restaurant_id" ]] || fail "Insert succeeded but no restaurant id was returned"
log "Inserted restaurant id: $restaurant_id"

log "Waiting for Elasticsearch to index restaurant id $restaurant_id"
found=0
deadline=$((SECONDS + INDEX_TIMEOUT_SECONDS))
while (( SECONDS < deadline )); do
  search_result="$(curl -fsS -X POST "$ELASTIC_URL/$ELASTIC_INDEX/_search" -H 'Content-Type: application/json' -d "{\"query\":{\"term\":{\"id\":$restaurant_id}}}")"

  if python3 - <<'PY' "$search_result"
import json
import sys

data = json.loads(sys.argv[1])
value = data.get('hits', {}).get('total', {}).get('value', 0)
if value > 0:
    sys.exit(0)
sys.exit(1)
PY
  then
    found=1
    break
  fi

  sleep 2
done

if [[ "$found" -ne 1 ]]; then
  fail "Restaurant id $restaurant_id was not indexed into Elasticsearch within ${INDEX_TIMEOUT_SECONDS}s"
fi

log "PASS: CDC pipeline is healthy and indexing changes into Elasticsearch"

