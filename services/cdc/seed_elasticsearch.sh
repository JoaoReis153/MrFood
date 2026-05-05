#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ELASTIC_URL="http://localhost:${CDC_ELASTIC_PORT:-9200}"
DOCKER_COMPOSE_PROJECT="${DOCKER_COMPOSE_PROJECT:-mrfood}"
DOCKER_COMPOSE_FILE="${DOCKER_COMPOSE_FILE:-$SCRIPT_DIR/../docker-compose.yml}"
MAPPING_FILE="${MAPPING_FILE:-$SCRIPT_DIR/mappings/restaurants.json}"
INDEX="restaurants"
DB_USER="${DB_USER:-restaurant}"
DB_NAME="${DB_NAME:-mrfood_restaurant}"

log()  { echo "[seed-es] $*"; }
fail() { echo "[seed-es][error] $*" >&2; exit 1; }

# ── 1. Ingest pipeline ────────────────────────────────────────────────────────

log "Ensuring ingest pipeline exists..."
curl -fsS -X PUT "$ELASTIC_URL/_ingest/pipeline/restaurants_location_pipeline" \
  -H "Content-Type: application/json" \
  -d '{
    "description": "Combine latitude/longitude into geo_point",
    "processors": [{
      "script": {
        "source": "if (ctx.latitude != null && ctx.longitude != null) { ctx.location = [\"lat\": ctx.latitude, \"lon\": ctx.longitude]; }"
      }
    }]
  }' > /dev/null
log "✔ Pipeline ready"

# ── 2. Index creation (idempotent) ────────────────────────────────────────────

[[ -f "$MAPPING_FILE" ]] || fail "Mapping file not found: $MAPPING_FILE"

HTTP_CODE=$(curl -sS -o /dev/null -w "%{http_code}" "$ELASTIC_URL/$INDEX")
if [[ "$HTTP_CODE" == "200" ]]; then
  log "✔ Index '$INDEX' already exists, skipping creation"
else
  log "Creating index '$INDEX'..."
  RESPONSE=$(curl -fsS -w "\n%{http_code}" -X PUT "$ELASTIC_URL/$INDEX" \
    -H "Content-Type: application/json" \
    --data @"$MAPPING_FILE")
  CODE=$(tail -1 <<< "$RESPONSE")
  [[ "$CODE" == "200" ]] || fail "Index creation failed (HTTP $CODE): $(head -1 <<< "$RESPONSE")"
  log "✔ Index created"
fi

# ── 3. Fetch from Postgres and bulk index ─────────────────────────────────────

QUERY="SELECT row_to_json(r) FROM (
  SELECT
    r.id, r.name, r.latitude, r.longitude, r.address,
    r.media_url, r.max_slots, r.owner_id, r.owner_name, r.sponsor_tier,
    COALESCE(
      ARRAY_AGG(rc.category ORDER BY rc.id) FILTER (WHERE rc.category IS NOT NULL),
      ARRAY[]::text[]
    ) AS categories
  FROM restaurants r
  LEFT JOIN restaurant_categories rc ON rc.restaurant_id = r.id
  GROUP BY r.id, r.name, r.latitude, r.longitude, r.address,
           r.media_url, r.max_slots, r.owner_id, r.owner_name, r.sponsor_tier
) r;"

log "Fetching restaurants from PostgreSQL..."
ROWS=$(docker compose -p "$DOCKER_COMPOSE_PROJECT" -f "$DOCKER_COMPOSE_FILE" \
  exec -T restaurant_db \
  psql -U "$DB_USER" -d "$DB_NAME" -t -A -c "$QUERY")

ROW_COUNT=$(grep -c . <<< "$ROWS" || true)
log "Fetched $ROW_COUNT rows"

if [[ "$ROW_COUNT" -eq 0 ]]; then
  log "No rows to index, skipping bulk"
  exit 0
fi

# Build NDJSON with explicit _id so re-runs are idempotent
NDJSON=$(while IFS= read -r row; do
  [[ -z "$row" ]] && continue
  id=$(python3 -c "import json,sys; print(json.loads(sys.argv[1])['id'])" "$row")
  printf '{"index":{"_index":"%s","_id":"%s"}}\n%s\n' "$INDEX" "$id" "$row"
done <<< "$ROWS")

log "Bulk indexing $ROW_COUNT documents..."
BULK_RESPONSE=$(curl -sS -X POST "$ELASTIC_URL/$INDEX/_bulk?refresh=wait_for" \
  -H "Content-Type: application/x-ndjson" \
  --data-binary "$NDJSON")

# Check for per-document errors in the bulk response
ERRORS=$(python3 - <<'PY' "$BULK_RESPONSE"
import json, sys
resp = json.loads(sys.argv[1])
if not resp.get("errors"):
    print("0")
    sys.exit(0)
failed = [
    item.get("index", {})
    for item in resp.get("items", [])
    if item.get("index", {}).get("error")
]
print(len(failed))
for f in failed[:5]:  # show first 5 failures
    print(f"  id={f.get('_id')} error={f.get('error')}", file=sys.stderr)
PY
)

if [[ "$ERRORS" != "0" ]]; then
  fail "$ERRORS document(s) failed to index (see above)"
fi

# ── 4. Verify ─────────────────────────────────────────────────────────────────

COUNT=$(curl -fsS "$ELASTIC_URL/$INDEX/_count" | python3 -c "import json,sys; print(json.load(sys.stdin)['count'])")
log "✔ Done — $INDEX now has $COUNT documents"