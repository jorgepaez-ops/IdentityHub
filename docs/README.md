# Documentación de Identity Hub

Esta página es el índice de `docs/`. Está organizada por la pregunta que usted probablemente trae: elija la suya y siga los enlaces. Si busca un término técnico, use el [glosario](glosario.md); para volver a la portada, el [README principal](../README.md).

| Si se pregunta... | Vaya a |
|---|---|
| ¿Qué hace y para quién? | [Sección 1](#1-qué-hace-y-para-quién) |
| ¿Cómo funciona por dentro? | [Sección 2](#2-cómo-funciona-por-dentro) |
| ¿Cómo lo levanto y lo opero? | [Sección 3](#3-cómo-lo-levanto-y-lo-opero) |
| ¿Es seguro y cómo se comprobó? | [Sección 4](#4-es-seguro-y-cómo-se-comprobó) |
| ¿Cómo integro mi aplicación? | [Sección 5](#5-cómo-integro-mi-aplicación) |
| ¿Qué se decidió y por qué? | [Sección 6](#6-qué-se-decidió-y-por-qué-adr) |
| ¿Cómo se desarrolló y se siguió el trabajo? | [Sección 7](#7-cómo-se-desarrolló-y-se-siguió-el-trabajo) |
| ¿Dónde se cumple cada punto del enunciado? | [Anexo](#anexo-cumplimiento-del-enunciado) |

## 1. ¿Qué hace y para quién?

- [Visión](../specs/00-vision.md): el problema que resuelve, el alcance y lo que queda fuera.
- [Manual de usuario](manuales/usuario.md): cómo usan el Hub las personas empleadas y las administradoras, y la aplicación Contabilidad, con capturas.
- [Guion de la demo](guion-demo.md): un recorrido de unos 5 minutos (más 4 opcionales) del sistema funcionando.
- [Guion del video](video/guion.md): la demostración de 15 minutos del ciclo DevSecOps.
- [Informe técnico](informe/informe-tecnico.md): la narración completa del proyecto, fuente del PDF final.

## 2. ¿Cómo funciona por dentro?

- [Manual de arquitectura](manuales/arquitectura.md): componentes, decisiones y patrones.
- Diagramas navegables: [arquitectura](diagramas/01-arquitectura.html), [flujo SSO](diagramas/02-flujo-sso.html) y [ciclo de vida del usuario](diagramas/03-ciclo-de-vida-usuario.html).
- Diagramas UML: [componentes](diagramas/uml/componentes.md), [despliegue local](diagramas/uml/despliegue.md), [despliegue en AWS](diagramas/uml/despliegue-aws.md), [casos de uso](diagramas/uml/casos-de-uso.md), [estados de la cuenta](diagramas/uml/estados-usuario.md) y las secuencias de [login con MFA](diagramas/uml/secuencia-login-mfa.md) y [SSO con PKCE](diagramas/uml/secuencia-sso-pkce.md).
- [Modelo de dominio](../specs/02-domain-model.md), [contrato OpenAPI](../specs/03-api/openapi.yaml) y [eventos y colas](../specs/04-events/topology.md): los datos y los contratos del sistema.
- [Mapa del repositorio](../README.md#mapa-del-repositorio): qué hay en cada carpeta.

## 3. ¿Cómo lo levanto y lo opero?

- [README principal, «Levantarlo»](../README.md#levantarlo): el camino más corto desde un clon nuevo.
- [Manual de despliegue y operación](manuales/despliegue-y-operacion.md): configuración, verificación, operaciones comunes y problemas frecuentes.
- [Guía de desarrollo](guia-desarrollo.md): entorno local, generación de código, pruebas y contribución.
- [Runbook de Falco](runbooks/falco-alerta.md): qué hacer cuando Falco detecta actividad anómala en un contenedor.
- [Runbook de la cola de notificaciones](runbooks/dlq-notificaciones.md): qué hacer cuando la cola de mensajes fallidos tiene mensajes.
- [`deploy/`](../deploy/README.md) e [`infraestructura/`](../infraestructura/README.md): los compose, la observabilidad y la referencia en AWS.

## 4. ¿Es seguro y cómo se comprobó?

- [Manual de seguridad](manuales/seguridad.md): amenazas, herramientas del pipeline, reportes y vulnerabilidades.
- [Modelo de amenazas (STRIDE)](../specs/05-security/threat-model.md) y [DFD en Threat Dragon](threat-model/README.md): riesgos y mitigaciones por componente.
- [Informe de seguridad](security-report.html): la evidencia versionada, con la remediación de cada hallazgo.
- [Evidencia por vulnerabilidad](evidencia/README.md): el «antes» y el «después» de cada hallazgo.
- [Fichas de vulnerabilidades](../security/README.md): VULN-001 a VULN-031 y la evidencia cruda de los escáneres.
- [Matriz de trazabilidad](../specs/07-traceability.md): requisito, escenario, prueba y control de CI.
- [Escenarios de aceptación](../specs/06-acceptance/README.md): el comportamiento esperado, en Gherkin.

## 5. ¿Cómo integro mi aplicación?

- [Guía de integración de terceros](manuales/integracion-terceros.md): el flujo OAuth 2.0 con PKCE paso a paso, el contenido del token y un checklist de seguridad, con Contabilidad como ejemplo.
- [Claves de servicio](manuales/claves-de-servicio.md): credenciales para sistemas que consultan el registro de auditoría sin sesión humana.
- [Contrato OpenAPI](../specs/03-api/openapi.yaml): los endpoints exactos.
- [`contabilidad/`](../contabilidad/README.md) e [`inventario/`](../inventario/README.md): el código de las dos aplicaciones de ejemplo, una por empresa.
- [ADR 0015](../specs/adr/0015-empresas-y-aislamiento-de-roles.md) y [escenarios de empresas](../specs/06-acceptance/empresas.feature): cómo el Hub separa los roles y los datos de cada empresa.

## 6. ¿Qué se decidió y por qué (ADR)?

- [Índice de decisiones (ADR 0001 a 0015)](../specs/adr/README.md): cada decisión con contexto, alternativas y consecuencias, incluidas las malas.
- Tres que ayudan a entender el resto: [IdP propio en lugar de Keycloak](../specs/adr/0002-idp-propio-en-lugar-de-keycloak.md), [SSO con Authorization Code y PKCE](../specs/adr/0009-sso-entre-dominios-con-authorization-code-y-pkce.md) y [línea base vulnerable deliberada](../specs/adr/0007-linea-base-vulnerable-deliberada.md).

## 7. ¿Cómo se desarrolló y se siguió el trabajo?

- [Bitácora](BITACORA.md): decisiones, hallazgos y avances, la entrada más reciente arriba.
- [Archivos de tareas](../odd/tasks/): el plan y el avance por semana, incluido el [cierre de la entrega](../odd/tasks/cierre-entrega.md).
- [Guía de desarrollo](guia-desarrollo.md): cómo se trabaja y se contribuye.
- [Scripts](../scripts/README.md): las utilidades de automatización.

## Estado final de la entrega

Con datos del repositorio al 2026-10-19:

- **`v1.0.0` está publicado** (tag del 2026-10-13, ejecución de `release.yml` 38460792159 en verde), con las imágenes `docker.io/jorgeivp/identity-hub-api`, `-worker` y `-web` firmadas con Cosign y con SBOM adjunto, y el release de GitHub con el informe técnico en PDF adjunto. Detalle en [`odd/tasks/cierre-entrega.md`](../odd/tasks/cierre-entrega.md) (tareas C2 y C3).
- **Matriz de trazabilidad:** 33 requisitos completos, 0 parciales, 0 sin cubrir y 0 diferidos ([matriz](../specs/07-traceability.md)).
- **Video:** el repositorio versiona solo el [guion](video/guion.md); la grabación la hace el equipo y no aparece versionada.
- **Trabajo futuro deliberado:** las mejoras de la integración de terceros (clientes en la base de datos, OpenID Connect, refresh tokens por cliente, introspección y revocación) y el outbox transaccional ([sección 9.1](manuales/integracion-terceros.md) de la guía de integración y [ADR 0006](../specs/adr/0006-publicacion-directa-sin-outbox.md)).

## Anexo: cumplimiento del enunciado

El contenido que sigue conecta los requisitos de `ENUNCIADO-TRABAJO-FINAL.md` con los artefactos realmente versionados. Es útil para calificar; para leer, use las secciones de arriba.

### Estructura del enunciado -> estructura del repositorio

| Estructura propuesta por el enunciado | Ubicación real | Justificación |
|---|---|---|
| `LICENSE` | [`../LICENSE`](../LICENSE) | Licencia Apache-2.0 versionada en la raíz. |
| `README.md` | [`../README.md`](../README.md) | Presenta el propósito, tecnologías, seguridad, inicio rápido y trazabilidad. |
| `docker-compose.yml` | [`../docker-compose.yml`](../docker-compose.yml) y [`../deploy/docker-compose.yml`](../deploy/docker-compose.yml) | El compose de raíz incluye el compose real de `deploy/`; `make setup` prepara el entorno local sin versionar secretos. |
| `.github/workflows/` | [`../.github/workflows/`](../.github/workflows/) y [`../e2e/`](../e2e/) | Los workflows automatizan CI/CD y ejecutan la suite E2E versionada. |
| `infraestructura/` | [`../infraestructura/`](../infraestructura/) | Módulos Terraform para la arquitectura AWS de referencia; no requieren una nube de pago para levantar el proyecto local. |
| `orquestacion/` | [`../deploy/`](../deploy/), [`../deploy/docker-compose.yml`](../deploy/docker-compose.yml) y [`../deploy/docker-compose.prod.yml`](../deploy/docker-compose.prod.yml) | `deploy/` concentra Docker Compose y observabilidad; el compose de producción simulada está terminado. |
| `servicios/` | [`../backend/`](../backend/), [`../frontend/`](../frontend/), [`../contabilidad/`](../contabilidad/), [`../inventario/`](../inventario/) y [`../e2e/`](../e2e/) | Los servicios se separan por responsabilidad; `e2e/` contiene sus pruebas de extremo a extremo. |
| `docs/` | [`docs/`](./), [`../specs/`](../specs/), [`../security/`](../security/) y [`../scripts/`](../scripts/) | `docs/` reúne la entrega; las especificaciones, evidencia de seguridad y trazabilidad versionada la respaldan. |

### Entregables y requisitos del enunciado

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
| Manual de usuario con capturas | 157–159 | [`manuales/usuario.md`](manuales/usuario.md) y sus 65 capturas en [`manuales/img/usuario/`](manuales/img/usuario/), regenerables con `make capturas` | hecho (T18) |
| Fase 1: Threat Dragon, DFD y STRIDE | 75–79 | [`threat-model/`](threat-model/) | hecho |
| Fase 2: hooks, SAST y SCA | 80–84 | [`../.pre-commit-config.yaml`](../.pre-commit-config.yaml) y [`../.github/workflows/ci.yml`](../.github/workflows/ci.yml) | hecho |
| Fase 3: build de imágenes, escaneo y gate de CVE | 85–89 | [`../.github/workflows/ci.yml`](../.github/workflows/ci.yml), [`../backend/Dockerfile`](../backend/Dockerfile) y [`../frontend/Dockerfile`](../frontend/Dockerfile) | hecho |
| Fase 4: pruebas unitarias y DAST | 90–93 | [`../.github/workflows/ci.yml`](../.github/workflows/ci.yml), [`../e2e/`](../e2e/) y los informes de OWASP ZAP (artefacto `zap-reports-<run_id>` de cada ejecución de CI; en local, `make scan-dast` los deja en `security/zap-reports/`, fuera de Git) | hecho |
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

### Entregables propios de la semana 4

No los exige el enunciado; amplían el alcance del Hub:

| Entregable | Ubicación actual | Estado |
|---|---|---|
| Grilla de roles y permisos configurable desde el Hub | [`../specs/adr/0013-roles-y-permisos-configurables-por-aplicacion.md`](../specs/adr/0013-roles-y-permisos-configurables-por-aplicacion.md) y [`../specs/06-acceptance/roles-y-permisos.feature`](../specs/06-acceptance/roles-y-permisos.feature) | hecho (T11 a T15; RF-021) |
| Guía de integración de plataformas de terceros | [`manuales/integracion-terceros.md`](manuales/integracion-terceros.md) | completa; la integración OAuth/OIDC de terceros y el outbox quedan como trabajo futuro (sección 9.1) |
| Claves de servicio para integraciones máquina (RF-018) | [`manuales/claves-de-servicio.md`](manuales/claves-de-servicio.md) y [`../specs/adr/0014-claves-de-servicio-para-integraciones-maquina.md`](../specs/adr/0014-claves-de-servicio-para-integraciones-maquina.md) | hecho (F4) |
| Exportación del registro de auditoría en CSV y JSON (RF-019) | [`manuales/usuario.md`](manuales/usuario.md) y [`../specs/06-acceptance/`](../specs/06-acceptance/) | hecho (F1) |
| Runbook de alertas de Falco | [`runbooks/falco-alerta.md`](runbooks/falco-alerta.md) | hecho (F3) |
| Empresas y aislamiento de roles (RF-022) | [`../specs/adr/0015-empresas-y-aislamiento-de-roles.md`](../specs/adr/0015-empresas-y-aislamiento-de-roles.md), [`../specs/06-acceptance/empresas.feature`](../specs/06-acceptance/empresas.feature) y la aplicación [`../inventario/`](../inventario/) | hecho (R1 a R5) |
