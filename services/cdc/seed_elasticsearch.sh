#!/usr/bin/env bash
# Seed Elasticsearch with restaurant data from the PostgreSQL database
# This is a direct bulk-load approach while CDC/Kafka pipeline is being debugged

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ELASTIC_URL="${ELASTIC_URL:-http://localhost:9200}"
DOCKER_COMPOSE_PROJECT="${DOCKER_COMPOSE_PROJECT:-mrfood}"
DOCKER_COMPOSE_FILE="${DOCKER_COMPOSE_FILE:-../docker-compose.yml}"

echo "Creating restaurants index with proper mapping..."
# Create/recreate the index with correct field mappings (id as long, not int)
curl -s -X DELETE "$ELASTIC_URL/restaurants" > /dev/null 2>&1 || true
curl -s -X PUT "$ELASTIC_URL/restaurants" -H "Content-Type: application/json" -d '{
  "mappings": {
    "properties": {
      "id": {"type": "long"},
      "name": {"type": "text"},
      "latitude": {"type": "double"},
      "longitude": {"type": "double"},
      "address": {"type": "text"},
      "media_url": {"type": "keyword"},
      "max_slots": {"type": "integer"},
      "owner_id": {"type": "long"},
      "owner_name": {"type": "text"},
      "sponsor_tier": {"type": "integer"}
    }
  }
}' > /dev/null

echo "Fetching restaurants from PostgreSQL..."
# Fetch restaurants with simple row_to_json conversion
QUERY="
SELECT row_to_json(r) FROM (
  SELECT restaurants.id, restaurants.name, restaurants.latitude, restaurants.longitude,
         restaurants.address, restaurants.media_url, restaurants.max_slots,
         restaurants.owner_id, restaurants.owner_name, restaurants.sponsor_tier
  FROM restaurants
) r;
"

echo "Bulk indexing into Elasticsearch..."
# Build ndjson and post to Elasticsearch
docker compose -p "$DOCKER_COMPOSE_PROJECT" -f "$DOCKER_COMPOSE_FILE" exec -T restaurant_db \
  psql -U restaurant -d mrfood_restaurant -t -c "$QUERY" | \
  while IFS= read -r line; do
    [ -z "$line" ] && continue
    echo "{\"index\":{\"_index\":\"restaurants\"}}"
    echo "$line"
  done | \
  (cat; echo) | \
  curl -s -X POST "$ELASTIC_URL/restaurants/_bulk" -H "Content-Type: application/x-ndjson" -d @- > /dev/null

# Wait a moment for index refresh
sleep 2

COUNT=$(curl -s "$ELASTIC_URL/restaurants/_count" | jq '.count')
echo "✓ Elasticsearch restaurants index now has $COUNT documents"

