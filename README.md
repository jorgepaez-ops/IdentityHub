# Identity Hub

## 1. Descripción

Identity Hub es un proveedor de identidad desarrollado desde cero, comparable en alcance didáctico a un Entra ID pequeño. Es el proyecto final universitario y usa una aplicación real para demostrar un pipeline DevSecOps de ciclo completo.

## 2. Insignias

[![CI](https://github.com/jorgepaez-ops/IdentityHub/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/jorgepaez-ops/IdentityHub/actions/workflows/ci.yml)
[![Versión](https://img.shields.io/github/v/tag/jorgepaez-ops/IdentityHub?label=versi%C3%B3n)](https://github.com/jorgepaez-ops/IdentityHub/tags)
[![Licencia](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

<!-- El badge de cobertura queda pendiente de un servicio que publique el resultado. -->

## 3. Estado del proyecto

- **Semana 2 cerrada:** 47 de 47 tareas completadas; el tag
  `v0.1.0-hardened` identifica el estado remediado en `main`, con CI verde.
- **Semana 3 en curso:** SSO entre dominios, MFA por correo, alta de empleados y frontend funcional. El alcance y el progreso están en
  [`odd/tasks/idp-semana-3.md`](odd/tasks/idp-semana-3.md).
- **Semana 4:** entrega final, incluida la portabilidad del arranque y los artefactos de publicación.

## 4. Tecnologías

| Área | Componentes verificados |
|---|---|
| Backend y procesamiento asíncrono | Go 1.26 para API y worker |
| Frontend | React, TypeScript, Vite y Nginx |
| Datos y mensajería | PostgreSQL 16, RabbitMQ 4 y Mailpit |
| Observabilidad | Prometheus, Loki, Grafana Alloy y Grafana |
| Seguridad en CI | Gitleaks, CodeQL, Semgrep, gosec mediante golangci-lint, govulncheck, npm audit, osv-scanner, Hadolint y Trivy |

El backend está escrito en Go por decisión del proyecto: el profesor permitió elegir el stack siempre que se cumplieran los requisitos de DevSecOps y de gestor de identidad. Las alternativas mostradas en el material del curso eran sugerencias, no una restricción.

La cobertura de integración tiene un gate mínimo de 70 % en el job **`6b · Pruebas de integración`** del workflow de CI. No existe todavía un servicio que publique esa cobertura como badge.

## 5. Arquitectura

La visión y los contratos del sistema están en [`specs/00-vision.md`](specs/00-vision.md). Para recorrer la solución visualmente, consulte los diagramas de [arquitectura](docs/diagramas/01-arquitectura.html), [flujo SSO](docs/diagramas/02-flujo-sso.html) y [ciclo de vida de usuario](docs/diagramas/03-ciclo-de-vida-usuario.html). Las decisiones de arquitectura (ADR 0001 a 0011) están en [`specs/adr/`](specs/adr/), con su formato explicado en [`specs/adr/README.md`](specs/adr/README.md).

## 6. Inicio rápido

1. Clone el repositorio y entre en su directorio.
2. Copie [`.env.example`](.env.example) como `.env` y complete los valores requeridos localmente. El archivo `.env` no se versiona.
3. Ejecute `make up`.

El objetivo usa `docker compose --env-file .env -f deploy/docker-compose.yml` y construye el stack. Al terminar informa estas direcciones:

| Servicio | Dirección |
|---|---|
| Aplicación web (Hub) | <http://identityhub.localhost:8080> (<http://localhost:8080> también sirve el Hub) |
| Contabilidad (marcador, T13) | <http://contabilidad.localhost:8080> |
| API | <http://localhost:8081/healthz> |
| RabbitMQ | <http://localhost:15672> |
| Mailpit | <http://localhost:8025> |

Use `make up-obs` para incluir Grafana en <http://localhost:3000> y Prometheus en <http://localhost:9090>. [`Makefile`](Makefile) y `make help` enumeran los demás comandos disponibles.

Actualmente el compose vive en `deploy/` y el entorno necesita un `.env` local. Un `docker-compose.yml` en la raíz y un arranque sin preparación adicional están planificados para la semana 4 como parte del requisito de portabilidad.

## 7. Seguridad y evidencia DevSecOps

- [Informe de seguridad](docs/security-report.html): la versión publicada se conserva como artefacto privado de claude.ai; el enlace no se incluye en el repositorio.
- [Fichas de vulnerabilidades](security/findings/): 29 fichas, de VULN-001 a VULN-029; 27 remediadas, VULN-019 con remediación parcial y VULN-028 como riesgo aceptado hasta 2026-12-25.
- [Evidencia por vulnerabilidad](docs/evidencia/): archivos `evidencia.json` por hallazgo; la [evidencia cruda de escáneres](security/evidence/) se conserva por ejecución.
- [Modelo de amenazas](specs/05-security/threat-model.md): riesgos y mitigaciones por componente.
- [ADR 0007](specs/adr/0007-linea-base-vulnerable-deliberada.md): explica la línea base vulnerable deliberada. El tag `v0.0.0-vuln-baseline` nunca debe desplegarse; `v0.1.0-hardened` representa el estado remediado.

El workflow de CI valida la deriva entre especificaciones y código, lint y tipos, secretos, análisis estático, dependencias, pruebas, cobertura de integración, configuración de contenedores e imágenes. Los escaneos de línea base y semanales complementan el workflow principal para conservar evidencia histórica y detectar avisos posteriores.

## 8. Gestión del proyecto y trazabilidad

- [Bitácora](docs/BITACORA.md): decisiones, avances y próximos pasos.
- [Tareas de la semana 2](odd/tasks/idp-semana-2.md): cerradas; [tareas de la semana 3](odd/tasks/idp-semana-3.md): en curso.
- [Matriz de trazabilidad](specs/07-traceability.md): artefacto generado por [`scripts/traceability.py`](scripts/traceability.py).
- [Especificaciones](specs/): requisitos, modelo de dominio, contrato [OpenAPI](specs/03-api/openapi.yaml), eventos y escenarios de aceptación.
- [Guía de desarrollo](docs/guia-desarrollo.md): ejecución, generación y pruebas para desarrollo local.

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
