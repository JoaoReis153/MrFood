# Config
PROJECT_NAME := mrfood
COMPOSE_FILE := services/docker-compose.yml
TEST_PACKAGES := ./services/auth/... ./services/booking/... ./services/restaurant/... ./services/review/... ./services/sponsor/... ./services/observability/...

# Load non-sensitive config (committed) and secrets (git-ignored)
-include services/config.env
-include services/.env

# Load config and secrets for docker compose interpolation
ENV_FILES := --env-file services/config.env
ENV_FILES += $(if $(wildcard services/.env),--env-file services/.env,)

DC := docker compose -p $(PROJECT_NAME) -f $(COMPOSE_FILE) $(ENV_FILES)
PYTHON := $(if $(wildcard scripts/.venv/bin/python),scripts/.venv/bin/python,python3)
CSV_SERVICES ?= all
CSV_ROWS ?= 200
CSV_FULL ?=

# Detect if using podman
IS_PODMAN := $(shell docker --version 2>/dev/null | grep -i podman)

PULL_FLAG :=
ifeq ($(IS_PODMAN),)
	PULL_FLAG := --pull=missing
endif

.PHONY: help create_env generate-csv generate-csv-auth generate-csv-restaurant generate-csv-review load-auth load-restaurant load-reviews load-all setup setup-full build run run-full bootstrap-search stop down restart logs test clean clean-volumes clean-all test test-bruno

help:
	@echo "MrFood Make Commands"
	@echo ""
	@echo "Setup & Data:"
	@echo "  make create_env                         - Create secret .env files from env.tmpl"
	@echo "  (config.env is already committed — no setup needed)"
	@echo "  make setup                              - Start core services and load data (no search/CDC)"
	@echo "  make setup-full                         - Start all services including search/CDC stack"
	@echo "  make generate-csv                       - Generate CSV seed data (default 200 rows)"
	@echo "  make generate-csv CSV_FULL=1            - Generate CSV seed data (full dataset)"
	@echo "  make load-reviews                       - Load review seed data into database"
	@echo "  make load-all                           - Load all seed data into databases"
	@echo ""
	@echo "Service Management:"
	@echo "  make run                                - Start core services (detached)"
	@echo "  make run-full                           - Start all services including search/CDC (detached)"
	@echo "  make bootstrap-search                   - Create ES index and register CDC connectors"
	@echo "  make stop                               - Stop services"
	@echo "  make down                               - Stop and remove services"
	@echo "  make restart                            - Restart services"
	@echo "  make logs                               - View service logs"
	@echo ""
	@echo "Build & Test:"
	@echo "  make build                              - Build service images"
	@echo "  make test                               - Run all Go tests"
	@echo ""
	@echo "Cleanup:"
	@echo "  make clean                              - Remove containers & images"
	@echo "  make clean-all                          - Full reset (all containers, images, volumes)"

## Run Bruno REST CI smoke tests
test-bruno:
	mkdir -p tests/mrfood-api/reports
	cd tests/mrfood-api/collections/users && npx --yes @usebruno/cli@latest run -r --env development --tests-only --reporter-junit ../../reports/users-junit.xml --reporter-json ../../reports/users-report.json
	cd tests/mrfood-api/collections/restaurants && npx --yes @usebruno/cli@latest run -r --env development --tests-only --reporter-junit ../../reports/restaurants-junit.xml --reporter-json ../../reports/restaurants-report.json
	cd tests/mrfood-api/collections/reservations && npx --yes @usebruno/cli@latest run -r --env development --tests-only --reporter-junit ../../reports/reservations-junit.xml --reporter-json ../../reports/reservations-report.json
	cd tests/mrfood-api/collections/reviews && npx --yes @usebruno/cli@latest run -r --env development --tests-only --reporter-junit ../../reports/reviews-junit.xml --reporter-json ../../reports/reviews-report.json
	@bash services/cdc/seed_elasticsearch.sh
	cd tests/mrfood-api/collections/search && npx --yes @usebruno/cli@latest run -r --env development --tests-only --reporter-junit ../../reports/search-junit.xml --reporter-json ../../reports/search-report.json
	cd tests/mrfood-api/collections/payment && npx --yes @usebruno/cli@latest run -r --env development --tests-only --reporter-junit ../../reports/payment-junit.xml --reporter-json ../../reports/payment-report.json
	cd tests/mrfood-api/collections/sponsor && npx --yes @usebruno/cli@latest run -r --env development --tests-only --reporter-junit ../../reports/sponsor-junit.xml --reporter-json ../../reports/sponsor-report.json

## Build services

# ============================================================================
# ENVIRONMENT
# ============================================================================

## Create services/.env from services/env.tmpl
## config.env (non-sensitive) is already committed — no action needed for it.
create_env:
	@if [ -f services/.env ]; then \
		echo "services/.env already exists."; \
	else \
		cp services/env.tmpl services/.env; \
		echo "Created services/.env"; \
	fi
	@echo "Fill in secret values in services/.env before running docker compose."

# ============================================================================
# DATA GENERATION
# ============================================================================

## Generate CSV seed data (default 200 rows, use CSV_FULL=1 for full dataset)
generate-csv:
	$(PYTHON) scripts/process_data.py --services $(CSV_SERVICES) $(if $(CSV_ROWS),--rows $(CSV_ROWS),) $(if $(CSV_FULL),--full,)

# ============================================================================
# DATA LOADING
# ============================================================================

load-csvs:
	@$(MAKE) --no-print-directory -j 3 load-auth load-restaurant load-reviews 
	@echo "✓ All data loaded successfully"

## Core setup: start services (no search/CDC) and load data
setup: run load-csvs
	@echo "✓ Setup complete! Services running and data loaded"

## Full setup: start all services including search/CDC stack and load data
setup-full: run-full search-bootstrap load-csvs
	@echo "✓ Full setup complete! Services running with search and data loaded"

# ============================================================================
# SERVICE MANAGEMENT
# ============================================================================

## Build service images (parallel, BuildKit enabled)
build:
	DOCKER_BUILDKIT=1 $(DC) build --parallel

build-no-cache:
	DOCKER_BUILDKIT=1 $(DC) build --no-cache --parallel

## Start core services (detached)
run:
	$(DC) up -d $(PULL_FLAG)

## Start all services including search/CDC profile (detached)
run-full:
	$(DC) --profile search up -d $(PULL_FLAG)

## Stop services
stop:
	$(DC) stop

## Stop and remove services
down:
	$(DC) down

down-volumes:
	$(DC) down -v

## Restart services
restart: down run

## View service logs
logs:
	$(DC) logs -f

logs-dump:
	$(DC) logs --tail=500

# ============================================================================
# TESTING & BUILDING
# ============================================================================

## Run all Go tests
test:
	go test -v -race $(TEST_PACKAGES)

# ============================================================================
# CLEANUP
# ============================================================================

## Remove containers + images + volumes (deletes DB data)
clean:
	$(DC) down --rmi local --volumes --remove-orphans

## Full reset (containers, images, volumes)
clean-all:
	$(DC) down --rmi all --volumes --remove-orphans


# ============================================================================
# SEARCH
# ============================================================================

## Start only search-related services (ES, Kafka, Connect) + bootstrap
search-run:
	$(DC) --profile search up -d $(PULL_FLAG)
	@$(MAKE) --no-print-directory search-bootstrap

## Lightweight ES setup for CI: create index only, no CDC connectors.
## seed_elasticsearch.sh is called automatically inside test-bruno.
search-seed:
	@echo "Waiting for Elasticsearch..."
	@curl -fsS "http://localhost:$(CDC_ELASTIC_PORT)/_cluster/health?wait_for_status=yellow&timeout=60s" > /dev/null
	@echo "✔ Elasticsearch ready"

	@HTTP_CODE=$$(curl -sS -o /tmp/es-response.json -w "%{http_code}" \
		-X PUT "http://localhost:$(CDC_ELASTIC_PORT)/$(ELASTICSEARCH_INDEX)" \
		-H 'Content-Type: application/json' \
		-d @services/cdc/mappings/restaurants.json || true); \
	if [ "$$HTTP_CODE" = "400" ]; then \
		echo "✔ Index already exists"; \
	elif [ "$$HTTP_CODE" != "200" ] && [ "$$HTTP_CODE" != "201" ]; then \
		echo "❌ Index creation failed (HTTP $$HTTP_CODE)"; cat /tmp/es-response.json; exit 1; \
	else \
		echo "✔ Index created (HTTP $$HTTP_CODE)"; \
	fi

## Bootstrap ES index and register CDC connectors
search-bootstrap:
	@echo "Waiting for Elasticsearch..."
	@curl -fsS "http://localhost:$(CDC_ELASTIC_PORT)/_cluster/health?wait_for_status=yellow&timeout=60s" > /dev/null
	@echo "✔ Elasticsearch ready"

	@HTTP_CODE=$$(curl -sS -o /tmp/es-response.json -w "%{http_code}" \
		-X PUT "http://localhost:$(CDC_ELASTIC_PORT)/$(ELASTICSEARCH_INDEX)" \
		-H 'Content-Type: application/json' \
		-d @services/cdc/mappings/restaurants.json || true); \
	if [ "$$HTTP_CODE" = "400" ]; then \
		echo "✔ Index already exists"; \
	elif [ "$$HTTP_CODE" != "200" ] && [ "$$HTTP_CODE" != "201" ]; then \
		echo "❌ Index creation failed (HTTP $$HTTP_CODE)"; cat /tmp/es-response.json; exit 1; \
	else \
		echo "✔ Index created (HTTP $$HTTP_CODE)"; \
	fi

	@bash services/cdc/register-connectors.sh
	@bash services/cdc/seed_elasticsearch.sh

## Stop search services
search-stop:
	$(DC) --profile search stop elasticsearch zookeeper kafka connect search

## Stop and remove search services
search-down:
	$(DC) --profile search rm -sf elasticsearch zookeeper kafka connect search

## Tail logs for search services only
search-logs:
	$(DC) --profile search logs -f elasticsearch zookeeper kafka connect search

## Full reset of search (removes elastic_data volume)
search-clean:
	$(DC) --profile search rm -sf elasticsearch zookeeper kafka connect search
	docker volume rm -f $(PROJECT_NAME)_elastic_data