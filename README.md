# Identity Hub

## 1. Descripción

Identity Hub es un proveedor de identidad desarrollado desde cero, comparable en alcance didáctico a un Entra ID pequeño. Es el proyecto final universitario y usa una aplicación real para demostrar un pipeline DevSecOps de ciclo completo.

## 2. Insignias

[![CI](https://github.com/jorgepaez-ops/IdentityHub/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/jorgepaez-ops/IdentityHub/actions/workflows/ci.yml)
[![Versión](https://img.shields.io/github/v/tag/jorgepaez-ops/IdentityHub?label=versi%C3%B3n)](https://github.com/jorgepaez-ops/IdentityHub/tags)
[![Licencia](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![Cobertura de integración](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/jorgepaez-ops/IdentityHub/badges/coverage.json)](https://github.com/jorgepaez-ops/IdentityHub/actions/workflows/ci.yml)

## 3. Estado del proyecto

- **Semana 2 cerrada:** 47 de 47 tareas completadas; el tag
  `v0.1.0-hardened` identifica el estado remediado en `main`, con CI verde.
- **Semana 3 cerrada:** SSO entre dominios, MFA por correo, alta de empleados y frontend funcional.
- **Semana 4:** documentación, IaC de referencia, grilla de roles, manual, informe y guion terminados. Las imágenes están publicadas en Docker Hub (`docker.io/jorgeivp/identity-hub-api`, `-worker` y `-web`, firmadas con Cosign y con SBOM adjunto). El seguimiento está en [`odd/tasks/idp-semana-4.md`](odd/tasks/idp-semana-4.md).
- **Funciones de cierre (F1 a F10, PR 32 a 43), integradas en `main`:** exportación del registro de auditoría en CSV y JSON (RF-019); SMTP con STARTTLS y autenticación obligatorios en el worker y hosts configurables (`SMTP_*`, `OAUTH_*`); Falco en el perfil de observabilidad con cuatro reglas propias, alerta en Grafana y [runbook](docs/runbooks/falco-alerta.md); claves de servicio para integraciones máquina (RF-018, [manual](docs/manuales/claves-de-servicio.md), ADR 0014); columna de controles de CI en la matriz; insignia de cobertura medida; y un filtro de CI para cambios solo de documentación. La integración OAuth/OIDC de terceros y el outbox (F5) quedan como trabajo futuro ([sección 9.1](docs/manuales/integracion-terceros.md)).
- **Release firmado:** `latest` solo se mueve después de firmar y atestar, y únicamente para el `vX.Y.Z` más alto; los pasos de red se reintentan y los artefactos se retienen 7 días. El tag `v0.9.1` (ejecución 38405885275) lo validó y se verificó con Cosign v3.1.3 sobre `v0.9.0` y `v0.9.1`.
- **Matriz de trazabilidad:** 33 requisitos; 25 completos, 7 parciales, 1 sin cubrir (RNF-010, latencia) y 0 diferidos.
- **Pendiente:** el tag de entrega `v1.0.0` y el release de GitHub con el informe en PDF, y la grabación del video, que hace el equipo.

## 4. Tecnologías

| Área | Componentes verificados |
|---|---|
| Backend y procesamiento asíncrono | Go 1.26 para API y worker |
| Frontend | React, TypeScript, Vite y Nginx |
| Datos y mensajería | PostgreSQL 16, RabbitMQ 4 y Mailpit |
| Observabilidad | Prometheus, Loki, Grafana Alloy, Grafana y Falco (detección en tiempo de ejecución) |
| Seguridad en CI | Gitleaks, CodeQL, Semgrep, gosec mediante golangci-lint, govulncheck, npm audit, osv-scanner, Hadolint y Trivy |

El backend está escrito en Go por decisión del proyecto: el profesor permitió elegir el stack siempre que se cumplieran los requisitos de DevSecOps y de gestor de identidad. Las alternativas mostradas en el material del curso eran sugerencias, no una restricción.

La cobertura de integración tiene un gate mínimo de 70 % en el job **`6b · Pruebas de integración`** del workflow de CI. El badge de cobertura muestra el valor medido en la última ejecución de CI sobre `main` (el job `6c` lo publica como `coverage.json` en la rama `badges`, sin servicio externo); el gate sigue siendo 70 %.

## 5. Arquitectura

La visión y los contratos del sistema están en [`specs/00-vision.md`](specs/00-vision.md). Para recorrer la solución visualmente, consulte los diagramas de [arquitectura](docs/diagramas/01-arquitectura.html), [flujo SSO](docs/diagramas/02-flujo-sso.html) y [ciclo de vida de usuario](docs/diagramas/03-ciclo-de-vida-usuario.html). Las decisiones de arquitectura (ADR 0001 a 0014) están en [`specs/adr/`](specs/adr/), con su formato explicado en [`specs/adr/README.md`](specs/adr/README.md).

## 6. Inicio rápido

1. Clone el repositorio y entre en su directorio.
2. Ejecute `make setup`: genera un `.env` local con secretos aleatorios a partir de [`.env.example`](.env.example). El archivo `.env` no se versiona y un `.env` existente no se sobrescribe.
3. Ejecute `make up`: genera los certificados SMTP locales (`make certs`, idempotente) y levanta el stack construyendo las imágenes. Si prefiere `docker compose up -d`, corra antes `make certs`: Mailpit exige STARTTLS y autenticación, y sin `deploy/certs/` el stack no arranca. Un `.env` generado antes de F2 necesita además `SMTP_PASSWORD` (ver «Si su `.env` es anterior a F2»).

```bash
git clone <url-del-repositorio> && cd <directorio>
make setup
make up        # o: make certs && docker compose --profile observability up -d
```

El `docker-compose.yml` de la raíz incluye `deploy/docker-compose.yml`; `make up` usa `docker compose --env-file .env -f deploy/docker-compose.yml`. Al terminar se exponen estas direcciones:

| Servicio | Dirección |
|---|---|
| Aplicación web (Hub) | <http://identityhub.localhost:8080> (<http://localhost:8080> también sirve el Hub) |
| Contabilidad (marcador, T13) | <http://contabilidad.localhost:8080> |
| API | <http://localhost:8081/healthz> |
| RabbitMQ | <http://localhost:15672> |
| Mailpit | <http://localhost:8025> |
| Grafana | <http://localhost:3000> |
| Prometheus | <http://localhost:9090> |

`make up` incluye Grafana (<http://localhost:3000>), Prometheus (<http://localhost:9090>), Loki, Alloy y Falco (`make falco-test` dispara eventos inofensivos que activan sus reglas; ver el [runbook](docs/runbooks/falco-alerta.md)). `make up-app` levanta solo la aplicación, sin observabilidad (lo usa CI). [`Makefile`](Makefile) y `make help` enumeran los demás comandos disponibles.

### Primer arranque

1. `make setup`: crea `.env` desde `.env.example` con secretos aleatorios, incluida `SMTP_PASSWORD`. Si `.env` ya existe, no lo modifica.
2. `make up`: ejecuta `make certs` por su cuenta (no hace falta correrlo a mano), construye las imágenes y levanta el stack completo, con Grafana, Prometheus, Loki, Alloy y Falco. `make up-app` levanta solo la aplicación.
3. Abra <http://identityhub.localhost:8080>.

### Levantar el stack

`make up` levanta todo (aplicación y observabilidad) y `make up-app` solo la aplicación (lo usa CI); `make down` detiene conservando volúmenes y `make clean` los borra. Servicios y puertos publicados, tiempos de espera, Falco y el kernel, y los problemas frecuentes están en la guía: [Levantar el stack](docs/guia-desarrollo.md#levantar-el-stack).

### Si su `.env` es anterior a F2

`make setup` no toca un `.env` existente, así que nunca le agrega `SMTP_PASSWORD`, y `make certs` y `make up` fallan con «falta SMTP_PASSWORD». Agregue solo esa línea:

```bash
grep -q '^SMTP_PASSWORD=' .env || printf '\nSMTP_PASSWORD=%s\n' "$(openssl rand -hex 24)" >> .env
make up
```

No use `python3 scripts/setup_env.py --force` sobre un entorno existente: regenera todos los secretos, pero PostgreSQL y RabbitMQ conservan las contraseñas antiguas en sus volúmenes y el stack deja de autenticarse (la salida sería `make clean`, que borra los datos).

El compose real vive en `deploy/` y el de la raíz lo incluye, de modo que un clon nuevo arranca con `make setup && make up`. Los secretos se generan localmente y nunca se versionan.

## 7. Seguridad y evidencia DevSecOps

- [Informe de seguridad](docs/security-report.html): versión final de la evidencia versionada, incluida la remediación de VULN-019.
- [Fichas de vulnerabilidades](security/findings/): 31 fichas, de VULN-001 a VULN-031 (VULN-001 a VULN-019 sembradas en la línea base); 30 remediadas y VULN-028 como no afectado (GO-2026-5932, código vulnerable no presente).
- [Evidencia por vulnerabilidad](docs/evidencia/): archivos `evidencia.json` por hallazgo; la [evidencia cruda de escáneres](security/evidence/) se conserva por ejecución.
- [Modelo de amenazas](specs/05-security/threat-model.md): riesgos y mitigaciones por componente.
- [ADR 0007](specs/adr/0007-linea-base-vulnerable-deliberada.md): explica la línea base vulnerable deliberada. El tag `v0.0.0-vuln-baseline` nunca debe desplegarse; `v0.1.0-hardened` representa el estado remediado.

El workflow de CI valida la deriva entre especificaciones y código, lint y tipos, secretos, análisis estático, dependencias, pruebas, cobertura de integración, configuración de contenedores e imágenes. Los escaneos de línea base y semanales complementan el workflow principal para conservar evidencia histórica y detectar avisos posteriores.

### Alertas abiertas conocidas

Estado de las alertas de code scanning en `main` una vez integradas las correcciones de P9b a P9d (2026-10-06). Lo demás (acciones sin fijar, diagramas, nginx, imágenes base de `web`, Checkov, AWS-0089) está corregido o justificado en el propio código.

| Herramienta | Regla | Ubicación | Motivo | Justificación o control compensatorio |
|---|---|---|---|---|
| Trivy | GO-2026-5932 | `golang.org/x/crypto` v0.56.0 en la imagen `api` | El aviso cubre el paquete `openpgp` y sus subpaquetes, que no tiene versión corregida; el módulo sigue haciendo falta para Argon2id (ADR 0004) | VULN-028, no afectado (`backend/osv-scanner.toml`, exclusión permanente): ningún paquete propio ni dependencia importa `openpgp` (`go list -deps`); osv-scanner solo lo marca por coincidencia a nivel de módulo y govulncheck no lo reporta |
| Trivy | DLA-4792-1 | `tzdata` 2026b en las imágenes `api` y `worker` (base `distroless/static-debian12`) | Debian publicó 2026c, pero la imagen distroless fijada todavía no lo incluye; severidad sin clasificar (UNKNOWN) | Solo son datos de zonas horarias. Se actualizará el digest de la base cuando upstream la reconstruya (el escaneo semanal lo detecta) |

Falsos positivos de secretos en SonarCloud (2026-10-07): `security/evidence/**` se excluye del análisis con `.sonarcloud.properties` porque son salidas almacenadas de los escáneres, conservadas como evidencia "antes" (VULN-001), no código; y el ejemplo literal del comentario de `.gitleaks.toml` (regla `secrets:S2068`) se reescribió sin un valor con forma de credencial. Justificación: los valores son sembrados, nunca reales ni reutilizados (confirmado por el responsable el 2026-10-06).

Las 27 alertas de Checkov eran omisiones ya justificadas en línea (`#checkov:skip=ID:motivo`): CI deja de subirlas a code scanning (se filtran del SARIF) pero la justificación sigue en el código y el gate sigue activo. La de Trivy AWS-0089 (bucket de logs del ALB sin logging propio) lleva `#trivy:ignore:AWS-0089` por el mismo motivo que `CKV_AWS_18`.

## 8. Gestión del proyecto y trazabilidad

- [Bitácora](docs/BITACORA.md): decisiones, avances y próximos pasos.
- [Tareas de la semana 2](odd/tasks/idp-semana-2.md): cerradas; [tareas de la semana 3](odd/tasks/idp-semana-3.md): cerradas; [tareas de la semana 4](odd/tasks/idp-semana-4.md) y [cierre de la entrega](odd/tasks/cierre-entrega.md): cierre en curso.
- [Matriz de trazabilidad](specs/07-traceability.md): artefacto generado por [`scripts/traceability.py`](scripts/traceability.py).
- [Especificaciones](specs/): requisitos, modelo de dominio, contrato [OpenAPI](specs/03-api/openapi.yaml), eventos y escenarios de aceptación.
- [Guía de desarrollo](docs/guia-desarrollo.md): ejecución, generación y pruebas para desarrollo local.
- [Manual de usuario](docs/manuales/usuario.md): flujos de empleado, administrador y Contabilidad con capturas.
- [Claves de servicio](docs/manuales/claves-de-servicio.md): emisión, uso y revocación de claves para integraciones máquina (RF-018).
- [Informe técnico](docs/informe/informe-tecnico.md): fuente del PDF final; ejecute `make informe` para regenerarlo.
- [Guion del video](docs/video/guion.md): demostración de 15 minutos; la grabación sigue pendiente.

Para una revisión técnica ordenada, empiece por la [visión](specs/00-vision.md), continúe con los [ADRs](specs/adr/), revise el [modelo de amenazas](specs/05-security/threat-model.md) y cierre con la [matriz de trazabilidad](specs/07-traceability.md). Esta ruta permite contrastar intención, diseño, controles y evidencia.

## 9. Estructura del repositorio

```text
.github/     workflows de automatización y CI
backend/     API y worker en Go
frontend/    aplicación React y configuración de Nginx
db/          migraciones de PostgreSQL
deploy/      compose de desarrollo y observabilidad
docs/        bitácora, diagramas, evidencia e informe
odd/         tareas y seguimiento de la feature
scripts/     utilidades, incluida la trazabilidad
security/    fichas y evidencia de seguridad
specs/       requisitos, contratos, ADR y aceptación
```

## 10. Licencia

Este proyecto se distribuye bajo la licencia [Apache-2.0](LICENSE).
