#!/usr/bin/env bash
set -euo pipefail

REGISTRY_REPO="europe-southwest1-docker.pkg.dev/mrfood-490623/mrfood-repo"

usage() {
  cat <<'EOF'
Usage: build_and_push_images.sh <version> [--dry-run]

Builds and pushes one image per service folder that contains a Dockerfile under services/.

Examples:
  ./services/build_and_push_images.sh v1.0.1
  ./services/build_and_push_images.sh v1.0.1 --dry-run
EOF
}

is_valid_tag() {
  local tag="$1"
  [[ "$tag" =~ ^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$ ]]
}

main() {
  if [[ $# -lt 1 || $# -gt 2 ]]; then
    usage
    exit 1
  fi

  local version="$1"
  local dry_run="false"

  if [[ $# -eq 2 ]]; then
    if [[ "$2" != "--dry-run" ]]; then
      usage
      exit 1
    fi
    dry_run="true"
  fi

  if ! is_valid_tag "$version"; then
    echo "Error: invalid Docker tag '$version'." >&2
    exit 1
  fi

  if ! command -v docker >/dev/null 2>&1; then
    echo "Error: docker is not installed or not in PATH." >&2
    exit 1
  fi

  local script_dir
  script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  if [[ ! -d "$script_dir/../kubernetes/values" ]]; then
    echo "Error: Kubernetes values directory not found at $script_dir/../kubernetes/values" >&2
    exit 1
  fi
  local values_dir
  values_dir="$(cd "$script_dir/../kubernetes/values" && pwd)"

  mapfile -t dockerfiles < <(find "$script_dir" -mindepth 2 -maxdepth 2 -type f -name Dockerfile | sort)

  if [[ ${#dockerfiles[@]} -eq 0 ]]; then
    echo "Error: no Dockerfiles found under $script_dir/*/." >&2
    exit 1
  fi

  for dockerfile in "${dockerfiles[@]}"; do
    local service_dir service_name image
    service_dir="$(dirname "$dockerfile")"
    service_name="$(basename "$service_dir")"
    image="${REGISTRY_REPO}/${service_name}:${version}"

    echo "[$service_name] docker build -t $image $service_dir"
    if [[ "$dry_run" != "true" ]]; then
      docker build -t "$image" "$service_dir"
    fi

    echo "[$service_name] docker push $image"
    if [[ "$dry_run" != "true" ]]; then
      docker push "$image"
    fi

    local values_file
    values_file="$values_dir/${service_name}.yaml"
    if [[ -f "$values_file" ]]; then
      echo "[$service_name] update $values_file image -> $image"
      if [[ "$dry_run" != "true" ]]; then
        sed -i "s|^image:.*$|image: ${image}|" "$values_file"
      fi
    else
      echo "[$service_name] warning: values file not found at $values_file"
    fi
  done

  echo "Done: processed ${#dockerfiles[@]} services."
}

main "$@"

