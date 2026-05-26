#!/usr/bin/env bash
# bootstrap_cloud.sh — First-time Cloud SQL setup: apply schemas then import seed CSVs.
#
# Run once after `terraform apply` creates the Cloud SQL instance and databases.
# Re-running is safe for schemas (CREATE IF NOT EXISTS) but will error on CSV
# imports if data already exists — pass --force to skip CSV import on conflict.
#
# Usage:
#   ./scripts/bootstrap_cloud.sh [--dry-run] [--force]
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
source "${REPO_ROOT}/gcp.env"

INSTANCE="mrfood-pg"
BUCKET="mrfood-cloudsql-schema-bootstrap-${GCP_PROJECT_ID}"
DRY_RUN=false
FORCE=false

for arg in "$@"; do
  case "${arg}" in
    --dry-run) DRY_RUN=true  ;;
    --force)   FORCE=true    ;;
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
# 2. Seed CSVs — strip header, stage in GCS, import into Cloud SQL
# ---------------------------------------------------------------------------
# Format: "gcs_object|database|table|col1,col2,..."
IMPORTS=(
  "processed_data/restaurant/restaurants.csv|mrfood_restaurant|restaurants|id,name,latitude,longitude,address,opening_time,closing_time,media_url,max_slots,owner_id,owner_name,sponsor_tier"
  "processed_data/restaurant/restaurant_categories.csv|mrfood_restaurant|restaurant_categories|restaurant_id,category"
  "processed_data/review/review.csv|mrfood_review|review|review_id,restaurant_id,user_id,comment,rating,created_at"
)

echo "━━━ Importing seed CSVs ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
for entry in "${IMPORTS[@]}"; do
  IFS='|' read -r gcs_object db table columns <<<"${entry}"
  local_file="/tmp/$(basename "${gcs_object}")"
  tmp_object="tmp/seed_$(basename "${gcs_object}")"

  echo "── ${gcs_object} → ${db}.${table}"

  local_csv="${SCRIPT_DIR}/${gcs_object}"

  if $DRY_RUN; then
    echo "  [dry-run] gcloud sql import csv ${INSTANCE} gs://${BUCKET}/${tmp_object} --database=${db} --table=${table} --columns=${columns}"
  else
    tail -n +2 "${local_csv}" > "${local_file}.noheader"
    gsutil -q cp "${local_file}.noheader" "gs://${BUCKET}/${tmp_object}"
    rm "${local_file}.noheader"

    if gcloud sql import csv "${INSTANCE}" \
        "gs://${BUCKET}/${tmp_object}" \
        --database="${db}" \
        --table="${table}" \
        --columns="${columns}" \
        --project="${GCP_PROJECT_ID}" \
        --quiet 2>&1; then
      echo "  ✓ imported"
    else
      if $FORCE; then
        echo "  ⚠ import failed (data may already exist) — skipping (--force)"
      else
        gsutil -q rm "gs://${BUCKET}/${tmp_object}" 2>/dev/null || true
        echo "  ✗ import failed. Re-run with --force to skip on conflict."
        exit 1
      fi
    fi

    gsutil -q rm "gs://${BUCKET}/${tmp_object}" 2>/dev/null || true
  fi
  echo ""
done

echo "✓ Bootstrap complete."
