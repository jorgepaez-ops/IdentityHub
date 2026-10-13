# Índice de la entrega final

Este índice conecta los requisitos de `ENUNCIADO-TRABAJO-FINAL.md` con los artefactos realmente versionados en Identity Hub. Úselo para revisar qué está cerrado y qué queda pendiente para el cierre en vivo de la semana 4.

## Estructura del enunciado -> estructura del repositorio

| Estructura propuesta por el enunciado | Ubicación real | Justificación |
|---|---|---|
| `LICENSE` | [`../LICENSE`](../LICENSE) | Licencia Apache-2.0 versionada en la raíz. |
| `README.md` | [`../README.md`](../README.md) | Presenta el propósito, tecnologías, seguridad, inicio rápido y trazabilidad. |
| `docker-compose.yml` | [`../docker-compose.yml`](../docker-compose.yml) y [`../deploy/docker-compose.yml`](../deploy/docker-compose.yml) | El compose de raíz incluye el compose real de `deploy/`; `make setup` prepara el entorno local sin versionar secretos. |
| `.github/workflows/` | [`../.github/workflows/`](../.github/workflows/) y [`../e2e/`](../e2e/) | Los workflows automatizan CI/CD y ejecutan la suite E2E versionada. |
| `infraestructura/` | [`../infraestructura/`](../infraestructura/) | Módulos Terraform para la arquitectura AWS de referencia; no requieren una nube de pago para levantar el proyecto local. |
| `orquestacion/` | [`../deploy/`](../deploy/), [`../deploy/docker-compose.yml`](../deploy/docker-compose.yml) y [`../deploy/docker-compose.prod.yml`](../deploy/docker-compose.prod.yml) | `deploy/` concentra Docker Compose y observabilidad; el compose de producción simulada está terminado. |
| `servicios/` | [`../backend/`](../backend/), [`../frontend/`](../frontend/), [`../contabilidad/`](../contabilidad/) y [`../e2e/`](../e2e/) | Los servicios se separan por responsabilidad; `e2e/` contiene sus pruebas de extremo a extremo. |
| `docs/` | [`docs/`](./), [`../specs/`](../specs/), [`../security/`](../security/) y [`../scripts/`](../scripts/) | `docs/` reúne la entrega; las especificaciones, evidencia de seguridad y trazabilidad versionada la respaldan. |

## Entregables y requisitos del enunciado

La columna **Ubicación actual** solo enlaza rutas que existen en este checkout. `Parcial` indica que hay evidencia o una base reutilizable, pero no satisface todavía el requisito completo.

| Requisito | Línea(s) | Ubicación actual | Estado |
|---|---:|---|---|
| README principal: propósito, tecnologías, licencia, badges e inicio rápido | 112–118 | [`../README.md`](../README.md) | hecho |
| Manual de arquitectura: microservicios, decisiones y patrones | 119–121 | [`manuales/arquitectura.md`](manuales/arquitectura.md) | hecho (T4) |
| Diagrama de componentes | 123–131 | [`diagramas/uml/componentes.md`](diagramas/uml/componentes.md) y [`diagramas/01-arquitectura.html`](diagramas/01-arquitectura.html) como apoyo | hecho |
| Diagrama de despliegue | 123–131 | [`diagramas/uml/despliegue.md`](diagramas/uml/despliegue.md) (local), [`diagramas/uml/despliegue-aws.md`](diagramas/uml/despliegue-aws.md) (referencia en AWS) y [`../deploy/docker-compose.yml`](../deploy/docker-compose.yml) | hecho |
| Diagrama de secuencia de un flujo crítico | 123–131 | [`diagramas/uml/secuencia-login-mfa.md`](diagramas/uml/secuencia-login-mfa.md), [`diagramas/uml/secuencia-sso-pkce.md`](diagramas/uml/secuencia-sso-pkce.md) y [`diagramas/02-flujo-sso.html`](diagramas/02-flujo-sso.html) como apoyo | hecho |
| Diagrama de estados del ciclo de vida de la cuenta | 123–131 | [`diagramas/uml/estados-usuario.md`](diagramas/uml/estados-usuario.md) y [`diagramas/03-ciclo-de-vida-usuario.html`](diagramas/03-ciclo-de-vida-usuario.html) como apoyo | hecho |
| Diagrama de casos de uso | 123–131 | [`diagramas/uml/casos-de-uso.md`](diagramas/uml/casos-de-uso.md) | hecho |
| DFD nivel 0 y nivel 1 con OWASP Threat Dragon | 123–131 | [`threat-model/`](threat-model/) | hecho |
| Manual de desarrollo: entorno, servicios, pruebas y contribución | 132–139 | [`guia-desarrollo.md`](guia-desarrollo.md) | hecho (T4) |
| Manual de despliegue y operación | 140–149 | [`manuales/despliegue-y-operacion.md`](manuales/despliegue-y-operacion.md) | hecho (T4, T7–T9) |
| Manual de seguridad: amenazas, herramientas, reportes y vulnerabilidades | 151–156 | [`manuales/seguridad.md`](manuales/seguridad.md) | hecho (T4) |
| Manual de usuario con capturas | 157–159 | [`manuales/usuario.md`](manuales/usuario.md) y sus 48 capturas en [`manuales/img/usuario/`](manuales/img/usuario/), regenerables con `make capturas` | hecho (T18) |
| Fase 1: Threat Dragon, DFD y STRIDE | 75–79 | [`threat-model/`](threat-model/) | hecho |
| Fase 2: hooks, SAST y SCA | 80–84 | [`../.pre-commit-config.yaml`](../.pre-commit-config.yaml) y [`../.github/workflows/ci.yml`](../.github/workflows/ci.yml) | hecho |
| Fase 3: build de imágenes, escaneo y gate de CVE | 85–89 | [`../.github/workflows/ci.yml`](../.github/workflows/ci.yml), [`../backend/Dockerfile`](../backend/Dockerfile) y [`../frontend/Dockerfile`](../frontend/Dockerfile) | hecho |
| Fase 4: pruebas unitarias y DAST | 90–93 | [`../.github/workflows/ci.yml`](../.github/workflows/ci.yml), [`../e2e/`](../e2e/) y [`../security/zap-reports/`](../security/zap-reports/) | hecho |
| Fase 5: IaC, Checkov/tfsec y orquestación simulada | 94–98 | [`../infraestructura/`](../infraestructura/), [`../deploy/docker-compose.prod.yml`](../deploy/docker-compose.prod.yml) y [`../.github/workflows/ci.yml`](../.github/workflows/ci.yml) | hecho (T8, T9 y T10) |
| Fase 6: métricas, logs y detección en ejecución | 99–103 | [`../deploy/docker-compose.yml`](../deploy/docker-compose.yml) y [`../deploy/observability/`](../deploy/observability/) | hecho: Prometheus, Grafana y Loki (vía Grafana Alloy) y Falco (cuatro reglas propias, alerta en Grafana y [runbook](runbooks/falco-alerta.md)) |
| Repositorio GitHub público con código, pipeline, IaC y documentación | 163–166 | [`../README.md`](../README.md), [`../.github/workflows/`](../.github/workflows/) e [`../infraestructura/`](../infraestructura/) | hecho (T21c; el PR de la fase 4 está fusionado) |
| Imágenes publicadas y versionadas en Docker Hub | 163–167 | [`../specs/adr/0011-docker-hub-como-registry.md`](../specs/adr/0011-docker-hub-como-registry.md) | hecho (C3): `docker.io/jorgeivp/identity-hub-api`, `-worker` y `-web`, firmadas con Cosign y con SBOM adjunto; validado con el tag `v0.9.0` |
| Informe técnico en PDF | 163–168 | PDF generado con `make informe` (no se versiona por tamaño; se adjunta al release de GitHub en T21), fuente en [`informe/informe-tecnico.md`](informe/informe-tecnico.md); `make informe` lo regenera | hecho (T19) |
| Video-demostración de 10–15 minutos | 163–169 | [`video/guion.md`](video/guion.md) | guion completo de 15 minutos (T20), con escenas de exportación de auditoría, claves de servicio y Falco; grabación a cargo del equipo |
| PDF: portada | 174 | [`informe/informe-tecnico.md`](informe/informe-tecnico.md) | hecho (T19) |
| PDF: introducción | 175 | [`informe/informe-tecnico.md`](informe/informe-tecnico.md) | hecho (T19) |
| PDF: arquitectura y diagramas UML | 176 | [`informe/informe-tecnico.md`](informe/informe-tecnico.md) | hecho (T19) |
| PDF: modelado de amenazas | 177 | [`informe/informe-tecnico.md`](informe/informe-tecnico.md) | hecho (T19) |
| PDF: implementación del pipeline | 178 | [`informe/informe-tecnico.md`](informe/informe-tecnico.md) | hecho (T19) |
| PDF: resultados de seguridad | 179 | [`informe/informe-tecnico.md`](informe/informe-tecnico.md) | hecho (T19) |
| PDF: monitoreo y observabilidad, si aplica | 180 | [`informe/informe-tecnico.md`](informe/informe-tecnico.md) | hecho (T19) |
| PDF: conclusiones | 181 | [`informe/informe-tecnico.md`](informe/informe-tecnico.md) | hecho (T19) |

## Entregables propios de la semana 4

No los exige el enunciado, pero responden a la demo en vivo con el profesor:

| Entregable | Ubicación actual | Estado |
|---|---|---|
| Grilla de roles y permisos configurable desde el Hub | [`../specs/adr/0013-roles-y-permisos-configurables-por-aplicacion.md`](../specs/adr/0013-roles-y-permisos-configurables-por-aplicacion.md) y [`../specs/06-acceptance/roles-y-permisos.feature`](../specs/06-acceptance/roles-y-permisos.feature) | hecho (T11 a T15; RF-021) |
| Guía de integración de plataformas de terceros | [`manuales/integracion-terceros.md`](manuales/integracion-terceros.md) | completa; la integración OAuth/OIDC de terceros y el outbox quedan como trabajo futuro (sección 9.1) |
| Claves de servicio para integraciones máquina (RF-018) | [`manuales/claves-de-servicio.md`](manuales/claves-de-servicio.md) y [`../specs/adr/0014-claves-de-servicio-para-integraciones-maquina.md`](../specs/adr/0014-claves-de-servicio-para-integraciones-maquina.md) | hecho (F4) |
| Exportación del registro de auditoría en CSV y JSON (RF-019) | [`manuales/usuario.md`](manuales/usuario.md) y [`../specs/06-acceptance/`](../specs/06-acceptance/) | hecho (F1) |
| Runbook de alertas de Falco | [`runbooks/falco-alerta.md`](runbooks/falco-alerta.md) | hecho (F3) |

## Cómo navegar

1. Empiece por el [`README principal`](../README.md) para ejecutar y ubicar el proyecto.
2. Consulte [`../specs/`](../specs/) para requisitos, contratos, ADR y trazabilidad.
3. Use la [`guía de desarrollo`](guia-desarrollo.md) para el entorno local, generación, pruebas y contribución.
4. Consulte los manuales de [`arquitectura`](manuales/arquitectura.md), [`despliegue y operación`](manuales/despliegue-y-operacion.md) y [`seguridad`](manuales/seguridad.md).
5. Revise el [`informe de seguridad`](security-report.html) junto con las evidencias de seguridad.
6. Lea la [`bitácora`](BITACORA.md) para avances, decisiones y estado histórico.
7. Prepare la demo en vivo de la sustentación con el [`guion de la demo`](guion-demo.md) (unos 5 minutos, más 4 opcionales) y el video con el [`guion del video`](video/guion.md) (15 minutos; la grabación sigue pendiente).
8. Siga el avance de la entrega en [`../odd/tasks/`](../odd/tasks/).

## Próximos artefactos

La documentación, los diagramas, la IaC de referencia, la grilla de roles, el manual de usuario, el informe técnico y el guion del video están verificados. Las imágenes ya están publicadas en Docker Hub (C3). Las funciones de cierre F1 a F4 y F6 a F10 están integradas en `main` y `v0.9.1` validó el release firmado; F5 queda como trabajo futuro. Quedan el tag `v1.0.0` y el release final, y la grabación del video, que hace el equipo.
