# Search CDC Bootstrap

This folder provides a local CDC pipeline so `restaurant` DB changes flow into Elasticsearch, while `search` service remains read-only.

## What It Starts

- Postgres (`restaurant_cdc_db`) with logical replication enabled
- Elasticsearch
- Kafka + Zookeeper
- Kafka Connect with:
  - Debezium Postgres source connector
  - Elasticsearch sink connector

## Start

```bash
cd services/cdc
./bootstrap_cdc.sh
```

## Stop

```bash
docker compose -f services/docker-compose.cdc.yml down
```

## Smoke Test

Run an end-to-end check (Connect + connectors + Postgres insert + Elasticsearch indexing):

```bash
cd services/cdc
./smoke_test.sh
```

Optional flags:

- `SKIP_REGISTER_CONNECTORS=1` to skip connector upsert
- `INDEX_TIMEOUT_SECONDS=120` to wait longer for indexing
- `CONNECT_URL` / `ELASTIC_URL` to override endpoints
- `ELASTIC_INDEX=restaurants` to target a specific index instead of `_all`

## Notes

- This bootstrap tracks only `public.restaurants` by default.
- If your search mapping/query relies on categories from another table, extend the CDC flow with an enrichment step.

