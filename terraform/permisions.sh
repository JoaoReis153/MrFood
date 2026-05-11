#!/bin/bash

set -euo pipefail

PROJECT="mrfood-490623"
BUCKET="gs://mrfood-cloudsql-schema-bootstrap-490623"
INSTANCES=(
  "mrfood-auth-pg"
  "mrfood-payment-pg"
  "mrfood-booking-pg"
  "mrfood-restaurant-pg"
  "mrfood-review-pg"
  "mrfood-sponsor-pg"
)

for instance in "${INSTANCES[@]}"; do
  echo "Getting service account for $instance..."
  SA=$(gcloud sql instances describe "$instance" \
    --project="$PROJECT" \
    --format="value(serviceAccountEmailAddress)")

  echo "Granting storage.objectViewer to $SA on $BUCKET"
  gsutil iam ch serviceAccount:${SA}:roles/storage.objectViewer "$BUCKET"

  echo ""
done

echo "Done! All service accounts granted access."