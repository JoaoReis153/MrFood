#!/bin/bash

# Check if service name was provided
if [ -z "$1" ]; then
  echo "Usage: $0 <service-name>"
  exit 1
fi

SERVICES_STRUCTURE="services"
BASE="$1"
ROOT="$SERVICES_STRUCTURE/$BASE"
TEMPLATE_DIR="template/microservice-template"

# Check if template directory exists
if [ ! -d "$TEMPLATE_DIR" ]; then
  echo "Error: Template directory '$TEMPLATE_DIR' not found!"
  exit 1
fi

# Create base directory
mkdir -p "$ROOT"

# Replace placeholders in template files
export SERVICE_NAME="$BASE"
find "$TEMPLATE_DIR" -name "*.tmpl" -type f | while read -r tmpl; do
  rel_path=${tmpl#"$TEMPLATE_DIR/"}
  target="$ROOT/${rel_path%.tmpl}"
  mkdir -p "$(dirname "$target")"
  
  awk -v name="$SERVICE_NAME" '{gsub(/\{\{\.ServiceName\}\}/, name); print}' "$tmpl" > "$target"
  echo "Created $target"
done

set -e

cd $ROOT/internal/api/grpc

protoc --go_out=. --go-grpc_out=. proto/protofile.proto

echo "Protobuf generation done"

cd "../../.." || exit 1

go mod tidy

echo "Service '$BASE' created successfully at $ROOT"
echo "✅ Ready! Run: cd $ROOT && make build"
