# Config
PROJECT_NAME := mrfood
COMPOSE_FILE := services/docker-compose.yml
TEST_PACKAGES := ./services/auth/... ./services/booking/... ./services/restaurant/... ./services/review/... ./services/sponsor/... ./services/observability/...

# Load non-sensitive config (committed) and secrets (git-ignored)
-include services/config.env
-include services/.env

# Load all .env files (shared and per-service) for docker compose interpolation
ENV_FILES := --env-file services/config.env
ENV_FILES += $(if $(wildcard services/.env),--env-file services/.env,)
ENV_FILES += $(foreach env,$(wildcard services/*/.env),--env-file $(env))

DC := docker compose -p $(PROJECT_NAME) -f $(COMPOSE_FILE) $(ENV_FILES)
PYTHON := $(if $(wildcard scripts/.venv/bin/python),scripts/.venv/bin/python,python3)
CSV_SERVICES ?= all
CSV_ROWS ?= 200
CSV_FULL ?=

.PHONY: help create_env generate-csv generate-csv-auth generate-csv-restaurant generate-csv-review load-auth load-restaurant load-reviews load-all setup build run bootstrap-search stop down restart logs test clean clean-volumes clean-all test test-bruno

help:
	@echo "MrFood Make Commands"
	@echo ""
	@echo "Setup & Data:"
	@echo "  make create_env                         - Create secret .env files from env.tmpl"
	@echo "  (config.env is already committed — no setup needed)"
	@echo "  make setup                              - Start services and load all data"
	@echo "  make generate-csv                       - Generate CSV seed data (default 200 rows)"
	@echo "  make generate-csv CSV_FULL=1            - Generate CSV seed data (full dataset)"
	@echo "  make load-reviews                       - Load review seed data into database"
	@echo "  make load-all                           - Load all seed data into databases"
	@echo ""
	@echo "Service Management:"
	@echo "  make run                                - Start all services (detached)"
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
	cd tests/mrfood-api/collections/ci && npx --yes @usebruno/cli@latest run -r --tests-only --reporter-junit ../../reports/bruno-junit.xml --reporter-json ../../reports/bruno-report.json

## Build services

# ============================================================================
# ENVIRONMENT
# ============================================================================

## Create secret .env files from services/env.tmpl
## config.env (non-sensitive) is already committed — no action needed for it.
create_env:
	@if [ -f services/.env ]; then \
		echo "services/.env already exists."; \
	else \
		cp services/env.tmpl services/.env; \
		echo "Created services/.env"; \
	fi
	@for f in services/*/env.tmpl; do \
		dir=$$(dirname "$$f"); \
		if [ -f "$$dir/.env" ]; then \
			echo "$$dir/.env already exists."; \
		else \
			cp "$$f" "$$dir/.env"; \
			echo "Created $$dir/.env"; \
		fi; \
	done
	@echo "Fill in secret values in all created .env files before running docker compose."

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

## Complete setup: start services and load all data
setup: run load-csvs
	@echo "✓ Setup complete! Services running and data loaded"

# ============================================================================
# SERVICE MANAGEMENT
# ============================================================================

## Build service images
build:
	$(DC) build

build-no-cache:
	$(DC) build --no-cache

## Start all services (detached)
run:
	$(DC) up -d --pull=missing
	@$(MAKE) --no-print-directory search-bootstrap

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

SEARCH_COMPOSE_FILE := services/docker-compose.cdc.yml
SEARCH_SERVICES := search elasticsearch zookeeper kafka connect restaurant restaurant_db kong auth auth_db otel-collector


DCS := docker compose -p $(PROJECT_NAME) \
	-f services/docker-compose.yml \
	-f $(SEARCH_COMPOSE_FILE) \
	$(ENV_FILES)

## Start only search-related services (ES, Kafka, Connect)
search-run:
	$(DCS) up -d $(SEARCH_SERVICES)
	@$(MAKE) --no-print-directory search-bootstrap

## Bootstrap ES index and register CDC connectors
search-bootstrap:
	@echo "Waiting for Elasticsearch..."
	@until curl -fsS http://localhost:$(CDC_ELASTIC_PORT) >/dev/null 2>&1; do sleep 2; done
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
	$(DCS) stop $(SEARCH_SERVICES)

## Stop and remove search services
search-down:
	$(DCS) rm -sf $(SEARCH_SERVICES)

## Tail logs for search services only
search-logs:
	$(DCS) logs -f $(SEARCH_SERVICES)

## Full reset of search (removes elastic_data and connect_plugins volumes)
search-clean:
	$(DCS) rm -sf $(SEARCH_SERVICES)
	docker volume rm -f $(PROJECT_NAME)_elastic_data $(PROJECT_NAME)_connect_plugins