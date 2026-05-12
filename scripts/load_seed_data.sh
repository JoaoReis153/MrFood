#!/usr/bin/env bash
# load_seed_data.sh — Import seed CSVs from GCS into Cloud SQL.
#
# Usage:
#   ./scripts/load_seed_data.sh [--dry-run]
#
# Prerequisites:
#   gcloud auth application-default login
#   gcloud config set project mrfood-490623
#
set -euo pipefail

# ── Config ────────────────────────────────────────────────────────────────────
PROJECT_ID="mrfood-490623"
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
  "processed_data/auth/app_user.csv|mrfood-auth-pg|mrfood_auth|app_user|user_id,username,password,email"
  "processed_data/restaurant/restaurants.csv|mrfood-restaurant-pg|mrfood_restaurant|restaurants|id,name,latitude,longitude,address,opening_time,closing_time,media_url,max_slots,owner_id,owner_name,sponsor_tier"
  "processed_data/restaurant/restaurant_categories.csv|mrfood-restaurant-pg|mrfood_restaurant|restaurant_categories|restaurant_id,category"
  "processed_data/review/review.csv|mrfood-review-pg|mrfood_review|review|review_id,restaurant_id,user_id,comment,rating,created_at"
)

# ── Main loop ─────────────────────────────────────────────────────────────────
for entry in "${IMPORTS[@]}"; do
  IFS='|' read -r gcs_object instance db table columns <<<"${entry}"

  echo "── ${gcs_object} ──────────────────────────────────────────────────"
  echo "   instance=${instance}  db=${db}  table=${table}"

  if $DRY_RUN; then
    echo "  [dry-run] gcloud sql import csv ${instance} gs://${BUCKET}/${gcs_object} --database=${db} --table=${table} --columns=${columns} --project=${PROJECT_ID} --quiet"
  else
    gcloud sql import csv "${instance}" \
      "gs://${BUCKET}/${gcs_object}" \
      --database="${db}" \
      --table="${table}" \
      --columns="${columns}" \
      --project="${PROJECT_ID}" \
      --quiet
    echo "   ✓ done"
  fi

  echo ""
done

echo "✓ All seed data imported."
