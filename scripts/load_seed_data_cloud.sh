#!/usr/bin/env bash
# load_seed_data_cloud.sh — Import seed CSVs from GCS into Cloud SQL.
#
# Usage:
#   ./scripts/load_seed_data_cloud.sh [--dry-run]
#
# Prerequisites:
#   gcloud auth application-default login
#   gcloud config set project mrfood-490623
#   make generate-csv + upload processed_data/ to gs://BUCKET/processed_data/
#
set -euo pipefail

# ── Config ────────────────────────────────────────────────────────────────────
PROJECT_ID="${TF_VAR_project_id:?TF_VAR_project_id is not set}"
INSTANCE="mrfood-pg"
BUCKET="kaggle_bucket_6194"
DRY_RUN=false

if [[ "${1:-}" == "--dry-run" ]]; then
  DRY_RUN=true
  echo "▶ Dry-run mode — no imports will be executed"
fi

# ── CSV → (gcs_object, database, table, columns) mapping ─────────────────────
# Format: "gcs_object|db_name|table_name|col1,col2,..."
IMPORTS=(
  "processed_data/restaurant/restaurants.csv|mrfood-pg|mrfood_restaurant|restaurants|id,name,latitude,longitude,address,opening_time,closing_time,media_url,max_slots,owner_id,owner_name,sponsor_tier"
  "processed_data/restaurant/restaurant_categories.csv|mrfood-pg|mrfood_restaurant|restaurant_categories|restaurant_id,category"
  "processed_data/review/review.csv|mrfood-pg|mrfood_review|review|review_id,restaurant_id,user_id,comment,rating,created_at"
)

# ── Main loop ─────────────────────────────────────────────────────────────────
for entry in "${IMPORTS[@]}"; do
  IFS='|' read -r gcs_object instance db table columns <<<"${entry}"

  echo "── ${gcs_object} ──────────────────────────────────────────────────"
  echo "   instance=${instance}  db=${db}  table=${table}"

  local_file="/tmp/$(basename "${gcs_object}")"
  tmp_object="tmp/$(basename "${gcs_object}")"

  if $DRY_RUN; then
    echo "  [dry-run] strip header → gs://${BUCKET}/${tmp_object}"
    echo "  [dry-run] gcloud sql import csv ${instance} gs://${BUCKET}/${tmp_object} --database=${db} --table=${table} --columns=${columns} --project=${PROJECT_ID} --quiet"
  else
    echo "   downloading → ${local_file}"
    gsutil -q cp "gs://${BUCKET}/${gcs_object}" "${local_file}"

    echo "   stripping header → gs://${BUCKET}/${tmp_object}"
    tail -n +2 "${local_file}" > "${local_file}.noheader"
    gsutil -q cp "${local_file}.noheader" "gs://${BUCKET}/${tmp_object}"
    rm "${local_file}" "${local_file}.noheader"

    gcloud sql import csv "${instance}" \
      "gs://${BUCKET}/${tmp_object}" \
      --database="${db}" \
      --table="${table}" \
      --columns="${columns}" \
      --project="${PROJECT_ID}" \
      --quiet

    gsutil -q rm "gs://${BUCKET}/${tmp_object}"
    echo "   ✓ done"
  fi

  echo ""
done

echo "✓ All cloud seed data imported."
