# Modelo de amenazas de Identity Hub

Este directorio contiene el modelo versionado de OWASP Threat Dragon para la entrega. Abra [`identity-hub.json`](identity-hub.json) y use este documento para revisar las fronteras, los flujos y el registro STRIDE. La fuente de verdad sigue siendo [`specs/05-security/threat-model.md`](../../specs/05-security/threat-model.md); el JSON no sustituye sus requisitos ni sus gates.

## Abrir el modelo

1. En Threat Dragon Desktop, seleccione **Open model** y abra `identity-hub.json`.
2. En Threat Dragon Web, use **Open a model from your computer** y seleccione el mismo archivo.
3. Revise primero `DFD nivel 0 — contexto` y luego `DFD nivel 1 — Identity Hub`.

El archivo conserva la estructura y el `version` de modelo observados en las demostraciones oficiales de Threat Dragon suministradas con esta tarea. No se validó mediante la interfaz de Threat Dragon en este entorno.

## Diagramas y fronteras de confianza

| Diagrama | Propósito | Fronteras |
|---|---|---|
| `DFD nivel 0 — contexto` | Muestra el navegador, Identity Hub y la cadena CI/CD → registry → despliegue. | Internet/no confiable, Identity Hub y cadena de suministro. |
| `DFD nivel 1 — Identity Hub` | Detalla Nginx, API, PostgreSQL, audit log, RabbitMQ, worker, SMTP/Mailpit y observabilidad. | Internet/no confiable, DMZ del contenedor web y red de aplicación Docker. |

Las fronteras señalan dónde cambia la confianza: del navegador a Nginx, de Nginx a la API, de la API a PostgreSQL y de la API al broker/worker. La cadena CI/CD es una frontera separada porque gobierna los artefactos que llegan al despliegue. Los flujos declaran protocolo y las banderas de red pública, cifrado y bidireccionalidad; el tráfico interno actual se representa explícitamente como interno, no como una afirmación de cifrado de transporte.

## Registro STRIDE

`Mitigated` indica que la contramedida descrita en el modelo fuente está implementada en el repositorio. `Open` marca las tres amenazas cuya contramedida falta o está incompleta (AM-008, AM-009 y AM-022); cada fila dice qué falta y qué tarea de la semana 4 lo cierra.

| Amenaza | Elemento del DFD | Categoría | Estado | Contramedida | Evidencia/especificación |
|---|---|---|---|---|---|
| AM-001 | Solicitud y sesión de autenticación | Spoofing | Mitigated | Argon2id, bloqueo tras 5 fallos y `limit_req` en Nginx. | [`threat-model.md`](../../specs/05-security/threat-model.md) — RF-017 |
| AM-002 | Evento de notificación | Spoofing | Mitigated | Rotación de refresh token y revocación de familia ante reuso. | [`threat-model.md`](../../specs/05-security/threat-model.md) — RF-005, RF-006 |
| AM-003 | HTTP interno: `/api` | Spoofing | Mitigated | Aceptar solo `EdDSA` antes de verificar la firma. | [`threat-model.md`](../../specs/05-security/threat-model.md) — RF-004 |
| AM-004 | Solicitud y sesión de autenticación | Spoofing | Mitigated | Respuestas iguales, comparación constante y hash señuelo. | [`threat-model.md`](../../specs/05-security/threat-model.md) — RF-001, RF-015 |
| AM-005 | HTTP interno: `/api` | Spoofing | Mitigated | El restablecimiento revoca todas las sesiones activas. | [`threat-model.md`](../../specs/05-security/threat-model.md) — RF-015 |
| AM-017 (MFA) | HTTPS: credenciales, MFA y sesión | Spoofing | Mitigated | Desafío SHA-256/HMAC-SHA256, uso único, vencimiento y límites de intentos. | [`threat-model.md`](../../specs/05-security/threat-model.md) — RF-013, RF-014, RF-017 |
| AM-006 | SQL parametrizado | Tampering | Mitigated | Consultas parametrizadas generadas por `sqlc`. | [`threat-model.md`](../../specs/05-security/threat-model.md) — RF-010 |
| AM-007 | HTTP interno: `/api` | Tampering | Mitigated | Roles consultados desde la base de datos en peticiones sensibles. | [`threat-model.md`](../../specs/05-security/threat-model.md) — RF-009 |
| AM-008 | Imagen firmada por digest | Tampering | Open | Firma Cosign keyless y despliegue por digest. Pendiente (T17): todavía no hay firma Cosign ni workflow de despliegue (cd.yml); las imágenes solo se construyen y escanean en CI. | [`threat-model.md`](../../specs/05-security/threat-model.md) — RNF-006 |
| AM-009 | Imagen firmada por digest | Tampering | Open | SBOM CycloneDX, `osv-scanner`, Dependabot y lockfiles. Parcial: osv-scanner, govulncheck, npm audit y lockfiles activos en CI; falta el SBOM CycloneDX por imagen (T17). | [`threat-model.md`](../../specs/05-security/threat-model.md) — RNF-004 |
| AM-010 | Evento de auditoría | Repudiation | Mitigated | Audit log append-only con actor, IP y user-agent; sin `UPDATE`/`DELETE`. | [`threat-model.md`](../../specs/05-security/threat-model.md) — RF-011 |
| AM-011 | Logs estructurados | Repudiation | Mitigated | Rol de aplicación sin borrado y envío de logs a Loki. | [`threat-model.md`](../../specs/05-security/threat-model.md) — RF-011, RNF-007 |
| AM-012 | Imagen firmada por digest | Information disclosure | Mitigated | Secretos por entorno, sin valores por defecto y `.env` fuera de Git. | [`threat-model.md`](../../specs/05-security/threat-model.md) — RNF-003 |
| AM-013 | HTTP interno: `/api` | Information disclosure | Mitigated | `Secret.String()` redactado e identificadores truncados a 8 caracteres. | [`threat-model.md`](../../specs/05-security/threat-model.md) — RNF-012 |
| AM-014 | Artefacto de despliegue | Information disclosure | Mitigated | En producción no se publica ningún puerto de base de datos ni broker. Implementado (T9): `deploy/docker-compose.prod.yml` elimina las publicaciones de puertos; solo `web` expone 8080. | [`threat-model.md`](../../specs/05-security/threat-model.md) — RNF-001 |
| AM-015 | Solicitud y sesión de autenticación | Information disclosure | Mitigated | CSP estricta, cookie `HttpOnly` y escape predeterminado de React. | [`threat-model.md`](../../specs/05-security/threat-model.md) — RNF-009 |
| AM-016 | Evento de notificación | Information disclosure | Mitigated | Token de 24 h y un uso; broker interno y worker sin registrar cuerpo. | [`threat-model.md`](../../specs/05-security/threat-model.md) — RF-002 |
| AM-017 (DoS) | Solicitud y sesión de autenticación | Denial of service | Mitigated | `limit_req` antes de API y Argon2id calibrado a ~250 ms. | [`threat-model.md`](../../specs/05-security/threat-model.md) — RNF-010 |
| AM-018 | HTTP interno: `/api` | Denial of service | Mitigated | `client_max_body_size 1m` y `http.MaxBytesReader`. | [`threat-model.md`](../../specs/05-security/threat-model.md) |
| AM-019 | Cola y DLQ | Denial of service | Mitigated | Regla de alerta de Grafana versionada (`deploy/observability/grafana/alerting/dlq.yml`) que se dispara con cualquier mensaje en `notifications.dlq` durante 2 min (umbral más estricto que el > 10 original: un mensaje ya es un correo perdido) y runbook de atención y purga. | [`runbooks/dlq-notificaciones.md`](../runbooks/dlq-notificaciones.md) — RNF-007 |
| AM-020 | Artefacto de despliegue | Elevation of privilege | Mitigated | Usuario no-root, capacidades eliminadas, raíz de solo lectura y base distroless. | [`threat-model.md`](../../specs/05-security/threat-model.md) — RNF-008 |
| AM-021 | HTTP interno: `/api` | Elevation of privilege | Mitigated | Autorización por ruta comprobada del lado del servidor. | [`threat-model.md`](../../specs/05-security/threat-model.md) — RF-009 |
| AM-022 | Imagen firmada por digest | Elevation of privilege | Open | Permisos mínimos por job y OIDC keyless. Parcial: permisos mínimos por job en ci.yml; la publicación con OIDC keyless en el registry llega con T17. | [`threat-model.md`](../../specs/05-security/threat-model.md) — RNF-006 |

## Nota sobre la evidencia de AM-014

La especificación cita como compuerta de AM-014 "Trivy config sobre el compose de producción". El Trivy local (`trivy config --help`) lista como escáneres de misconfiguración azure-arm, cloudformation, dockerfile, helm, kubernetes, terraform, terraformplan-json, terraformplan-snapshot y ansible; ninguno es Docker Compose. Por eso la evidencia real de AM-014 es la comprobación `docker compose config` de T9 (ningún servicio salvo `web` publica puertos), no Trivy config.
