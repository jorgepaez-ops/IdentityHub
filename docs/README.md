# Índice de la entrega final

Este índice conecta los requisitos de `ENUNCIADO-TRABAJO-FINAL.md` con los artefactos realmente versionados en Identity Hub. Úselo para revisar primero qué ya existe, qué sirve como antecedente y qué queda planificado para la semana 4.

## Estructura del enunciado -> estructura del repositorio

| Estructura propuesta por el enunciado | Ubicación real | Justificación |
|---|---|---|
| `LICENSE` | [`../LICENSE`](../LICENSE) | Licencia Apache-2.0 versionada en la raíz. |
| `README.md` | [`../README.md`](../README.md) | Presenta el propósito, tecnologías, seguridad, inicio rápido y trazabilidad. |
| `docker-compose.yml` | [`../deploy/docker-compose.yml`](../deploy/docker-compose.yml) | El compose actual vive en `deploy/`; T6 creará la entrada portátil desde la raíz. |
| `.github/workflows/` | [`../.github/workflows/`](../.github/workflows/) y [`../e2e/`](../e2e/) | Los workflows automatizan CI/CD y ejecutan la suite E2E versionada. |
| `infraestructura/` | No existe todavía | T8 creará los módulos de Terraform para la arquitectura de referencia. |
| `orquestacion/` | [`../deploy/`](../deploy/) y [`../deploy/docker-compose.yml`](../deploy/docker-compose.yml) | `deploy/` concentra Docker Compose y observabilidad; T9 definirá la variante de producción. |
| `servicios/` | [`../backend/`](../backend/), [`../frontend/`](../frontend/), [`../contabilidad/`](../contabilidad/) y [`../e2e/`](../e2e/) | Los servicios se separan por responsabilidad; `e2e/` contiene sus pruebas de extremo a extremo. |
| `docs/` | [`docs/`](./), [`../specs/`](../specs/), [`../security/`](../security/) y [`../scripts/`](../scripts/) | `docs/` reúne la entrega; las especificaciones, evidencia de seguridad y trazabilidad versionada la respaldan. |

## Entregables y requisitos del enunciado

La columna **Ubicación actual** solo enlaza rutas que existen en este checkout. `Parcial` indica que hay evidencia o una base reutilizable, pero no satisface todavía el requisito completo.

| Requisito | Línea(s) | Ubicación actual | Estado |
|---|---:|---|---|
| README principal: propósito, tecnologías, licencia, badges e inicio rápido | 112–118 | [`../README.md`](../README.md) | hecho |
| Manual de arquitectura: microservicios, decisiones y patrones | 119–121 | [`../specs/00-vision.md`](../specs/00-vision.md) y [`../specs/adr/`](../specs/adr/) | parcial (T4) |
| Diagrama de componentes | 123–131 | [`diagramas/01-arquitectura.html`](diagramas/01-arquitectura.html) | parcial (T2) |
| Diagrama de despliegue | 123–131 | No existe todavía (la topología actual está en [`../deploy/docker-compose.yml`](../deploy/docker-compose.yml)) | pendiente (T2) |
| Diagrama de secuencia de un flujo crítico | 123–131 | [`diagramas/02-flujo-sso.html`](diagramas/02-flujo-sso.html) | parcial (T2) |
| Diagrama de casos de uso | 123–131 | No existe todavía | pendiente (T2) |
| DFD nivel 0 y nivel 1 con OWASP Threat Dragon | 123–131 | [`../specs/05-security/threat-model.md`](../specs/05-security/threat-model.md) | parcial (T3) |
| Manual de desarrollo: entorno, servicios, pruebas y contribución | 132–139 | [`guia-desarrollo.md`](guia-desarrollo.md) | parcial (T4) |
| Manual de despliegue y operación | 140–149 | [`../deploy/docker-compose.yml`](../deploy/docker-compose.yml) | parcial (T4) |
| Manual de seguridad: amenazas, herramientas, reportes y vulnerabilidades | 151–156 | [`../specs/05-security/threat-model.md`](../specs/05-security/threat-model.md), [`security-report.html`](security-report.html) y [`../security/findings/`](../security/findings/) | parcial (T4) |
| Manual de usuario con capturas | 157–159 | No existe todavía | pendiente (T18) |
| Fase 1: Threat Dragon, DFD y STRIDE | 75–79 | [`../specs/05-security/threat-model.md`](../specs/05-security/threat-model.md) | parcial (T3) |
| Fase 2: hooks, SAST y SCA | 80–84 | [`../.pre-commit-config.yaml`](../.pre-commit-config.yaml) y [`../.github/workflows/ci.yml`](../.github/workflows/ci.yml) | hecho |
| Fase 3: build de imágenes, escaneo y gate de CVE | 85–89 | [`../.github/workflows/ci.yml`](../.github/workflows/ci.yml), [`../backend/Dockerfile`](../backend/Dockerfile) y [`../frontend/Dockerfile`](../frontend/Dockerfile) | hecho |
| Fase 4: pruebas unitarias y DAST | 90–93 | [`../.github/workflows/ci.yml`](../.github/workflows/ci.yml), [`../e2e/`](../e2e/) y [`../security/zap-reports/`](../security/zap-reports/) | hecho |
| Fase 5: IaC, Checkov/tfsec y orquestación simulada | 94–98 | [`../deploy/docker-compose.yml`](../deploy/docker-compose.yml) | parcial (T8, T9 y T10) |
| Fase 6: métricas, logs y detección en ejecución | 99–103 | [`../deploy/docker-compose.yml`](../deploy/docker-compose.yml) y [`../deploy/observability/`](../deploy/observability/) | parcial: Prometheus, Grafana y Loki (vía Grafana Alloy) hechos; Falco fuera salvo que sobre tiempo |
| Repositorio GitHub público con código, pipeline, IaC y documentación | 163–166 | [`../README.md`](../README.md) y [`../.github/workflows/`](../.github/workflows/) | parcial (T21) |
| Imágenes publicadas y versionadas en Docker Hub | 163–167 | [`../specs/adr/0011-docker-hub-como-registry.md`](../specs/adr/0011-docker-hub-como-registry.md) | pendiente (T17) |
| Informe técnico en PDF | 163–168 | No existe todavía | pendiente (T19) |
| Video-demostración de 10–15 minutos | 163–169 | [`guion-demo.md`](guion-demo.md) | parcial (T20) |
| PDF: portada | 174 | No existe todavía | pendiente (T19) |
| PDF: introducción | 175 | No existe todavía | pendiente (T19) |
| PDF: arquitectura y diagramas UML | 176 | No existe todavía | pendiente (T19) |
| PDF: modelado de amenazas | 177 | No existe todavía | pendiente (T19) |
| PDF: implementación del pipeline | 178 | No existe todavía | pendiente (T19) |
| PDF: resultados de seguridad | 179 | No existe todavía | pendiente (T19) |
| PDF: monitoreo y observabilidad, si aplica | 180 | No existe todavía | pendiente (T19) |
| PDF: conclusiones | 181 | No existe todavía | pendiente (T19) |

## Entregables propios de la semana 4

No los exige el enunciado, pero responden a la demo en vivo con el profesor:

| Entregable | Ubicación actual | Estado |
|---|---|---|
| Grilla de roles y permisos configurable desde el Hub | No existe todavía | pendiente (T11 a T15) |
| Guía de integración de plataformas de terceros | No existe todavía | pendiente (T5) |

## Cómo navegar

1. Empiece por el [`README principal`](../README.md) para ejecutar y ubicar el proyecto.
2. Consulte [`../specs/`](../specs/) para requisitos, contratos, ADR y trazabilidad.
3. Use la [`guía de desarrollo`](guia-desarrollo.md) para el entorno local, generación y pruebas.
4. Revise el [`informe de seguridad`](security-report.html) junto con las evidencias de seguridad.
5. Lea la [`bitácora`](BITACORA.md) para avances, decisiones y estado histórico.
6. Prepare la demostración con el [`guion`](guion-demo.md).
7. Siga el avance de la entrega en [`../odd/tasks/`](../odd/tasks/).

## Próximos artefactos

T2 incorpora los diagramas UML versionables, T3 el modelo de Threat Dragon, T4 completa los manuales, T8 añade IaC y T19 produce el informe PDF. Este índice debe actualizarse cuando cada artefacto quede verificado.
