# Identity Hub — atajos del proyecto.
#
# Regla de diseño: todo lo que corre en CI tiene que poder correrse aquí con el
# mismo nombre. Si `make scan` y el pipeline divergen, el pipeline deja de ser
# una red de seguridad y pasa a ser una sorpresa.

COMPOSE     := docker compose --env-file .env -f deploy/docker-compose.yml
COMPOSE_OBS := $(COMPOSE) --profile observability
SQLC_VERSION := v1.31.1
SQLC         ?= sqlc

.DEFAULT_GOAL := help
.PHONY: help up down logs ps restart build test test-go test-integration test-front e2e lint fmt gen scan scan-secrets scan-deps scan-image scan-config migrate psql rabbit mail clean

help: ## Muestra esta ayuda
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

# ── Entorno ──────────────────────────────────────────────────────────────
up: ## Levanta el stack de desarrollo
	$(COMPOSE) up -d --build
	@echo ""
	@echo "  Aplicación      http://localhost:8080"
	@echo "  API             http://localhost:8081/healthz"
	@echo "  RabbitMQ        http://localhost:15672   (credentials from .env)"
	@echo "  Mailpit         http://localhost:8025"
	@echo ""
	@echo "  Observabilidad: make up-obs  →  Grafana en http://localhost:3000"

up-obs: ## Levanta el stack incluida la observabilidad
	$(COMPOSE_OBS) up -d --build
	@echo "  Grafana         http://localhost:3000     (credentials from .env)"
	@echo "  Prometheus      http://localhost:9090"

down: ## Detiene el stack conservando los volúmenes
	$(COMPOSE_OBS) down

clean: ## Detiene el stack y BORRA los volúmenes de datos
	$(COMPOSE_OBS) down -v

ps: ## Estado de los contenedores
	$(COMPOSE) ps

logs: ## Sigue los logs (make logs S=api)
	$(COMPOSE) logs -f $(S)

restart: ## Reconstruye y reinicia un servicio (make restart S=api)
	$(COMPOSE) up -d --build $(S)

build: ## Construye las tres imágenes
	$(COMPOSE) build

# ── Desarrollo ───────────────────────────────────────────────────────────
gen: ## Regenera todo lo derivado de los specs (RNF-011)
	cd backend && go generate ./internal/api
	@test "$$($(SQLC) version)" = "$(SQLC_VERSION)" || \
		(echo "sqlc $(SQLC_VERSION) is required" && exit 1)
	$(SQLC) generate
	cd frontend && npm run gen:api
	python3 scripts/traceability.py

fmt: ## Formatea el código
	cd backend && gofmt -w .

lint: ## Lint y comprobación de tipos
	cd backend && go vet ./...
	@if command -v golangci-lint >/dev/null 2>&1; then cd backend && golangci-lint run --timeout=5m; else echo "  (golangci-lint no instalado: go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2)"; fi
	cd frontend && npm run lint

test: test-go test-front ## Todas las pruebas

test-go: ## Pruebas de Go con detector de carreras
	cd backend && go test -race -coverprofile=coverage.out -covermode=atomic ./...
	cd backend && go tool cover -func=coverage.out | tail -1

test-integration: ## Pruebas de integración con PostgreSQL temporal
	cd backend && go test -race -tags=integration ./...

test-front: ## Pruebas del frontend
	cd frontend && npm run test

e2e: ## Pruebas de extremo a extremo contra el stack levantado
	@echo "Pendiente para la semana 3: Playwright."

migrate: ## Aplica las migraciones pendientes
	$(COMPOSE) run --rm migrate

psql: ## Consola de PostgreSQL
	$(COMPOSE) exec db psql -U identity -d identity

rabbit: ## Abre la interfaz de RabbitMQ
	open http://localhost:15672 || xdg-open http://localhost:15672

mail: ## Abre Mailpit
	open http://localhost:8025 || xdg-open http://localhost:8025

# ── Seguridad: los mismos gates que en CI ────────────────────────────────
scan: scan-secrets scan-deps scan-config scan-image ## Todos los gates de seguridad en local

scan-secrets: ## Gitleaks sobre el historial completo (RNF-003)
	@echo "── Gitleaks ──────────────────────────────────────────────"
	docker run --rm -v "$(PWD):/repo" zricethezav/gitleaks:v8.18.4 \
		detect --source=/repo --verbose || true

scan-deps: ## govulncheck y npm audit (RNF-004)
	@echo "── govulncheck ───────────────────────────────────────────"
	cd backend && go run golang.org/x/vuln/cmd/govulncheck@latest ./... || true
	@echo "── npm audit ─────────────────────────────────────────────"
	cd frontend && npm audit --audit-level=high || true

scan-config: ## Trivy config y Hadolint (RNF-008)
	@echo "── Trivy config ──────────────────────────────────────────"
	docker run --rm -v "$(PWD):/src" aquasec/trivy:0.56.2 \
		config --severity HIGH,CRITICAL /src || true
	@echo "── Hadolint ──────────────────────────────────────────────"
	@for f in backend/Dockerfile frontend/Dockerfile; do \
		echo "  $$f"; \
		docker run --rm -i hadolint/hadolint:v2.12.0 hadolint - < $$f || true; \
	done

scan-image: build ## Trivy sobre las imágenes construidas (RNF-004)
	@echo "── Trivy sobre las imágenes ──────────────────────────────"
	@for img in api worker web; do \
		echo "  identity-hub-$$img"; \
		docker run --rm -v /var/run/docker.sock:/var/run/docker.sock \
			aquasec/trivy:0.56.2 image --severity HIGH,CRITICAL \
			--ignore-unfixed identity-hub-$$img:latest 2>&1 | tail -15 || true; \
	done
