# ── Project ───────────────────────────────────────────────────────────────────
PROJECT_NAME  := mrfood
COMPOSE_FILE  := services/docker-compose.yml
TEST_PACKAGES := \
    ./services/auth/...         \
    ./services/booking/...      \
    ./services/notification/... \
    ./services/payment/...      \
    ./services/restaurant/...   \
    ./services/review/...       \
    ./services/sponsor/...

# ── Environment (committed config, git-ignored secrets, GCP infra vars) ───────
-include gcp.env
-include services/config.env
-include services/.env

ENV_FILES := --env-file services/config.env
ENV_FILES += $(if $(wildcard services/.env),--env-file services/.env,)

# ── Tools ─────────────────────────────────────────────────────────────────────
DC     := docker compose -p $(PROJECT_NAME) -f $(COMPOSE_FILE) $(ENV_FILES)
PYTHON := $(if $(wildcard scripts/.venv/bin/python),scripts/.venv/bin/python,python3)

IS_PODMAN  := $(shell docker --version 2>/dev/null | grep -i podman)
PULL_FLAG  :=
BUILD_FLAG :=
ifeq ($(IS_PODMAN),)
    PULL_FLAG  := --pull=missing
    BUILD_FLAG := --parallel
endif

# ── Defaults ──────────────────────────────────────────────────────────────────
CSV_SERVICES ?= all
CSV_ROWS     ?= 200
CSV_FULL     ?=
LOAD_ARGS    ?=

GATEWAY_IP ?=
BASE_URL   ?= http://localhost:8000
VUS        ?= 20
DURATION   ?= 2m

STRESS_DIR         := tests/stress
BRUNO_URL_OVERRIDE := $(if $(GATEWAY_IP),--env-var "baseUrl=http://$(GATEWAY_IP)",)
K6_ENV             := -e BASE_URL=$(BASE_URL) \
                      -e STRESS_EMAIL=$(STRESS_EMAIL) \
                      -e STRESS_PASSWORD=$(STRESS_PASSWORD)

# ── Phony targets ─────────────────────────────────────────────────────────────
.PHONY: help \
    deploy seed destroy undeploy \
    create-env generate-csv load-local load-cloud \
    build setup setup-full setup-observability \
    stop down down-volumes restart logs \
    test test-bruno \
    clean clean-all \
    search-bootstrap search-seed search-logs search-clean \
    stress stress-smoke \
    stress-auth stress-restaurants stress-reviews stress-reservations \
    stress-search stress-payments stress-sponsor

# ============================================================================
# HELP
# ============================================================================

help:
	@echo "MrFood — available commands"
	@echo ""
	@echo "Cloud"
	@echo "  deploy                  Deploy infrastructure + Kubernetes workloads to GCP"
	@echo "  destroy                 Destroy all GCP infrastructure"
	@echo "  undeploy                Uninstall Helm releases and delete GKE namespace"
	@echo "  seed                    Re-seed Cloud SQL (truncate + reimport CSV data)"
	@echo ""
	@echo "Local"
	@echo "  create-env              Create services/.env from env.tmpl"
	@echo "  setup                   Start core services"
	@echo "  setup-full              Start all services including search + CDC"
	@echo "  setup-observability     Start observability stack only"
	@echo "  build                   Build service images"
	@echo "  stop                    Stop services (keep containers)"
	@echo "  down                    Stop and remove containers"
	@echo "  down-volumes            Stop and remove containers and volumes"
	@echo "  restart                 Restart all services"
	@echo "  logs                    Tail all service logs"
	@echo ""
	@echo "Data"
	@echo "  generate-csv            Generate CSV seed data  [CSV_ROWS=200] [CSV_FULL=1]"
	@echo "  load-local              Load CSV into local Docker containers  [LOAD_ARGS=--dry-run]"
	@echo "  load-cloud              Load CSV into Cloud SQL via GCS  [LOAD_ARGS=--dry-run]"
	@echo ""
	@echo "Testing"
	@echo "  test                    Run Go unit tests"
	@echo "  test-bruno              Run Bruno API tests  [GATEWAY_IP=x.x.x.x for cloud]"
	@echo ""
	@echo "Search"
	@echo "  search-bootstrap        Register CDC connectors and seed Elasticsearch"
	@echo "  search-seed             Seed Elasticsearch only (no connectors)"
	@echo "  search-logs             Tail search service logs"
	@echo "  search-clean            Remove search containers and volumes"
	@echo ""
	@echo "Cleanup"
	@echo "  clean                   Remove containers, local images, volumes"
	@echo "  clean-all               Remove containers, all images, volumes"
	@echo ""
	@echo "Stress  [BASE_URL=...] [VUS=20] [DURATION=2m]"
	@echo "  stress                  Full concurrent stress test"
	@echo "  stress-smoke            Quick smoke test (1 VU, 1 iteration)"
	@echo "  stress-auth             Auth endpoints"
	@echo "  stress-restaurants      Restaurant endpoints"
	@echo "  stress-reviews          Review endpoints"
	@echo "  stress-reservations     Reservation endpoints"
	@echo "  stress-search           Search endpoint"
	@echo "  stress-payments         Payment endpoints"
	@echo "  stress-sponsor          Sponsor endpoints"

# ============================================================================
# CLOUD DEPLOYMENT
# ============================================================================

deploy:
	@bash scripts/deploy.sh

seed:
	@bash scripts/seed.sh $(LOAD_ARGS)

destroy:
	@bash scripts/destroy.sh

undeploy:
	@bash scripts/undeploy.sh

# ============================================================================
# ENVIRONMENT
# ============================================================================

create-env:
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
	$(PYTHON) scripts/process_data.py \
		--services $(CSV_SERVICES) \
		$(if $(CSV_ROWS),--rows $(CSV_ROWS),) \
		$(if $(CSV_FULL),--full,)

load-local:
	@bash scripts/load_seed_data_local.sh $(LOAD_ARGS)

load-cloud:
	@bash scripts/load_seed_data_cloud.sh $(LOAD_ARGS)

# ============================================================================
# SERVICE MANAGEMENT
# ============================================================================

build:
	DOCKER_BUILDKIT=1 $(DC) build $(BUILD_FLAG)

setup:
	$(DC) up -d $(PULL_FLAG)
	@echo "✓ Core services running"

setup-full: setup search-bootstrap
	@echo "✓ All services running with search"

setup-observability:
	$(DC) up -d $(PULL_FLAG) otel-collector prometheus loki grafana tempo

stop:
	$(DC) stop

down:
	$(DC) down

down-volumes:
	$(DC) down --volumes

restart: down setup

logs:
	$(DC) logs -f

# ============================================================================
# TESTING
# ============================================================================

test:
	go test -v -race $(TEST_PACKAGES) 2>&1 | tee /tmp/test_output.txt; \
	echo ""; \
	echo "━━━ Results ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"; \
	echo "PASS: $$(grep -c '^--- PASS' /tmp/test_output.txt)"; \
	echo "FAIL: $$(grep -c '^--- FAIL' /tmp/test_output.txt)"; \
	grep -q '^--- FAIL' /tmp/test_output.txt && exit 1 || exit 0

test-bruno:
	@mkdir -p tests/mrfood-api/reports
	@rc=0; \
	(cd tests/mrfood-api/collections/users && npx --yes @usebruno/cli@latest run -r --env development --tests-only $(BRUNO_URL_OVERRIDE) --reporter-junit ../../reports/users-junit.xml --reporter-json ../../reports/users-report.json) || rc=1; \
	(cd tests/mrfood-api/collections/restaurants && npx --yes @usebruno/cli@latest run -r --env development --tests-only $(BRUNO_URL_OVERRIDE) --reporter-junit ../../reports/restaurants-junit.xml --reporter-json ../../reports/restaurants-report.json) || rc=1; \
	(cd tests/mrfood-api/collections/reservations && npx --yes @usebruno/cli@latest run -r --env development --tests-only $(BRUNO_URL_OVERRIDE) --reporter-junit ../../reports/reservations-junit.xml --reporter-json ../../reports/reservations-report.json) || rc=1; \
	(cd tests/mrfood-api/collections/reviews && npx --yes @usebruno/cli@latest run -r --env development --tests-only $(BRUNO_URL_OVERRIDE) --reporter-junit ../../reports/reviews-junit.xml --reporter-json ../../reports/reviews-report.json) || rc=1; \
	$(if $(GATEWAY_IP),,bash services/cdc/seed_elasticsearch.sh;) \
	(cd tests/mrfood-api/collections/search && npx --yes @usebruno/cli@latest run -r --env development --tests-only $(BRUNO_URL_OVERRIDE) --reporter-junit ../../reports/search-junit.xml --reporter-json ../../reports/search-report.json) || rc=1; \
	(cd tests/mrfood-api/collections/payment && npx --yes @usebruno/cli@latest run -r --env development --tests-only $(BRUNO_URL_OVERRIDE) --reporter-junit ../../reports/payment-junit.xml --reporter-json ../../reports/payment-report.json) || rc=1; \
	(cd tests/mrfood-api/collections/sponsor && npx --yes @usebruno/cli@latest run -r --env development --tests-only $(BRUNO_URL_OVERRIDE) --reporter-junit ../../reports/sponsor-junit.xml --reporter-json ../../reports/sponsor-report.json) || rc=1; \
	exit $$rc

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
	@curl -fsS "http://localhost:$(ELASTICSEARCH_PORT)/_cluster/health?wait_for_status=yellow&timeout=60s" > /dev/null
	@echo "✔ Elasticsearch ready"
	@bash services/cdc/register-connectors.sh
	@bash services/cdc/seed_elasticsearch.sh

search-seed:
	@curl -fsS "http://localhost:$(ELASTICSEARCH_PORT)/_cluster/health?wait_for_status=yellow&timeout=120s" > /dev/null
	@echo "✔ Elasticsearch ready"
	@bash services/cdc/seed_elasticsearch.sh

search-logs:
	$(DC) --profile search logs -f elasticsearch zookeeper kafka connect search

search-clean:
	$(DC) --profile search rm -sf elasticsearch zookeeper kafka connect search
	docker volume rm -f $(PROJECT_NAME)_elastic_data

# ============================================================================
# STRESS TESTS (k6)
# ============================================================================

stress-smoke:
	k6 run $(K6_ENV) $(STRESS_DIR)/smoke.js

stress:
	k6 run $(K6_ENV) -e VUS=$(VUS) -e DURATION=$(DURATION) $(STRESS_DIR)/full.js

stress-auth:
	k6 run $(K6_ENV) $(STRESS_DIR)/scenarios/auth.js

stress-restaurants:
	k6 run $(K6_ENV) $(STRESS_DIR)/scenarios/restaurants.js

stress-reviews:
	k6 run $(K6_ENV) $(STRESS_DIR)/scenarios/reviews.js

stress-reservations:
	k6 run $(K6_ENV) $(STRESS_DIR)/scenarios/reservations.js

stress-search:
	k6 run $(K6_ENV) $(STRESS_DIR)/scenarios/search.js

stress-payments:
	k6 run $(K6_ENV) $(STRESS_DIR)/scenarios/payments.js

stress-sponsor:
	k6 run $(K6_ENV) $(STRESS_DIR)/scenarios/sponsor.js
