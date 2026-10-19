# Identity Hub

[![CI](https://github.com/jorgepaez-ops/IdentityHub/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/jorgepaez-ops/IdentityHub/actions/workflows/ci.yml)
[![Versión](https://img.shields.io/github/v/tag/jorgepaez-ops/IdentityHub?label=versi%C3%B3n)](https://github.com/jorgepaez-ops/IdentityHub/tags)
[![Licencia](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![Cobertura de integración](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/jorgepaez-ops/IdentityHub/badges/coverage.json)](https://github.com/jorgepaez-ops/IdentityHub/actions/workflows/ci.yml)

## Qué es

Identity Hub es un **proveedor de identidad** escrito desde cero: el lugar donde una persona demuestra quién es una sola vez (contraseña y un código enviado por correo) y desde donde las aplicaciones de la organización saben quién es y qué puede hacer. Incluye inicio de sesión único entre aplicaciones (SSO), roles y permisos configurables por aplicación, registro de auditoría y claves de servicio para integraciones entre sistemas. Viene con una aplicación de ejemplo, **Contabilidad**, que delega su inicio de sesión en el Hub.

Es el proyecto final de una especialización universitaria en ciberseguridad (énfasis DevSecOps). Además del producto, entrega un **pipeline DevSecOps completo**: análisis de código y dependencias, escaneo de imágenes, pruebas de seguridad dinámicas, análisis de infraestructura como código, imágenes firmadas en Docker Hub y observabilidad con detección en tiempo de ejecución.

**Una analogía.** Piense en la portería de un edificio de oficinas. Usted muestra su documento una sola vez en la portería (el Hub) y le dan una credencial. Las oficinas (las aplicaciones) no le piden el documento: miran la credencial y comprueban que la firmó la portería. En el Hub, esa credencial es un token firmado que dice quién es usted y qué permisos tiene en cada aplicación. Si no conoce un término, búsquelo en el [glosario](docs/glosario.md).

## Con qué está hecho

| Área | Componentes |
|---|---|
| Backend y procesamiento asíncrono | Go 1.26 para la API y el worker |
| Frontend | React, TypeScript, Vite y Nginx |
| Datos y mensajería | PostgreSQL 16, RabbitMQ 4 y Mailpit |
| Observabilidad | Prometheus, Loki, Grafana Alloy, Grafana y Falco (detección en tiempo de ejecución) |
| Seguridad en CI | Gitleaks, CodeQL, Semgrep, gosec mediante golangci-lint, govulncheck, npm audit, osv-scanner, Hadolint y Trivy |

El backend está escrito en Go por decisión del proyecto: el profesor permitió elegir el stack siempre que se cumplieran los requisitos de DevSecOps y de gestor de identidad.

Para ver cómo encajan las piezas, abra los diagramas de [arquitectura](docs/diagramas/01-arquitectura.html), [flujo SSO](docs/diagramas/02-flujo-sso.html) y [ciclo de vida del usuario](docs/diagramas/03-ciclo-de-vida-usuario.html), o lea el [manual de arquitectura](docs/manuales/arquitectura.md).

## Levantarlo

Necesita Docker con Compose y `make`. Desde un clon nuevo:

```bash
make setup     # crea .env con secretos aleatorios (no sobrescribe uno existente)
make up        # genera certificados SMTP locales, construye las imágenes y levanta todo
```

1. **Revise `BOOTSTRAP_ADMIN_EMAIL`** en el `.env` generado (por defecto `admin@example.test`). Es el primer administrador. No hay usuario ni contraseña por defecto.
2. **Abra Mailpit** (<http://localhost:8025>): es la bandeja de correo local. Acepte allí la invitación del administrador y elija su contraseña. El código de verificación (MFA) de cada inicio de sesión también llega a esta bandeja.
3. **Entre al Hub** en <http://identityhub.localhost:8080>.

| Servicio | Dirección |
|---|---|
| Hub (aplicación web) | <http://identityhub.localhost:8080> |
| Contabilidad (aplicación de ejemplo) | <http://contabilidad.localhost:8080> |
| API (estado de salud) | <http://localhost:8081/healthz> |
| Mailpit (correo local) | <http://localhost:8025> |
| RabbitMQ (cola de mensajes) | <http://localhost:15672> |
| Grafana (paneles y alertas) | <http://localhost:3000> |
| Prometheus (métricas) | <http://localhost:9090> |

`make up-app` levanta solo la aplicación, sin observabilidad; `make down` detiene conservando los datos y `make clean` los borra; `make help` lista todos los comandos. ¿Tiene un `.env` antiguo, o algo no arranca? Vea [Levantar el stack](docs/guia-desarrollo.md#levantar-el-stack) y el [manual de despliegue y operación](docs/manuales/despliegue-y-operacion.md).

## Estado actual

La versión `v1.0.0` (2026-10-13) está publicada con su release en GitHub y las imágenes firmadas en Docker Hub. El repositorio versiona el guion del video de demostración; la grabación la hace el equipo. El historial semana a semana y una foto del estado al cierre (incluidas las alertas de seguridad que seguían abiertas en ese momento) están en la [bitácora](docs/BITACORA.md); el estado vigente de cada alerta se ve en la pestaña *Security* de GitHub.

## Mapa del repositorio

| Carpeta o archivo | Qué hay | Cuándo mirarla |
|---|---|---|
| [`docs/`](docs/README.md) | Manuales, diagramas, informe técnico, guion de la demo y video, bitácora, evidencia y glosario | Siempre que busque un documento: empiece por su [índice](docs/README.md) |
| [`specs/`](specs/README.md) | Visión, requisitos, modelo de dominio, contratos (OpenAPI y eventos), modelo de amenazas, decisiones (ADR) y escenarios de aceptación | Para saber qué debe hacer el sistema y por qué se diseñó así |
| [`backend/`](backend/README.md) | API y worker en Go | Para leer o modificar la lógica del servidor |
| [`frontend/`](frontend/README.md) | Consola web del Hub (React) y su Nginx | Para leer o modificar lo que ve el usuario del Hub |
| [`contabilidad/`](contabilidad/README.md) | Aplicación de ejemplo que usa el Hub para iniciar sesión | Para ver cómo integra su aplicación un tercero |
| [`db/`](db/README.md) | Migraciones y consultas SQL de PostgreSQL | Para cambiar o entender el esquema de datos |
| [`deploy/`](deploy/README.md) | Docker Compose (desarrollo y producción simulada) y observabilidad (Prometheus, Grafana, Loki, Alloy, Falco) | Para levantar, operar u observar el sistema |
| [`infraestructura/`](infraestructura/README.md) | Terraform de la arquitectura de referencia en AWS | Para ver cómo se desplegaría en la nube (no hace falta para correr el proyecto) |
| [`e2e/`](e2e/README.md) | Pruebas de extremo a extremo con Playwright | Para ver cómo se comprueba el sistema como lo usaría una persona |
| [`tests/`](tests/README.md) | Prueba de carga de latencia (k6) | Para revisar el requisito de rendimiento |
| [`security/`](security/README.md) | Fichas de vulnerabilidades, evidencia de escáneres e informes de ZAP | Para auditar qué se encontró y cómo se remedió |
| [`scripts/`](scripts/README.md) | Utilidades de automatización y sus pruebas | Para entender qué hacen los comandos de `make` por dentro |
| [`tools/`](tools/README.md) | Versiones fijadas de las herramientas de desarrollo y CI | Al actualizar generadores o escáneres |
| [`.github/`](.github/README.md) | Workflows de CI/CD, escaneos y release, y Dependabot | Para ver el pipeline DevSecOps |
| [`odd/`](odd/tasks/) | Archivos de tareas con el seguimiento del trabajo por semana | Para ver cómo se planificó y avanzó el proyecto |
| [`Makefile`](Makefile) | Comandos del día a día (`make help` los enumera) | Para levantar, probar, escanear o regenerar código |
| [`docker-compose.yml`](docker-compose.yml) | Archivo de la raíz que incluye el compose real de `deploy/` | Rara vez: es solo la puerta de entrada de `docker compose` |
| [`.env.example`](.env.example) | Plantilla de variables de entorno que `make setup` completa | Para saber qué se puede configurar |
| [`AGENTS.md`](AGENTS.md) y [`CLAUDE.md`](CLAUDE.md) | Instrucciones para los asistentes de IA usados durante el desarrollo | Solo para conocer cómo se trabajó con ellos; no hacen falta para usar el proyecto |
| [`LICENSE`](LICENSE) | Licencia Apache-2.0 | Antes de reutilizar el código |

La raíz también guarda la configuración de herramientas: `.pre-commit-config.yaml`, `.gitleaks.toml` y `.gitleaksignore` (búsqueda de secretos), `.zap/` (reglas del escáner dinámico), `.sonarcloud.properties` y `sqlc.yaml` (generación del código de acceso a datos).

## Rutas de lectura por perfil

Cada ruta va en orden: lea solo hasta donde necesite.

### Profesor o evaluador

1. [Índice de la entrega](docs/README.md#anexo-cumplimiento-del-enunciado): cada requisito del enunciado y dónde se cumple.
2. [Informe técnico](docs/informe/informe-tecnico.md): la narración completa del proyecto y del pipeline.
3. [Informe de seguridad](docs/security-report.html): hallazgos, remediación y evidencia.
4. [Guion de la demo](docs/guion-demo.md): un recorrido de unos 5 minutos del sistema funcionando.
5. [Matriz de trazabilidad](specs/07-traceability.md): requisito, escenario, prueba y control de CI, uno por uno.

### Desarrollador

1. [Guía de desarrollo](docs/guia-desarrollo.md): entorno local, generación de código y pruebas.
2. [Manual de arquitectura](docs/manuales/arquitectura.md): componentes y decisiones.
3. [Visión](specs/00-vision.md) y [decisiones (ADR)](specs/adr/README.md): qué se quiso lograr y por qué se eligió cada camino.
4. [Contrato OpenAPI](specs/03-api/openapi.yaml): la fuente de verdad de la API.

### Operador o DevOps

1. [Manual de despliegue y operación](docs/manuales/despliegue-y-operacion.md): configuración, verificación y problemas frecuentes.
2. [`deploy/`](deploy/README.md): qué contiene cada compose y la observabilidad.
3. [Runbook de Falco](docs/runbooks/falco-alerta.md) y [runbook de la cola de notificaciones](docs/runbooks/dlq-notificaciones.md): qué hacer cuando suena cada alerta.
4. [`infraestructura/`](infraestructura/README.md): la referencia en AWS con Terraform.

### Seguridad

1. [Manual de seguridad](docs/manuales/seguridad.md): amenazas, herramientas del pipeline y reportes.
2. [Modelo de amenazas](specs/05-security/threat-model.md) y [DFD en Threat Dragon](docs/threat-model/README.md): riesgos y mitigaciones por componente.
3. [Fichas de vulnerabilidades](security/README.md): cada hallazgo con su remediación.
4. [Evidencia por vulnerabilidad](docs/evidencia/README.md): el «antes» y el «después» de cada hallazgo.

### Empresa que integra su aplicación

1. [Guía de integración de terceros](docs/manuales/integracion-terceros.md): cómo una aplicación externa delega su inicio de sesión en el Hub (OAuth 2.0 con PKCE), con Contabilidad como ejemplo completo.
2. [`contabilidad/`](contabilidad/README.md): el código de la aplicación de ejemplo.
3. [Claves de servicio](docs/manuales/claves-de-servicio.md): credenciales para integraciones entre sistemas, sin sesión humana.
4. [Manual de usuario](docs/manuales/usuario.md): cómo se administran aplicaciones, roles y permisos desde el Hub.

## Seguridad y evidencia

- [Informe de seguridad](docs/security-report.html) y [manual de seguridad](docs/manuales/seguridad.md): la versión final de la evidencia y cómo leerla.
- [Fichas de vulnerabilidades](security/findings/): VULN-001 a VULN-031. La línea base vulnerable es deliberada ([ADR 0007](specs/adr/0007-linea-base-vulnerable-deliberada.md)): el tag `v0.0.0-vuln-baseline` nunca debe desplegarse, y `v0.1.0-hardened` marca el estado remediado.
- [Modelo de amenazas](specs/05-security/threat-model.md): riesgos y mitigaciones por componente.
- El workflow de CI valida la deriva entre especificaciones y código, secretos, análisis estático, dependencias, pruebas, cobertura de integración (mínimo 70 %), configuración de contenedores e imágenes; los escaneos de línea base y semanales conservan la evidencia histórica. Detalle en [`.github/`](.github/README.md).

## Gestión y trazabilidad

- [Bitácora](docs/BITACORA.md): decisiones, avances y estado histórico, la entrada más reciente arriba.
- [Archivos de tareas](odd/tasks/): seguimiento semana a semana ([semana 2](odd/tasks/idp-semana-2.md), [semana 3](odd/tasks/idp-semana-3.md), [semana 4](odd/tasks/idp-semana-4.md), [cierre de la entrega](odd/tasks/cierre-entrega.md)).
- [Matriz de trazabilidad](specs/07-traceability.md): generada por [`scripts/traceability.py`](scripts/traceability.py).
- [Glosario](docs/glosario.md): los términos técnicos explicados en simple.

## Licencia

Este proyecto se distribuye bajo la licencia [Apache-2.0](LICENSE).
