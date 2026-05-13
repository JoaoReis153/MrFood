#!/usr/bin/env bash
# load_seed_data_local.sh — Load seed data into local Docker containers.
#
# Usage:
#   ./scripts/load_seed_data_local.sh [--dry-run]
#
# Prerequisites:
#   make generate-csv
#   make run  (containers must be running)
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
DATA_DIR="${SCRIPT_DIR}/processed_data"

# ── Load env ──────────────────────────────────────────────────────────────────
# shellcheck source=/dev/null
source "${ROOT_DIR}/services/config.env"
if [[ -f "${ROOT_DIR}/services/.env" ]]; then
  # shellcheck source=/dev/null
  set -o allexport; source "${ROOT_DIR}/services/.env"; set +o allexport
fi

DOCKER_PROJECT="${DOCKER_PROJECT:-mrfood}"
KEYCLOAK_URL="${KEYCLOAK_URL:-http://localhost:8080}"
KEYCLOAK_REALM="mrfood"
KEYCLOAK_ADMIN_USER="${KEYCLOAK_ADMIN_USER:-admin}"
KEYCLOAK_ADMIN_PASS="${KEYCLOAK_ADMIN_PASS:-admin}"
DRY_RUN=false

if [[ "${1:-}" == "--dry-run" ]]; then
  DRY_RUN=true
  echo "▶ Dry-run mode — no imports will be executed"
fi

# ── Helpers ───────────────────────────────────────────────────────────────────
# pg_copy <container> <user> <db> <table> <csv> [columns]
# columns: optional comma-separated list (required when table has a SERIAL pk not in the CSV)
pg_copy() {
  local container="$1" user="$2" db="$3" table="$4" csv="$5" columns="${6:-}"
  local target="${table}"
  [[ -n "${columns}" ]] && target="${table}(${columns})"

  if [[ ! -f "${csv}" ]]; then
    echo "  ⚠  ${csv} not found — skipping"
    return
  fi

  echo "── ${csv##*/} → ${db}.${table} ──────────────────────────────────────"

  if $DRY_RUN; then
    local rows
    rows=$(( $(wc -l < "${csv}") - 1 ))
    echo "  [dry-run] would COPY ${rows} rows into ${container}:${db}.${target}"
    echo ""
    return
  fi

  docker exec -i "${container}" \
    env PGPASSWORD="${PGPASSWORD}" \
    psql -U "${user}" -d "${db}" \
    -c "TRUNCATE ${table} RESTART IDENTITY CASCADE" \
    -c "\copy ${target} FROM STDIN CSV HEADER" \
    < "${csv}"

  echo "   ✓ done"
  echo ""
}

# ── Keycloak users ────────────────────────────────────────────────────────────
load_keycloak_users() {
  local users_csv="${DATA_DIR}/auth/users.csv"

  if [[ ! -f "${users_csv}" ]]; then
    echo "⚠  ${users_csv} not found — skipping Keycloak import (run make generate-csv first)"
    return
  fi

  echo "── Keycloak users ──────────────────────────────────────────────────────"
  echo "   source=${users_csv}  realm=${KEYCLOAK_REALM}"

  if $DRY_RUN; then
    local count
    count=$(( $(wc -l < "${users_csv}") - 1 ))
    echo "  [dry-run] would import ${count} users into Keycloak at ${KEYCLOAK_URL}"
    echo ""
    return
  fi

  echo "   obtaining admin token..."
  local token
  token=$(curl -sf \
    -d "grant_type=password&client_id=admin-cli&username=${KEYCLOAK_ADMIN_USER}&password=${KEYCLOAK_ADMIN_PASS}" \
    "${KEYCLOAK_URL}/realms/master/protocol/openid-connect/token" \
    | python3 -c "import json,sys; print(json.load(sys.stdin)['access_token'])")

  echo "   importing users..."
  python3 - <<EOF
import csv, json, sys, urllib.request, urllib.error

users_csv  = "${users_csv}"
base_url   = "${KEYCLOAK_URL}/admin/realms/${KEYCLOAK_REALM}/users"
token      = "${token}"
headers    = {"Content-Type": "application/json", "Authorization": f"Bearer {token}"}

created = skipped = 0
with open(users_csv, newline="") as f:
    for row in csv.DictReader(f):
        payload = {
            "username": row["username"],
            "email":    row["email"],
            "enabled":  True,
            "credentials": [
                {"type": "password", "value": row["password"], "temporary": False}
            ],
        }
        data = json.dumps(payload).encode()
        req  = urllib.request.Request(base_url, data=data, headers=headers, method="POST")
        try:
            urllib.request.urlopen(req)
            created += 1
        except urllib.error.HTTPError as e:
            if e.code == 409:
                skipped += 1
            else:
                print(f"  error creating {row['username']}: {e.code} {e.read().decode()}", file=sys.stderr)

print(f"  created={created}  skipped(already exist)={skipped}")
EOF

  echo "   ✓ done"
  echo ""
}

# ── Auth: Keycloak ────────────────────────────────────────────────────────────
load_keycloak_users

# ── Restaurant ────────────────────────────────────────────────────────────────
PGPASSWORD="${RESTAURANT_POSTGRES_PASSWORD}" \
  pg_copy "${DOCKER_PROJECT}-restaurant_db-1" \
          "${RESTAURANT_POSTGRES_USER}" \
          "${RESTAURANT_POSTGRES_DB}" \
          "restaurants" \
          "${DATA_DIR}/restaurant/restaurants.csv"

PGPASSWORD="${RESTAURANT_POSTGRES_PASSWORD}" \
  pg_copy "${DOCKER_PROJECT}-restaurant_db-1" \
          "${RESTAURANT_POSTGRES_USER}" \
          "${RESTAURANT_POSTGRES_DB}" \
          "restaurant_categories" \
          "${DATA_DIR}/restaurant/restaurant_categories.csv" \
          "restaurant_id,category"

# ── Review ────────────────────────────────────────────────────────────────────
PGPASSWORD="${REVIEW_POSTGRES_PASSWORD}" \
  pg_copy "${DOCKER_PROJECT}-review_db-1" \
          "${REVIEW_POSTGRES_USER}" \
          "${REVIEW_POSTGRES_DB}" \
          "review" \
          "${DATA_DIR}/review/review.csv" \
          "review_id,restaurant_id,user_id,comment,rating,created_at"

echo "✓ All local seed data imported."
