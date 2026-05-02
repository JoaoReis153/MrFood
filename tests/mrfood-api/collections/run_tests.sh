#!/usr/bin/env bash

set -euo pipefail

# Resolve script location safely
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

ROOT_DIR="$SCRIPT_DIR"
REPORT_DIR="$ROOT_DIR/reports"

mkdir -p "$REPORT_DIR"

echo "🚀 Starting Bruno tests..."
echo "Root: $ROOT_DIR"
echo "Reports: $REPORT_DIR"
echo "-------------------------------------"

cd "$ROOT_DIR"

for collection in */; do
  # skip non-directories just in case
  [ -d "$collection" ] || continue

  echo ""
  echo "🧪 Running collection: $collection"

  cd "$collection"

  name="${collection%/}"

  npx --yes @usebruno/cli@latest run -r \
    --tests-only \
    --reporter-junit "../../reports/${name}-junit.xml" \
    --reporter-json "../../reports/${name}-report.json"

  cd ..
done

echo ""
echo "✅ All Bruno tests completed!"
echo "📦 Reports generated in: $REPORT_DIR"
