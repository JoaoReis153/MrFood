#!/bin/bash

# Check if service name was provided
if [ -z "$1" ]; then
  echo "Usage: $0 <service-name>"
  exit 1
fi

SERVICES_STRUCTURE="services"
BASE="$1"
ROOT="$SERVICES_STRUCTURE/$BASE"

# Create directories
mkdir -p $ROOT/{cmd,internal/api/rest/handler,internal/api/rest/router,internal/app,internal/repository,internal/service,pkg,scripts}

# Create files
touch \
$ROOT/cmd/main.go \
$ROOT/internal/api/rest/rest.go \
$ROOT/internal/api/rest/handler/handler.go \
$ROOT/internal/api/rest/router/router.go \
$ROOT/internal/app/app.go \
$ROOT/internal/repository/repository.go \
$ROOT/internal/service/service.go \
$ROOT/pkg/models.go \
$ROOT/pkg/response.go \
$ROOT/Dockerfile \
$ROOT/Makefile \
$ROOT/go.mod \
$ROOT/go.sum \
$ROOT/LICENSE \
$ROOT/README.md

echo "Service '$BASE' created successfully at $ROOT"