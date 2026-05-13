# Config
PROJECT_NAME := mrfood
COMPOSE_FILE := services/docker-compose.yml
TEST_PACKAGES := ./services/auth/... ./services/booking/... ./services/restaurant/... ./services/review/... ./services/sponsor/... ./services/observability/...

# Load non-sensitive config (committed) and secrets (git-ignored)
-include services/config.env
-include services/.env

ENV_FILES := --env-file services/config.env
ENV_FILES += $(if $(wildcard services/.env),--env-file services/.env,)

DC := docker compose -p $(PROJECT_NAME) -f $(COMPOSE_FILE) $(ENV_FILES)
PYTHON := $(if $(wildcard scripts/.venv/bin/python),scripts/.venv/bin/python,python3)
CSV_SERVICES ?= all
CSV_ROWS ?= 200
CSV_FULL ?=

IS_PODMAN := $(shell docker --version 2>/dev/null | grep -i podman)
PULL_FLAG :=
BUILD_FLAG :=

ifeq ($(IS_PODMAN),)
	PULL_FLAG := --pull=missing
	BUILD_FLAG := --parallel
endif

.PHONY: help create_env generate-csv setup setup-full build run run-full stop down restart logs test test-bruno clean clean-all search-bootstrap search-logs search-clean

help:
	@echo "MrFood — available commands"
	@echo ""
	@echo "  make create_env      Create services/.env from env.tmpl"
	@echo "  make generate-csv    Generate CSV seed data (CSV_ROWS=200, CSV_FULL=1)"
	@echo "  make setup           Start core services"
	@echo "  make setup-full      Start all services including search/CDC"
	@echo "  make run             Start core services (detached)"
	@echo "  make run-full        Start all services including search/CDC (detached)"
	@echo "  make stop            Stop services"
	@echo "  make down            Stop and remove containers"
	@echo "  make restart         Restart services"
	@echo "  make logs            Tail logs"
	@echo "  make build           Build service images"
	@echo "  make test            Run Go tests"
	@echo "  make test-bruno      Run Bruno API tests"
	@echo "  make clean           Remove containers, images, volumes"
	@echo "  make clean-all       Full reset (all images included)"
	@echo "  make search-bootstrap  Create ES index and register CDC connectors"
	@echo "  make search-logs     Tail search service logs"
	@echo "  make search-clean    Remove search containers and volumes"

# ============================================================================
# ENVIRONMENT
# ============================================================================

create_env:
	@if [ -f services/.env ]; then \
		echo "services/.env already exists."; \
	else \
		cp services/env.tmpl services/.env; \
		echo "Created services/.env — fill in secret values before running."; \
	fi

# ============================================================================
# DATA GENERATION
# ============================================================================

generate-csv:
	$(PYTHON) scripts/process_data.py --services $(CSV_SERVICES) $(if $(CSV_ROWS),--rows $(CSV_ROWS),) $(if $(CSV_FULL),--full,)

# ============================================================================
# SERVICE MANAGEMENT
# ============================================================================

build:
	DOCKER_BUILDKIT=1 $(DC) build $(BUILD_FLAG)

run:
	$(DC) up -d $(PULL_FLAG)

run-full:
	$(DC) --profile search up -d $(PULL_FLAG)

setup: run
	@echo "✓ Core services running"

setup-full: run-full search-bootstrap
	@echo "✓ All services running with search"

stop:
	$(DC) stop

down:
	$(DC) down

restart: down run

logs:
	$(DC) logs -f

# ============================================================================
# TESTING
# ============================================================================

test:
	go test -v -race $(TEST_PACKAGES)

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

# ============================================================================
# CLEANUP
# ============================================================================

clean:
	$(DC) down --rmi local --volumes --remove-orphans

clean-all:
	$(DC) down --rmi all --volumes --remove-orphans

# ============================================================================
# SEARCH
# ============================================================================

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
		echo "✔ Index created"; \
	fi
	@bash services/cdc/register-connectors.sh
	@bash services/cdc/seed_elasticsearch.sh

search-logs:
	$(DC) --profile search logs -f elasticsearch zookeeper kafka connect search

search-clean:
	$(DC) --profile search rm -sf elasticsearch zookeeper kafka connect search
	docker volume rm -f $(PROJECT_NAME)_elastic_data
