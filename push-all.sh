#!/bin/bash
set -e

# -------------------------------------------------------
# Configuration
# -------------------------------------------------------
PROJECT_ID="mrfood-490623"
REGION="europe-southwest1"
REPOSITORY="mrfood-repo"
TAG="${1:-v1.0.1}"  # Pass a tag as argument, defaults to v1.0.1
                     # Usage: ./push-all.sh v1.0.2

REGISTRY="${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPOSITORY}"

# -------------------------------------------------------
# Authenticate
# -------------------------------------------------------
echo "🔐 Authenticating with Artifact Registry..."
gcloud auth configure-docker "${REGION}-docker.pkg.dev" --quiet

# -------------------------------------------------------
# Ensure buildx builder exists for cross-platform builds
# -------------------------------------------------------
echo "🔧 Setting up buildx for linux/amd64..."
docker buildx inspect amd64-builder > /dev/null 2>&1 || \
  docker buildx create --name amd64-builder --use
docker buildx use amd64-builder

# -------------------------------------------------------
# Find all services
# -------------------------------------------------------
SERVICES_DIR="$(dirname "$0")/services"

if [ ! -d "$SERVICES_DIR" ]; then
  echo "❌ services/ directory not found. Run this script from the repo root."
  exit 1
fi

SERVICES=$(find "$SERVICES_DIR" -maxdepth 2 -name "Dockerfile" | sed 's|/Dockerfile||' | xargs -I{} basename {})

echo ""
echo "📦 Found services:"
for SERVICE in $SERVICES; do
  echo "   - $SERVICE"
done
echo ""

# -------------------------------------------------------
# Build and push each service
# -------------------------------------------------------
SUCCESS=()
FAILED=()

for SERVICE in $SERVICES; do
  IMAGE="${REGISTRY}/${SERVICE}:${TAG}"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "🔨 Building ${SERVICE}:${TAG} for linux/amd64..."

  if docker buildx build \
    --platform linux/amd64 \
    --tag "${IMAGE}" \
    --push \
    "${SERVICES_DIR}/${SERVICE}"; then
    echo "✅ ${SERVICE} done"
    SUCCESS+=("$SERVICE")
  else
    echo "❌ Failed to build/push ${SERVICE}"
    FAILED+=("$SERVICE")
  fi

  echo ""
done

# -------------------------------------------------------
# Summary
# -------------------------------------------------------
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📊 Summary"
echo ""
echo "✅ Succeeded (${#SUCCESS[@]}):"
for S in "${SUCCESS[@]}"; do echo "   - $S"; done

if [ ${#FAILED[@]} -gt 0 ]; then
  echo ""
  echo "❌ Failed (${#FAILED[@]}):"
  for F in "${FAILED[@]}"; do echo "   - $F"; done
  exit 1
fi

echo ""
echo "🚀 All images pushed to ${REGISTRY}"