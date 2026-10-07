# Identity Hub — atajos del proyecto.
#
# Regla de diseño: todo lo que corre en CI tiene que poder correrse aquí con el
# mismo nombre. Si `make scan` y el pipeline divergen, el pipeline deja de ser
# una red de seguridad y pasa a ser una sorpresa.

COMPOSE     := docker compose --env-file .env -f deploy/docker-compose.yml
COMPOSE_OBS := $(COMPOSE) --profile observability
COMPOSE_PROD := docker compose --env-file .env -f deploy/docker-compose.yml -f deploy/docker-compose.prod.yml
SQLC_VERSION := v1.31.1
SQLC         ?= sqlc

.DEFAULT_GOAL := help
.PHONY: help setup up down informe logs ps restart build test test-go test-integration test-front e2e _e2e-run capturas _capturas-run scan-dast _scan-dast-run spec-drift lint fmt gen scan scan-secrets scan-deps scan-image scan-config scan-iac migrate psql rabbit mail clean up-prod down-prod

help: ## Muestra esta ayuda
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

# ── Entorno ──────────────────────────────────────────────────────────────
setup: ## Genera .env con secretos aleatorios (no sobrescribe uno existente)
	python3 scripts/setup_env.py

up: ## Levanta el stack de desarrollo
	$(COMPOSE) up -d --build
	@echo ""
	@echo "  Hub             http://identityhub.localhost:8080  (localhost:8080 también sirve el Hub)"
	@echo "  Contabilidad    http://contabilidad.localhost:8080"
	@echo "  API             http://localhost:8081/healthz"
	@echo "  RabbitMQ        http://localhost:15672   (credentials from .env)"
	@echo "  Mailpit         http://localhost:8025"
	@echo ""
	@echo "  Observabilidad: make up-obs  →  Grafana en http://localhost:3000"

up-prod: ## Levanta la producción simulada (imágenes por digest, sin build ni puertos de desarrollo)
	$(COMPOSE_PROD) up -d --no-build
	@echo "  Hub             http://identityhub.localhost:8080  (único puerto publicado)"

down-prod: ## Detiene la producción simulada conservando los volúmenes
	$(COMPOSE_PROD) down

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
	cd contabilidad && npm run lint && npm run typecheck

test: test-go test-front ## Todas las pruebas

test-go: ## Pruebas de Go con detector de carreras
	cd backend && go test -race -coverprofile=coverage.out -covermode=atomic ./...
	cd backend && go tool cover -func=coverage.out | tail -1

test-integration: ## Pruebas de integración con PostgreSQL temporal (imprime la cobertura del gate de CI)
	cd backend && go test -race -tags=integration -coverprofile=coverage.out -covermode=atomic -coverpkg=./internal/... ./...
	cd backend && go tool cover -func=coverage.out | tail -1

test-front: ## Pruebas del frontend (consola del Hub y Contabilidad)
	cd frontend && npm run test
	cd contabilidad && npm run test

# Límite por IP mientras corren las suites locales (e2e, scan-dast). E2E_LOGIN_IP_MAX_FAILURES
# sigue funcionando como alias retrocompatible.
LOCAL_TEST_LOGIN_IP_MAX_FAILURES ?= $(or $(E2E_LOGIN_IP_MAX_FAILURES),1000)

# Envuelve un objetivo interno: sube solo el límite por IP (RF-017), lo restaura con trap en
# EXIT/INT/TERM y avisa si alguna IP quedó bloqueada. Ver scripts/with-raised-login-limit.sh.
WITH_RAISED_LIMIT = COMPOSE='$(COMPOSE)' LOCAL_TEST_LOGIN_IP_MAX_FAILURES=$(LOCAL_TEST_LOGIN_IP_MAX_FAILURES) \
    scripts/with-raised-login-limit.sh $(MAKE) --no-print-directory

e2e: ## Pruebas de extremo a extremo contra el stack levantado
	@# Todas las peticiones salen de la misma IP: se sube solo el límite por IP
	@# (RF-017) mientras corre la suite y se restaura al terminar, pase o falle.
	@$(WITH_RAISED_LIMIT) _e2e-run

_e2e-run:
	cd e2e && npm ci --ignore-scripts && npx playwright test

capturas: ## Regenera las capturas del manual de usuario (docs/manuales/img/usuario) contra el stack levantado
	@$(WITH_RAISED_LIMIT) _capturas-run

_capturas-run:
	cd e2e && npm ci --ignore-scripts && npm run capturas

ZAP_IMAGE ?= ghcr.io/zaproxy/zaproxy:2.17.0
ZAP_REPORTS := security/zap-reports
ZAP_RUN = docker run --rm --add-host identityhub.localhost:host-gateway \
    --add-host contabilidad.localhost:host-gateway \
    -v "$(CURDIR)/$(ZAP_REPORTS):/zap/wrk:rw" \
    -v "$(CURDIR)/.zap/rules.tsv:/zap/wrk/rules.tsv:ro" \
    -v "$(CURDIR)/specs/03-api/openapi.yaml:/zap/wrk/openapi.yaml:ro" $(ZAP_IMAGE)

scan-dast: ## DAST con OWASP ZAP (baseline Hub y Contabilidad + API) contra el stack levantado; rompe con riesgo medio o más
	@# ZAP corre con -I: no decide él. scripts/zap-gate.py es el único punto de decisión.
	@# El escaneo de API golpea /api/v1/auth/ desde una sola IP: se sube el límite por IP
	@# (RF-017) mientras corre y se restaura al terminar, igual que en make e2e.
	@# Al API scan NO se le pasa -c: con un archivo de reglas ZAP cambia a la política "Default Policy"
	@# con todas las reglas activas (DOM XSS lanza navegadores y agota la memoria del contenedor);
	@# sin él usa API-Minimal. Las reglas WARN/IGNORE las aplica scripts/zap-gate.py sobre el JSON.
	@# El escaneo de API va directo a la API (:8081), sin Nginx: su limit_req (5r/s en /api/v1/auth/)
	@# cortaba respuestas a mitad del escaneo (410 → 429) y ZAP lo leía como inyección SQL booleana
	@# (falso positivo 40018, run 37058826831). Nginx sigue cubierto por los dos baseline y por E2E.
	@rm -rf $(ZAP_REPORTS) && mkdir -p $(ZAP_REPORTS) && chmod 777 $(ZAP_REPORTS)
	@$(WITH_RAISED_LIMIT) _scan-dast-run

_scan-dast-run:
	$(ZAP_RUN) zap-baseline.py -t http://identityhub.localhost:8080 -c rules.tsv -I \
	    -J hub.json -r hub.html && \
	  $(ZAP_RUN) zap-baseline.py -t http://contabilidad.localhost:8080 -c rules.tsv -I \
	    -J contabilidad.json -r contabilidad.html && \
	  $(ZAP_RUN) zap-api-scan.py -t openapi.yaml -f openapi -O http://identityhub.localhost:8081 \
	    -I -J api.json -r api.html && \
	  python3 scripts/zap-gate.py --rules .zap/rules.tsv \
	    $(ZAP_REPORTS)/hub.json $(ZAP_REPORTS)/contabilidad.json $(ZAP_REPORTS)/api.json

spec-drift: ## Verifica sin red la matriz y escenarios Gherkin contra E2E
	python3 scripts/traceability_test.py
	python3 scripts/zap_gate_test.py
	python3 scripts/setup_env_test.py
	python3 scripts/traceability.py --check

informe: ## Genera el informe técnico en PDF (docs/informe/informe-tecnico.pdf) con Chromium de Playwright
	cd docs/informe && npm ci --ignore-scripts && node build.mjs

migrate: ## Aplica las migraciones pendientes
	$(COMPOSE) run --rm migrate

psql: ## Consola de PostgreSQL
	$(COMPOSE) exec db psql -U identity -d identity

rabbit: ## Abre la interfaz de RabbitMQ
	open http://localhost:15672 || xdg-open http://localhost:15672

mail: ## Abre Mailpit
	open http://localhost:8025 || xdg-open http://localhost:8025

# ── Seguridad: los mismos gates que en CI ────────────────────────────────
scan: scan-secrets scan-deps scan-config scan-iac scan-image ## Todos los gates de seguridad en local

scan-secrets: ## Gitleaks sobre el historial completo (RNF-003)
	@echo "── Gitleaks ──────────────────────────────────────────────"
	docker run --rm -v "$(PWD):/repo" ghcr.io/gitleaks/gitleaks:v8.24.3 \
		detect --source=/repo --verbose || true

# govulncheck se instala desde tools/go.mod, como en CI (sin @versión; go.sum fija la versión).
scan-deps: ## govulncheck y npm audit (RNF-004)
	@echo "── govulncheck ───────────────────────────────────────────"
	go -C tools install golang.org/x/vuln/cmd/govulncheck
	cd backend && "$$(go env GOPATH)/bin/govulncheck" ./... || true
	@echo "── npm audit ─────────────────────────────────────────────"
	cd frontend && npm audit --audit-level=high || true
	cd contabilidad && npm audit --audit-level=high || true

# Same Trivy as CI (trivy-action v0.36.0 defaults to v0.70.0), pinned by digest. baseline-scan.yml keeps
# 0.56.2 on purpose so the before/after VULN evidence stays comparable with its historical runs.
TRIVY_IMAGE := aquasec/trivy:0.70.0@sha256:be1190afcb28352bfddc4ddeb71470835d16462af68d310f9f4bca710961a41e

scan-config: ## Trivy config y Hadolint (RNF-008)
	@echo "── Trivy config ──────────────────────────────────────────"
	docker run --rm -v "$(PWD):/src" $(TRIVY_IMAGE) \
		config --severity HIGH,CRITICAL /src || true
	@echo "── Hadolint ──────────────────────────────────────────────"
	@for f in backend/Dockerfile frontend/Dockerfile; do \
		echo "  $$f"; \
		docker run --rm -i hadolint/hadolint:v2.12.0 hadolint - < $$f || true; \
	done

scan-iac: ## Checkov sobre Terraform, Dockerfiles y workflows
	@echo "── Checkov ───────────────────────────────────────────────"
	docker run --rm -v "$(PWD):/src" -w /src bridgecrew/checkov:3.3.23 \
		-d . --framework terraform dockerfile github_actions --compact --quiet || true

scan-image: build ## Trivy sobre las imágenes construidas (RNF-004)
	@echo "── Trivy sobre las imágenes ──────────────────────────────"
	@for img in api worker web; do \
		echo "  identity-hub-$$img"; \
		docker run --rm -v /var/run/docker.sock:/var/run/docker.sock \
			$(TRIVY_IMAGE) image --severity HIGH,CRITICAL \
			--ignore-unfixed identity-hub-$$img:latest 2>&1 | tail -15 || true; \
	done
