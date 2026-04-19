#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_FILE="$SCRIPT_DIR/../docker-compose.cdc.yml"
BASE_COMPOSE_FILE="$SCRIPT_DIR/../docker-compose.yml"
RESTAURANT_COMPOSE_FILE="$SCRIPT_DIR/../docker-compose.restaurant.yml"
DOCKER_COMPOSE_PROJECT="${DOCKER_COMPOSE_PROJECT:-mrfood}"

ELASTIC_URL="${ELASTIC_URL:-http://localhost:${CDC_ELASTIC_PORT:-9200}}"
CONNECT_URL="${CONNECT_URL:-http://localhost:${CDC_CONNECT_PORT:-8083}}"

wait_for_http() {
  local url="$1"
  local name="$2"
  local retries="${3:-60}"

  echo "Waiting for $name at $url"
  for ((i=1; i<=retries; i++)); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      echo "$name is ready"
      return 0
    fi
    sleep 2
  done

  echo "Timed out waiting for $name" >&2
  return 1
}

echo "Starting CDC stack (Postgres logical replication, Elasticsearch, Kafka, Connect)..."
if docker ps -a --format '{{.Names}}' | grep -qx 'search_elasticsearch'; then
  echo "Detected existing CDC containers from another compose invocation; skipping compose up."
else
  docker compose -p "$DOCKER_COMPOSE_PROJECT" -f "$COMPOSE_FILE" up -d
fi

wait_for_http "$ELASTIC_URL" "Elasticsearch"
wait_for_http "$CONNECT_URL/connectors" "Kafka Connect"

echo "Setting up PostgreSQL logical replication for CDC..."
# Wait for restaurant_db to be healthy
docker compose -p "$DOCKER_COMPOSE_PROJECT" -f "$BASE_COMPOSE_FILE" -f "$RESTAURANT_COMPOSE_FILE" up -d restaurant_db >/dev/null 2>&1 || true
sleep 5

# Create publication if it doesn't exist
docker compose -p "$DOCKER_COMPOSE_PROJECT" -f "$BASE_COMPOSE_FILE" -f "$RESTAURANT_COMPOSE_FILE" exec -T restaurant_db psql \
  -U "${RESTAURANT_POSTGRES_USER:-restaurant}" \
  -d "${RESTAURANT_POSTGRES_DB:-mrfood_restaurant}" \
  -c "CREATE PUBLICATION restaurant_search_publication FOR TABLE restaurants;" 2>/dev/null || \
  echo "  (publication may already exist)"

echo "Ensuring Elasticsearch index 'restaurants' exists..."
if ! curl -fsS -I "$ELASTIC_URL/restaurants" >/dev/null 2>&1; then
  curl -fsS -X PUT "$ELASTIC_URL/restaurants" \
    -H 'Content-Type: application/json' \
    -d '{
      "settings": {
        "number_of_shards": 1,
        "number_of_replicas": 0
      },
      "mappings": {
        "properties": {
          "id": {"type": "long"},
          "name": {
            "type": "text",
            "fields": {
              "keyword": {"type": "keyword"}
            }
          },
          "address": {"type": "text"},
          "categories": {"type": "keyword"},
          "location": {"type": "geo_point"},
          "latitude": {"type": "double"},
          "longitude": {"type": "double"},
          "media_url": {"type": "keyword"},
          "max_slots": {"type": "integer"},
          "owner_id": {"type": "long"},
          "owner_name": {"type": "text"},
          "sponsor_tier": {"type": "integer"}
        }
      }
    }' >/dev/null
fi

CONNECT_URL="$CONNECT_URL" "$SCRIPT_DIR/register-connectors.sh"

echo "CDC bootstrap finished."
echo "- Elasticsearch: $ELASTIC_URL"
echo "- Kafka Connect: $CONNECT_URL"
echo "- To see connectors: curl -s $CONNECT_URL/connectors | cat"

