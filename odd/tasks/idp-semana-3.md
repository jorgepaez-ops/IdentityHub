# idp-semana-3 — SSO entre dominios, MFA por correo, alta de empleados y frontend funcional

Documento de feature (ODD). Es el único archivo de tareas de esta feature; el espejo en memoria
(`odd/idp-semana-3/tasks`) lo reconcilia Claude. Codex edita este archivo (casillas, `Commit:`,
handoff, preguntas); lee también `AGENTS.md` en la raíz del repo.

## Objetivo

Que Identity Hub funcione de punta a punta como lo haría un Entra ID pequeño, y que se pueda
demostrar en vivo en clase: un admin da de alta a un empleado con su cargo, el empleado recibe la
invitación y el código de segundo factor en un buzón (Mailpit), inicia sesión y entra a una
aplicación de negocio **en otro dominio** (Contabilidad) sin volver a escribir la contraseña, y
esa aplicación le muestra solo las vistas de su rol. Todo con los mismos gates de la semana 2 y
con DAST (ZAP) y E2E (Playwright) añadidos al pipeline.

## Problema y motivo

- El backend de la semana 2 está completo y probado, pero no tiene interfaz: el frontend es el
  esqueleto de la semana 1 (`frontend/src/App.tsx`, solo health/readiness). Sin UI no hay historia
  de usuario demostrable, ni manual con capturas, ni video.
- El "SSO" no existe todavía: una aplicación en otro dominio no puede usar la sesión del Hub (la
  cookie de refresh es `SameSite=Strict`, `Path=/api/v1/auth`, y no hay endpoints de autorización).
- Los mockups de la semana 2 asumen capacidades que la API no tiene: roles de negocio
  (`contador_senior`, `analista_contable`; el `Role` del OpenAPI es un enum `[admin, user]`) y
  alta de usuarios por un admin (no existe `POST /api/v1/admin/users`).
- Quedan seis requisitos diferidos por Q14 de la semana 2 (RF-013 a RF-016, RF-018, RF-019) y
  pendientes chicos anotados al cerrarla.

## Referencias

- Mockups (claude.ai, privados del usuario): **Identity Hub Console**
  https://claude.ai/artifact/P7haQLMveVuxaGRrTfsdrY y **Contabilidad Ledger**
  https://claude.ai/artifact/Xf2UF7Ynm1AwE8ePKSGRE5 (ambos del 2026-09-25; HTML y JS sin
  framework, sistema visual compartido: IBM Plex Sans/Mono, Space Grotesk en el Hub, Fraunces en
  Contabilidad, acento azul `#2d4f8f` en el Hub y ámbar `#8a5a12` en Contabilidad, modo oscuro).
  Referencia de pantallas de cuenta (MFA, sesiones): **Identity Hub Mockup**
  https://claude.ai/artifact/AhucmxcE1sxe239yXJZjbR (2026-09-12, boceto anterior más amplio).
- Cierre de la semana 2: `odd/tasks/idp-semana-2.md`, `docs/BITACORA.md`, tag `v0.1.0-hardened`.

## Alcance

**Dentro:**
- SSO entre dominios con OAuth 2.0 authorization code + PKCE en versión mínima (un cliente
  registrado en configuración: Contabilidad).
- Alta de empleados por un admin, con invitación por correo para definir la contraseña, y roles de
  negocio.
- MFA con código de un solo uso enviado por correo (RF-013 y RF-014 enmendados; sustituye a TOTP).
- RF-015 (restablecimiento de contraseña) y RF-016 (sesiones activas).
- Frontend: consola de Identity Hub y aplicación Contabilidad, según los mockups; refresh
  compartido entre pestañas (ADR 0005, Q4 de la semana 2).
- Dominios locales separados con HTTPS local (`mkcert`).
- DAST con OWASP ZAP en CI y E2E con Playwright (escenarios Gherkin corregidos al contrato vigente).
- Pendientes chicos de la semana 2 (ver T10).

**Fuera (semana 4, entrega):** publicación en Docker Hub con SBOM (Syft) y firma (Cosign), IaC con
Checkov, `docker-compose.prod.yml`, Threat Dragon, diagramas UML faltantes, manuales, VULN-019
(las 6 imágenes del compose restantes), informe PDF y video.

**Fuera salvo que sobre tiempo:** RF-018 (claves de servicio) y RF-019 (exportación del audit
log). Siguen **diferidos** en la matriz de trazabilidad, no olvidados.

**Fuera de alcance del producto (se documenta como límite):** OIDC completo (discovery, registro
dinámico de clientes, consentimiento, `id_token`), TOTP/WebAuthn, correo real por internet
(Mailpit hace de servidor de correo en el entorno de demo).

## Restricciones

- `specs/` manda; el OpenAPI (camelCase, RFC 7807) es el contrato y `gen.go` no se edita a mano.
  Las enmiendas de esta feature son las de T3; ninguna otra tarea toca `specs/` sin volver a
  "Cambios de spec propuestos".
- Nunca debilitar un gate. Nunca secretos reales en el repo. Los certificados de `mkcert` y su CA
  local **no se versionan** (se generan con un comando documentado).
- Controles de seguridad del flujo OAuth, no negociables: `redirect_uri` con coincidencia exacta,
  `state` obligatorio, PKCE `S256` obligatorio (sin `plain`), código de autorización de un solo
  uso con vencimiento corto, cliente y `redirect_uri` fijados en configuración, CORS abierto solo
  en `/token` y sin credenciales. Cada control lleva su prueba.
- Codex no tiene red ni Docker (ver `CLAUDE.md`): lo que exija `npm install` de paquetes nuevos,
  `docker compose`, `mkcert` o ZAP lo cierra Claude.
- Tamaño orientativo por tarea: ~400 líneas de cambio (heurística, no tope).
- Estrategia de entrega: `ask-on-risk`. Commits de unidad de trabajo en `feat/idp-semana-3`;
  push, PR y merge los decide el usuario (un PR por corte de fase, merge commit).

## Modo TDD resuelto

- **TDD estricto: habilitado** (fuente: configuración del orquestador, "Strict TDD Mode:
  enabled"). RED, GREEN y REFACTOR observados por tarea.
- **Ejecutores:** backend `make test-go` (`go test -race ./...`) e integración
  `make test-integration` (tag `integration`, PostgreSQL real); frontend `npm run test` (Vitest);
  E2E `make e2e` (Playwright, lo crea T14).

## Criterios de aceptación de la feature

1. Demo en vivo reproducible con un guion documentado: alta de empleado → correo de invitación en
   Mailpit → definir contraseña → login con código MFA desde Mailpit → abrir Contabilidad en su
   dominio → redirección al Hub → vuelta sin pedir la contraseña si ya había sesión → vistas según
   el rol.
2. Los dos dominios responden por HTTPS local y la sesión del Hub no es legible desde Contabilidad
   (la aplicación solo recibe el token por el flujo de autorización).
3. Cada control del flujo OAuth tiene prueba que falla si se quita.
4. `ci.yml` en verde con los jobs nuevos de ZAP y Playwright; ningún gate debilitado.
5. Matriz de trazabilidad al día: RF-013 a RF-016 y los RF nuevos de T3 cubiertos; RF-018 y
   RF-019 visibles como diferidos si no entran.
6. Hallazgos nuevos de ZAP u otros gates: ficha propia o anotados para la semana 4, sin inventar ids.

## Decisiones tomadas (2026-09-26 y 2026-09-27, con el usuario)

- **D1 · Stack.** Go para backend y worker es válido: el profesor dejó libre la elección
  tecnológica siempre que se cumplan los requisitos de DevSecOps y el alcance de gestor de
  identidad; el gráfico del curso con Python/Node eran sugerencias. Se deja constancia en T1.
- **D2 · Registro de imágenes.** Docker Hub (obligatorio del curso, `vX.Y.Z` y `latest`), al
  final del proyecto (semana 4). Sustituye a GHCR de la ADR 0008; se documenta en T2.
- **D3 · Dominios.** Identity Hub y Contabilidad en dominios distintos, como en un despliegue real
  (el Hub hace de Entra ID; Contabilidad es la aplicación del usuario). Rutas bajo un mismo origen
  quedan descartadas: harían el SSO trivial e indemostrable.
- **D4 · SSO.** OAuth 2.0 authorization code + PKCE en versión mínima, un cliente, HTTPS local con
  `mkcert`.
- **D5 · MFA.** Código de un solo uso por correo (Mailpit), no TOTP/QR, por tiempo y alcance. Se
  documenta que es más débil que TOTP (depende del buzón) y que se aceptó conscientemente.
- **D6 · Alta de empleados.** La hace un admin con el cargo/rol; el empleado define su contraseña
  con un enlace de invitación de un solo uso (mismo mecanismo de token que RF-015). El admin nunca
  conoce la contraseña.
- **D7 · Buzón de la demo.** La interfaz web de Mailpit (`:8025`) es el buzón que se muestra en
  clase; los comandos `curl` quedan como plan B y Playwright lee los correos por la API de Mailpit.

## Preguntas abiertas (una a la vez, las decide el usuario)

- **P1 · Catálogo de roles.** ¿Los roles de negocio son exactamente los del mockup (`admin`,
  `contador_senior`, `analista_contable`) o hay más? ¿`user` se conserva?
- **P2 · Autoregistro (RF-001).** Con alta por admin, ¿el registro público sigue abierto (como una
  cuenta de invitado) o se cierra, como en un directorio corporativo?
- **P3 · Nombres de dominio locales.** Propuesta: `hub.empresa.test` y `contabilidad.test`
  (`.test` está reservado, RFC 2606).
- **P4 · MFA obligatorio u opcional.** ¿Todos los empleados, solo admins, o a elección del usuario?

---

## Fase 0 — Enmiendas de spec y decisiones (sin código)

### T1 — Documentación desactualizada y constancia de decisiones
- [ ] Estado · Ejecutor: `Codex` · Solo docs
- README (estado "Semana 2 de 4", CI rojo, repo en línea base), `AGENTS.md` (Go 1.25 y
  `x/crypto` v0.17.0 → Go 1.26 tras T34b) y la ficha VULN-028 (dice que Dockerfile y CI siguen en
  Go 1.25). Nota breve en README sobre D1 (stack elegido con aval del profesor).
- Verificación: `python3 scripts/traceability.py --check`.
- Commit: —

### T2 — ADRs: SSO mínimo, MFA por correo, Docker Hub
- [ ] Estado · Ejecutor: `Claude` (decisión de arquitectura) · Solo docs
- ADR 0009 (SSO entre dominios con authorization code + PKCE, alcance mínimo y límites frente a
  OIDC), ADR 0010 (MFA por correo en lugar de TOTP; enmienda RF-013/014) y ADR 0011 (Docker Hub;
  sustituye la parte de registro de la ADR 0008, que queda "sustituida parcialmente").
- Commit: —

### T3 — Enmienda de requisitos, OpenAPI y escenarios
- [ ] Estado · Ejecutor: `Codex` · Depende de: T2 y P1, P2, P4
- `specs/01-requirements.md`: RF-013/014 a código por correo; requisito nuevo de alta de empleados
  por admin con invitación; requisito nuevo del flujo de autorización para aplicaciones cliente;
  roles de negocio en el modelo de dominio.
- `specs/03-api/openapi.yaml`: `POST /api/v1/admin/users`, aceptación de invitación, endpoints de
  RF-013 a RF-016 ajustados, `GET /oauth/authorize` y `POST /oauth/token`, `Role` ampliado.
- `specs/06-acceptance/*.feature`: corregir los escenarios que esperan `refreshToken` en el cuerpo
  (va en cookie desde C1 de la semana 2) y añadir los escenarios nuevos.
- Regenerar `gen.go` con `~/go/bin/oapi-codegen`; `make spec-drift` en verde.
- Commit: —

## Fase 1 — Backend

### T4 — Roles de negocio
- [ ] Estado · Ejecutor: `Codex` · Depende de: T3
- Migración y catálogo de roles; RBAC y claims `roles` del JWT con los valores nuevos; pruebas de
  que un rol desconocido se rechaza.
- Commit: —

### T5 — Alta de empleados por admin e invitación por correo
- [ ] Estado · Ejecutor: `Codex` · Depende de: T3, T4
- `POST /api/v1/admin/users` (solo admin, auditado), token de invitación de un solo uso con
  vencimiento, plantilla del worker, endpoint para definir la contraseña con el token.
- Commit: —

### T6 — RF-015 Restablecimiento de contraseña
- [ ] Estado · Ejecutor: `Codex` · Depende de: T5 (reutiliza el mecanismo de token)
- Respuesta no enumerable, token de una hora, revocación de las sesiones anteriores al cambiar.
- Commit: —

### T7 — RF-013/014 MFA por correo
- [ ] Estado · Ejecutor: `Codex` · Depende de: T3
- Activación, desafío `202` con `mfa_token` temporal, código de 6 dígitos con vencimiento corto,
  límite de intentos y auditoría. Quitar el rechazo explícito de cuentas con MFA de la semana 2.
- Commit: —

### T8 — RF-016 Sesiones activas
- [ ] Estado · Ejecutor: `Codex` · Depende de: T3
- Listar sesiones (IP, user-agent, creación, último uso) y revocar una concreta.
- Commit: —

### T9 — Autorización para aplicaciones cliente (authorization code + PKCE)
- [ ] Estado · Ejecutor: `Codex` (con revisión reforzada de Claude) · Depende de: T3, T7
- Sesión del Hub en su dominio; `GET /oauth/authorize` (valida cliente, `redirect_uri` exacto,
  `state`, `code_challenge` S256; si no hay sesión, lleva al login del Hub con MFA) y
  `POST /oauth/token` (canjea código + `code_verifier` por access token; código de un solo uso).
  Cliente Contabilidad fijado en configuración. Una prueba por cada control de "Restricciones".
- Commit: —

### T10 — Pendientes chicos de la semana 2
- [ ] Estado · Ejecutor: `Codex`
- `POST /api/v1/auth/login` con `{}` responde 500 en vez de 400; `/readyz` devuelve
  `err.Error()` por dependencia (puede filtrar host/usuario/base); investigar la intermitencia de
  `TestRF001_UsersAceptaArgon2idYRechazaMD5`.
- Commit: —

## Fase 2 — Dominios locales y frontend

### T11 — Dominios separados con HTTPS local
- [ ] Estado · Ejecutor: `Claude` (Docker y `mkcert`; Codex no puede) · Depende de: P3
- Dos `server` en Nginx (Hub y Contabilidad) con TLS local, entradas de `/etc/hosts` documentadas,
  certificados fuera del repo, cabeceras RNF-009 en ambos dominios.
- Commit: —

### T12 — Consola de Identity Hub (React)
- [ ] Estado · Ejecutor: `Codex` · Depende de: T5, T7, T8, T11
- Según el mockup: login con MFA, directorio de usuarios, alta y edición (estado y roles, regla de
  auto-deshabilitado de RF-010), audit log; páginas de cuenta: verificar correo
  (`/verify-email?token=…`, Q15), definir contraseña, restablecer, perfil y sesiones. Refresh
  compartido entre pestañas (Web Locks o `BroadcastChannel`).
- Commit: —

### T13 — Aplicación Contabilidad (React, otro dominio)
- [ ] Estado · Ejecutor: `Codex` · Depende de: T9, T11
- Según el mockup: inicio de sesión por redirección con PKCE, verificación del JWT con el JWKS del
  Hub, vistas por rol (Resumen, Transacciones, Cierre contable, Administración) con datos de
  ejemplo. Sin usuarios propios.
- Commit: —

## Fase 3 — Verificación, DAST y cierre

### T14 — E2E con Playwright
- [ ] Estado · Ejecutor: `Claude` (instala Playwright y navegadores) + `Codex` (pruebas)
- Una prueba por escenario Gherkin, lectura de correos por la API de Mailpit, el guion de la demo
  como prueba; `make e2e` real y job en CI; `spec-drift` comprueba la correspondencia.
- Commit: —

### T15 — DAST con OWASP ZAP en CI
- [ ] Estado · Ejecutor: `Claude`
- ZAP contra el stack levantado en CI (baseline y escaneo de API con el OpenAPI); umbral que rompe
  la build; hallazgos a fichas.
- Commit: —

### T16 — Hook de pre-commit real
- [ ] Estado · Ejecutor: `Claude`
- `pre-commit install` documentado en `docs/guia-desarrollo.md` y verificado con un secreto de
  prueba que el hook bloquea (sin llegar a commitearse).
- Commit: —

### T17 — Revisión de fase, trazabilidad, bitácora e informe
- [ ] Estado · Ejecutor: `Claude (revisión)`
- Matriz de trazabilidad, entrada en `docs/BITACORA.md`, guion de la demo en `docs/`, informe de
  seguridad (tercera versión si hay hallazgos de ZAP).
- Commit: —

---

## Progreso

| Fase | Tareas | Hechas |
|---|---|---|
| 0 — Enmiendas de spec y decisiones | T1 a T3 (3) | 0 |
| 1 — Backend | T4 a T10 (7) | 0 |
| 2 — Dominios locales y frontend | T11 a T13 (3) | 0 |
| 3 — Verificación, DAST y cierre | T14 a T17 (4) | 0 |
| **Total** | **17** | **0** |

## Siguiente paso

Resolver P1 a P4 con el usuario (una a la vez); después T1 y T2 en paralelo (docs), y T3.

## Cambios de spec propuestos

Todos en T3, tras T2 y las respuestas a P1, P2 y P4.

## Notas de handoff Codex

(vacío)
