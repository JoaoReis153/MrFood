#!/usr/bin/env bash
# bootstrap_cloud.sh — Cloud SQL setup: apply schemas then import seed CSVs.
#
# Safe to re-run — seed tables are truncated before each import.
#
# Usage:
#   ./scripts/bootstrap_cloud.sh [--dry-run]
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
source "${REPO_ROOT}/gcp.env"

INSTANCE="${CLOUDSQL_INSTANCE}"
BUCKET="${GCS_SEED_BUCKET}"
DRY_RUN=false

for arg in "$@"; do
  case "${arg}" in
    --dry-run) DRY_RUN=true ;;
  esac
done

$DRY_RUN && echo "▶ Dry-run mode — no changes will be made"

run() {
  if $DRY_RUN; then
    echo "  [dry-run] $*"
  else
    "$@"
  fi
}

# ---------------------------------------------------------------------------
# 1. Schemas — upload each db_setup.sql to GCS then import into Cloud SQL
# ---------------------------------------------------------------------------
# Format: "service_name|database_name"
SCHEMAS=(
  "restaurant|mrfood_restaurant"
  "review|mrfood_review"
  "booking|mrfood_booking"
  "payment|mrfood_payment"
  "sponsor|mrfood_sponsor"
)

echo ""
echo "━━━ Applying schemas ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
for entry in "${SCHEMAS[@]}"; do
  IFS='|' read -r svc db <<<"${entry}"
  sql_file="${REPO_ROOT}/services/${svc}/db_setup.sql"
  gcs_path="tmp/schema_${svc}.sql"

  echo "── ${svc} → ${db}"

  if $DRY_RUN; then
    echo "  [dry-run] gsutil cp ${sql_file} gs://${BUCKET}/${gcs_path}"
    echo "  [dry-run] gcloud sql import sql ${INSTANCE} gs://${BUCKET}/${gcs_path} --database=${db} --quiet"
  else
    gsutil -q cp "${sql_file}" "gs://${BUCKET}/${gcs_path}"
    gcloud sql import sql "${INSTANCE}" \
      "gs://${BUCKET}/${gcs_path}" \
      --database="${db}" \
      --project="${GCP_PROJECT_ID}" \
      --quiet
    gsutil -q rm "gs://${BUCKET}/${gcs_path}"
    echo "  ✓ schema applied"
  fi
  echo ""
done

# ---------------------------------------------------------------------------
# 2. Truncate seed tables — FK-safe order so re-runs are always clean
# ---------------------------------------------------------------------------
# Format: "database|SQL"
TRUNCATES=(
  "mrfood_restaurant|TRUNCATE restaurant_categories, restaurants RESTART IDENTITY CASCADE;"
  "mrfood_review|TRUNCATE review RESTART IDENTITY CASCADE;"
)

echo "━━━ Truncating seed tables ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
for entry in "${TRUNCATES[@]}"; do
  IFS='|' read -r db sql <<<"${entry}"
  tmp_object="tmp/truncate_${db}.sql"

  echo "── ${db}"

  if $DRY_RUN; then
    echo "  [dry-run] ${sql}"
  else
    echo "${sql}" | gsutil -q cp - "gs://${BUCKET}/${tmp_object}"
    gcloud sql import sql "${INSTANCE}" \
      "gs://${BUCKET}/${tmp_object}" \
      --database="${db}" \
      --project="${GCP_PROJECT_ID}" \
      --quiet
    gsutil -q rm "gs://${BUCKET}/${tmp_object}"
    echo "  ✓ truncated"
  fi
  echo ""
done

# ---------------------------------------------------------------------------
# 3. Seed CSVs — strip header, stage in GCS, import into Cloud SQL
# ---------------------------------------------------------------------------
# Format: "local_path|database|table|col1,col2,..."
IMPORTS=(
  "scripts/processed_data/restaurant/restaurants.csv|mrfood_restaurant|restaurants|id,name,latitude,longitude,address,opening_time,closing_time,media_url,max_slots,owner_id,owner_name,sponsor_tier"
  "scripts/processed_data/restaurant/restaurant_categories.csv|mrfood_restaurant|restaurant_categories|restaurant_id,category"
  "scripts/processed_data/review/review.csv|mrfood_review|review|review_id,restaurant_id,user_id,comment,rating,created_at"
)

echo "━━━ Importing seed CSVs ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
for entry in "${IMPORTS[@]}"; do
  IFS='|' read -r local_path db table columns <<<"${entry}"
  local_csv="${REPO_ROOT}/${local_path}"
  tmp_object="tmp/seed_$(basename "${local_path}")"
  tmp_file="/tmp/seed_$(basename "${local_path}").noheader"

  echo "── ${local_path} → ${db}.${table}"

  if $DRY_RUN; then
    echo "  [dry-run] gcloud sql import csv ${INSTANCE} gs://${BUCKET}/${tmp_object} --database=${db} --table=${table} --columns=${columns}"
  else
    tail -n +2 "${local_csv}" > "${tmp_file}"
    gsutil -q cp "${tmp_file}" "gs://${BUCKET}/${tmp_object}"
    rm "${tmp_file}"

    gcloud sql import csv "${INSTANCE}" \
      "gs://${BUCKET}/${tmp_object}" \
      --database="${db}" \
      --table="${table}" \
      --columns="${columns}" \
      --project="${GCP_PROJECT_ID}" \
      --quiet

    gsutil -q rm "gs://${BUCKET}/${tmp_object}"
    echo "  ✓ imported"
  fi
  echo ""
done

echo "✓ Bootstrap complete."
