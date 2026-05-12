#!/usr/bin/env bash
set -euo pipefail

terraform state rm module.cloudsql_foundation.google_service_networking_connection.private_vpc_connection 2>/dev/null || true

terraform destroy "$@"
