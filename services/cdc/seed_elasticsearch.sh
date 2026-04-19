#!/usr/bin/env bash
# Seed Elasticsearch with restaurant data from the PostgreSQL database
# This is a direct bulk-load approach while CDC/Kafka pipeline is being debugged

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ELASTIC_URL="${ELASTIC_URL:-http://localhost:9200}"
DOCKER_COMPOSE_PROJECT="${DOCKER_COMPOSE_PROJECT:-mrfood}"
DOCKER_COMPOSE_FILE="${DOCKER_COMPOSE_FILE:-../docker-compose.yml}"

echo "Creating restaurants index with proper mapping..."

echo "Ensuring ingest pipeline restaurants_location_pipeline exists..."
curl -s -X PUT "$ELASTIC_URL/_ingest/pipeline/restaurants_location_pipeline" \
  -H "Content-Type: application/json" -d '{
  "description": "Populate geo_point location from latitude/longitude",
  "processors": [
    {
      "script": {
        "source": "if (ctx.latitude != null && ctx.longitude != null) { ctx.location = ['\''lat'\'': ctx.latitude, '\''lon'\'': ctx.longitude]; }"
      }
    }
  ]
}' > /dev/null

# Create/recreate the index with correct field mappings (id as long, not int)
curl -s -X DELETE "$ELASTIC_URL/restaurants" > /dev/null 2>&1 || true
curl -s -X PUT "$ELASTIC_URL/restaurants" -H "Content-Type: application/json" -d '{
  "settings": {
    "index.default_pipeline": "restaurants_location_pipeline"
  },
  "mappings": {
    "properties": {
      "id": {"type": "long"},
      "name": {"type": "text"},
      "latitude": {"type": "double"},
      "longitude": {"type": "double"},
      "location": {"type": "geo_point"},
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

echo "Backfilling missing location fields..."
curl -s -X POST "$ELASTIC_URL/restaurants/_update_by_query?conflicts=proceed&refresh=true" \
  -H "Content-Type: application/json" -d '{
  "script": {
    "lang": "painless",
    "source": "if (ctx._source.latitude != null && ctx._source.longitude != null) { ctx._source.location = ['\''lat'\'': ctx._source.latitude, '\''lon'\'': ctx._source.longitude]; }"
  },
  "query": {
    "bool": {
      "must": [
        {"exists": {"field": "latitude"}},
        {"exists": {"field": "longitude"}}
      ],
      "must_not": [
        {"exists": {"field": "location"}}
      ]
    }
  }
}' > /dev/null

COUNT=$(curl -s "$ELASTIC_URL/restaurants/_count" | jq '.count')
echo "✓ Elasticsearch restaurants index now has $COUNT documents"

