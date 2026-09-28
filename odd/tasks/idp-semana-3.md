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
- Dominios locales separados sin preparación del equipo: `identityhub.localhost` y
  `contabilidad.localhost` por HTTP (D10).
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
- Nunca debilitar un gate. Nunca secretos reales en el repo. Ningún certificado ni clave TLS se
  versiona (el modo HTTPS opcional de la semana 4 los genera en un contenedor).
- Controles de seguridad del flujo OAuth, no negociables: `redirect_uri` con coincidencia exacta,
  `state` obligatorio, PKCE `S256` obligatorio (sin `plain`), código de autorización de un solo
  uso con vencimiento corto, cliente y `redirect_uri` fijados en configuración, CORS abierto solo
  en `/oauth/token` y en el JWKS, sin credenciales y solo para el origen del cliente (ADR 0009). Cada control lleva su prueba.
- Codex no tiene red ni Docker (ver `CLAUDE.md`): lo que exija `npm install` de paquetes nuevos,
  `docker compose`, navegadores o ZAP lo cierra Claude.
- **Portabilidad (requisito de la entrega):** un compañero clona el repo, ejecuta
  `docker compose up -d` y el proyecto funciona, sin instalar nada más que Docker ni editar archivos
  del sistema. Ninguna decisión de esta feature puede romperlo.
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
2. Los dos dominios responden en Chrome y Firefox sin tocar `/etc/hosts` ni instalar certificados,
   y la sesión del Hub no es legible desde Contabilidad (la aplicación solo recibe el token por el
   flujo de autorización).
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
- **D4 · SSO.** OAuth 2.0 authorization code + PKCE en versión mínima, un cliente (dominios y
  transporte en D10).
- **D5 · MFA.** Código de un solo uso por correo (Mailpit), no TOTP/QR, por tiempo y alcance. Se
  documenta que es más débil que TOTP (depende del buzón) y que se aceptó conscientemente.
- **D6 · Alta de empleados.** La hace un admin con el cargo/rol; el empleado define su contraseña
  con un enlace de invitación de un solo uso (mismo mecanismo de token que RF-015). El admin nunca
  conoce la contraseña.
- **D7 · Buzón de la demo.** La interfaz web de Mailpit (`:8025`) es el buzón que se muestra en
  clase; los comandos `curl` quedan como plan B y Playwright lee los correos por la API de Mailpit.

- **D8 · Modelo de roles (P1, 2026-09-27).** Roles del directorio separados de los roles de
  aplicación, como los *app roles* de Entra ID:
  - Directorio (Identity Hub): `admin` (sistemas: alta de empleados y asignación de accesos) y
    `user` (rol base de toda cuenta; sin roles de aplicación = sin acceso a aplicaciones).
  - Aplicación Contabilidad: `contabilidad.senior` (todas las opciones, incluidos aprobar y cierre
    del mes) y `contabilidad.analista` (solo sus transacciones, sin cierre).
  - El token emitido para una aplicación lleva solo los roles de esa aplicación.
  - Separación de funciones: `admin` no ve datos contables salvo que se le asigne un rol de
    Contabilidad, y **un admin no puede asignarse roles a sí mismo** (lo hace otro admin, auditado;
    análogo a la regla de auto-deshabilitado de RF-010).
  - Escena de la demo: empleado con solo `user` se autentica bien (MFA incluido) y Contabilidad le
    muestra "sin acceso": autenticar y autorizar son pasos distintos.

- **D9 · Autoregistro cerrado (P2, 2026-09-27).** Como en un directorio corporativo, nadie se
  registra solo: la única forma de tener cuenta es el alta por un admin con invitación (D6).
  `POST /api/v1/auth/register` se retira del contrato con enmienda documentada en T3; su lógica
  (hash Argon2id, eventos, plantillas) se reutiliza en el alta por admin. RF-001 se reescribe como
  alta de empleados, y RF-002 queda cubierto por la invitación: definir la contraseña con el enlace
  prueba la posesión del correo. Menos superficie de ataque (cuentas falsas, abuso de envío de
  correos).

- **D10 · Dominios locales y portabilidad (P3, 2026-09-27).** `identityhub.localhost` (Hub:
  consola y login) y `contabilidad.localhost` (aplicación). Chrome y Firefox resuelven `*.localhost`
  a la propia máquina sin `/etc/hosts` y lo tratan como contexto seguro, así que la cookie `Secure`
  funciona por HTTP sin certificados. Son sitios distintos: `localhost` no está en la Public Suffix
  List (verificado 2026-09-27), así que cada `*.localhost` es su propio sitio y el SSO entre dominios
  sigue siendo demostrable. Se descartan `.test` y `mkcert` por exigir preparar el equipo. Costos
  aceptados: la demo es en Chrome o Firefox (Safari no resuelve `*.localhost` igual) y HSTS no aplica
  por HTTP. **HTTPS opcional en la semana 4**: certificado autofirmado con SAN de ambos nombres,
  generado con `openssl` por un servicio de una sola ejecución del compose (idea tomada del
  `generate-dev-cert.sh` de otro proyecto del usuario, sin `ifconfig` ni dependencias del host) y un
  comando documentado por sistema operativo para confiar en él. T11 confirma con Playwright en
  Chromium y Firefox la resolución de `*.localhost` y las cookies `Secure` antes de construir encima;
  si falla, se vuelve a `.test` con script.

- **D11 · MFA obligatorio para todos (P4, 2026-09-27).** Todo inicio de sesión exige el código de
  un solo uso por correo (D5), como una política de acceso condicional de Entra ID. Como el correo
  ya queda verificado con la invitación (D9), no hay paso de "activar el segundo factor": RF-013 se
  reduce a la política y RF-014 al desafío en el login.

- **D12 · Reenvío de invitación (2026-09-27).** Un admin puede reenviar la invitación de una cuenta
  `pending_verification`: `POST /api/v1/admin/users/{userId}/invitation` anula los tokens de
  invitación anteriores, emite uno nuevo de 24 h y queda auditado. Sin esto, una invitación vencida
  o perdida deja la cuenta bloqueada para siempre (hallazgo R4 de la revisión nativa de T5). Va en
  T6 (mismo mecanismo de token) con enmienda al spec; la consola (T12) muestra la acción.
- **D13 · El restablecimiento desbloquea (2026-09-28).** Un restablecimiento completado pasa una
  cuenta `locked` (RF-017) a `active` y el conteo de intentos fallidos ignora los anteriores al
  restablecimiento. El bloqueo frena la adivinanza de contraseña; el restablecimiento exige
  controlar el correo, un factor distinto, así que desbloquear no agrega exposición y evita
  depender de soporte. Queda auditado aparte. No reactiva cuentas deshabilitadas ni pendientes.

## Preguntas abiertas

Ninguna: P1 a P4 resueltas en D8 a D11; D12 y D13 salieron de revisiones.

---

## Fase 0 — Enmiendas de spec y decisiones (sin código)

### T1 — Documentación desactualizada y constancia de decisiones
- [x] Estado · Ejecutor: `Codex` · Solo docs
- README (estado "Semana 2 de 4", CI rojo, repo en línea base), `AGENTS.md` (Go 1.25 y
  `x/crypto` v0.17.0 → Go 1.26 tras T34b) y la ficha VULN-028 (dice que Dockerfile y CI siguen en
  Go 1.25). Nota breve en README sobre D1 (stack elegido con aval del profesor).
- Verificación: `python3 scripts/traceability.py --check`.
- Hecho (2026-09-27, Codex; revisión de Claude). README reorganizado en 10 secciones para el
  evaluador y para quien clone el repo: descripción, badges reales (CI, versión por tag, licencia;
  cobertura pendiente de un servicio que la publique, sin URL inventada, y el gate del 70 % del job
  "6b · Pruebas de integración" citado en el texto), estado, tecnologías con la nota de D1,
  arquitectura, inicio rápido honesto (hoy `.env` + `make up`; la portabilidad llega en la semana 4),
  mapa de evidencia de seguridad, gestión y trazabilidad, estructura y licencia. 27 enlaces
  relativos comprobados. `AGENTS.md`: Go 1.26 y versiones reales de go.mod. VULN-028: Dockerfile y
  CI ya en Go 1.26. Claude verificó cada dato contra `go.mod`, `backend/Dockerfile`, `ci.yml` y el
  `Makefile`, y corrigió que `specs/adr/README.md` no es un índice (ahora enlaza la carpeta).
  El enunciado original del README no está en el repo: se usó `ANALISIS-REQUISITOS.md` §6.
- Commit: `892e906`

### T2 — ADRs: SSO mínimo, MFA por correo, Docker Hub
- [x] Estado · Ejecutor: `Claude` (decisión de arquitectura) · Solo docs
- ADR 0009 (SSO entre dominios con authorization code + PKCE, alcance mínimo y límites frente a
  OIDC), ADR 0010 (MFA por correo en lugar de TOTP; enmienda RF-013/014) y ADR 0011 (Docker Hub;
  sustituye la parte de registro de la ADR 0008, que queda "sustituida parcialmente").
- Hecho (2026-09-27). Decisiones de diseño que fija la ADR 0009 y que T3 y T9 deben respetar:
  - La sesión del Hub usa una cookie propia `SameSite=Lax` (una llegada a `/oauth/authorize` es
    una navegación de primer nivel desde otro sitio y `Strict` no la acompañaría); el refresh de la
    consola conserva su cookie `Strict`. Su representación en base de datos se fija en T3.
  - Contabilidad es cliente público (SPA sin secreto): PKCE sustituye al secreto.
  - Sin refresh token para la aplicación: al vencer el access token vuelve a `/oauth/authorize`.
  - Access token con `aud` = cliente y solo los roles de esa aplicación (D8).
  - CORS también en el JWKS (la aplicación verifica la firma en el navegador); se corrigió la
    restricción de este archivo, que solo mencionaba `/token`.
  - Límites documentados: no es OIDC (sin `id_token`, discovery, `userinfo` ni cierre de sesión
    único).
  - ADR 0010 deja constancia de que NIST SP 800-63B no admite el correo como autenticador fuera de
    banda: riesgo aceptado a sabiendas por alcance.
- Commit: `dc681e5`

### T3 — Enmienda de requisitos, OpenAPI y escenarios
- [x] Estado · Ejecutor: `Codex` · Depende de: T2 (decisiones D8 a D11)
- `specs/01-requirements.md`: RF-013/014 a código por correo; requisito nuevo de alta de empleados
  por admin con invitación; requisito nuevo del flujo de autorización para aplicaciones cliente;
  roles de negocio en el modelo de dominio.
- `specs/03-api/openapi.yaml`: `POST /api/v1/admin/users`, aceptación de invitación, endpoints de
  RF-013 a RF-016 ajustados, `GET /oauth/authorize` y `POST /oauth/token`, `Role` ampliado.
- `specs/06-acceptance/*.feature`: corregir los escenarios que esperan `refreshToken` en el cuerpo
  (va en cookie desde C1 de la semana 2) y añadir los escenarios nuevos.
- Regenerar `gen.go` con `~/go/bin/oapi-codegen`; `make spec-drift` en verde.
- Hecho (2026-09-27). Codex hizo la mayor parte y se cortó por límite de uso de su cuenta
  (job task-muk3wc8y-7divbz, "usage limit") antes de reportar; Claude verificó y completó:
  - Requisitos: RF-001 (alta por admin), RF-002 (aceptar invitación), RF-003 (202 + MFA), RF-009
    (roles de directorio y de aplicación), RF-010 (no autoasignarse roles), RF-011 (eventos),
    RF-012, RF-013/014 (MFA obligatorio por correo), RF-015 (token compartido) y RF-020 nuevo
    (autorización de aplicaciones cliente). Cada uno con nota de enmienda fechada y su fuente.
  - OpenAPI: fuera `register`, `verifyEmail`, `enrollMfa`, `activateMfa`, `disableMfa`; nuevos
    `createEmployee`, `acceptInvitation`, `resendMfaCode`, `authorizeClient`, `exchangeAuthorizationCode`;
    `Role` = admin, user, contabilidad.senior, contabilidad.analista. Parámetros OAuth en snake_case
    por ser nombres de protocolo (única excepción al camelCase).
  - Escenarios: refresh por cookie corregido; nuevo `autorizacion-oauth.feature`.
  - Código: stubs `501` para las operaciones nuevas; retirado el handler HTTP de registro y su
    prueba; la lógica de `internal/auth/registration` se conserva para T5.
  - Completado por Claude: (1) Codex no regeneró `frontend/src/api/schema.d.ts` (el job de deriva
    de CI habría fallado): `make gen`. (2) Hueco de seguridad en el spec: faltaba qué hace
    `/oauth/authorize` sin sesión y quién crea la cookie SSO. Ahora: `verifyMfa` emite
    `hub_session` (`SameSite=Lax`, `Path=/oauth`) además del refresh `Strict`; sin sesión,
    `302` al login del Hub con `continue` que solo admite rutas relativas que empiecen por
    `/oauth/authorize` (sin redirección abierta); dos escenarios nuevos lo cubren.
  - Verificación (Claude): `go build`, `go vet` (con y sin tag `integration`), `make test-go`,
    `make lint` (0 issues, ESLint sin avisos), `tsc` y Vitest del frontend, `make gen` sin deriva,
    `traceability.py --check`, y `make test-integration` contra PostgreSQL 16 temporal: todo verde,
    cobertura de integración 75,1 %.
  - Estado transitorio conocido: el contrato ya exige `202` + MFA en el login, pero el login
    implementado sigue devolviendo `200` con tokens hasta T7. `backend/internal/api/verify_email.go`
    quedó sin ruta (su operación salió del contrato): T5 lo reutiliza para aceptar la invitación o
    lo borra.
  - Observaciones de GGA (hook de pre-commit) al commitear: `handleBindingError` en
    `backend/internal/api/server.go` (preexistente, no de T3) responde con `http.Error` en texto
    plano, no RFC 7807, y devuelve el texto interno del binding: se suma a T10. El campo
    `registration` y `SetRegistrationService` de `Server` quedaron sin uso: los limpia T5.
    (GGA marcó FAILED en una primera pasada por ese hallazgo preexistente y PASSED al repetirla:
    su veredicto no es determinista.)
- Commit: `9523f27`

## Fase 1 — Backend

### T4 — Roles de negocio
- [x] Estado · Ejecutor: `Sonnet` (Codex sin cuota) · Depende de: T3
- Migración y catálogo de roles; RBAC y claims `roles` del JWT con los valores nuevos; pruebas de
  que un rol desconocido se rechaza.
- Hecho (2026-09-27). Paquete `internal/auth/roles` (catálogo D8; `Directory` y `ForApplication`
  para T9), migración `000004_business_roles` (sin cambio de esquema: `roles.name` ya era texto),
  asignación por PATCH con: rol desconocido `400`, sin rol base `user` `400`, autoasignación `400`
  auditada como `role_assignment_rejected` (el escenario Gherkin fija `400`, igual que el
  autodeshabilitado), y tokens de consola (login y refresh) solo con roles de directorio.
  - Revisión de Claude antes de commitear: faltaba la regla del rol base `user` (se podía quitar) y
    la migración de bajada fallaba con roles asignados (`user_roles.role_id` es `ON DELETE
    RESTRICT`). Sonnet corrigió ambas con RED observado; Claude añadió el escenario Gherkin del rol
    base, que el contrato no tenía.
  - Revisión nativa (RDD): el primer candidato (toda la rama, 3077 líneas) excedió el presupuesto de
    contexto de los revisores (`lens_context_budget_exceeded`, por `gen.go`); se revisó el commit
    `fde38cb` solo: **aprobado**, con 10 observaciones informativas. Dos eran reales y Claude las
    corrigió con TDD en `4670436`: la autoasignación con una solicitud además inválida se rechazaba
    sin auditar (ahora la regla de autoasignación corre primero) y `Directory` aceptaba nombres sin
    punto fuera del catálogo. Esa corrección también pasó revisión nativa: **aprobada**.
  - Observaciones informativas que quedan para después: `ForApplication(roles, "")` aún no filtra
    por catálogo (nadie la llama así; T9 debe usar `Directory` o corregirla); UUID cero tratado como
    autoasignación (el handler siempre pone el actor autenticado); la bajada de la migración revoca
    asignaciones en silencio (documentado en el propio SQL).
  - Verificación: `make test-go`, `make lint` (0 issues), `go vet` con tag `integration`,
    `make test-integration` contra PostgreSQL 16 temporal (75,6 %), `traceability.py --check`.
- Commit: `fde38cb`, `4670436`

### T5 — Alta de empleados por admin e invitación por correo
- [x] Estado · Ejecutor: `Sonnet` (Codex sin cuota) · Depende de: T3, T4
- `POST /api/v1/admin/users` (solo admin, auditado), token de invitación de un solo uso con
  vencimiento, plantilla del worker, endpoint para definir la contraseña con el token.
- Hecho (2026-09-27, `28293dd`). Paquetes `internal/auth/employee` e `internal/auth/invitation`,
  handlers compuestos en `Routes()`, migración `000005` (valor `invitation` en `token_purpose`,
  reutiliza `verification_tokens`; `password_reset` ya existía para T6; la bajada reconstruye el
  enum y descarta tokens de invitación, probada con filas), consumo atómico del token con
  activación en un solo `UPDATE`, evento `user.invited` agregado a `specs/04-events/asyncapi.yaml`
  (faltaba), la aceptación reutiliza `user.email_verified`, enlace `/invitations/accept?token=…`
  (decisión de ruta para T12). `verify_email.go` y el cableado de registro/verificación salieron
  de `server.go` y `main.go`.
  - Verificación de Claude: unitarias, lint, `make gen` sin deriva; la primera corrida de
    integración falló en `TestRF002_AceptarPersisteHashArgon2idYActivaLaCuenta` y
    `TestRF006_DosRenovacionesConcurrentesUnaGana`; tres corridas completas más con `-race`
    salieron en verde: intermitentes, sumadas a T10.
  - Revisión nativa: **aprobada** con 14 observaciones informativas. A corregir (T5-fix, Codex):
    Argon2id se calcula antes de validar el token en un endpoint sin autenticación (DoS barato);
    la prueba de token vencido es vacía (el doble falla siempre); paquetes
    `internal/auth/registration` y `verification` huérfanos. Nuevo alcance: reenvío de invitación
    (D12, va en T6). Se acepta sin cambio la publicación antes del commit (compromiso de la
    ADR 0006).
  - T5-fix (2026-09-27, Codex, `8b7d5b3`): prevalidación barata del token antes de Argon2id,
    prueba de integración real de token vencido (410 y la cuenta sigue pendiente), borrado de
    `registration`/`verification` y sus queries. Unitarias, lint e integración en verde.
    Revisión nativa (alto, 16 archivos, 906 líneas, 4 lentes): **aprobada**, acusada el
    2026-09-28 (`review-5d47886d5c3f9228`). Observaciones informativas: el error de la
    prevalidación queda envuelto dos veces (`invitation.go:81`, R2) y su rama de error no tiene
    prueba (`invitation.go:79-82`, R3).
- Commit: `28293dd`, `8b7d5b3`

### T6 — RF-015 Restablecimiento de contraseña
- [x] Estado · Ejecutor: `Codex` · Depende de: T5 (reutiliza el mecanismo de token)
- Respuesta no enumerable, token de una hora, revocación de las sesiones anteriores al cambiar.
- Incluye D12: `POST /api/v1/admin/users/{userId}/invitation` (reenvío), con enmienda al spec
  (requisito, OpenAPI y escenario) antes de implementarlo.
- Hecho (2026-09-28, Codex; revisión de Claude). Paquetes `internal/auth/passwordreset` e
  `internal/auth/invitationresend`, handlers compuestos en `Routes()`. La solicitud hace siempre
  el mismo `INSERT … SELECT` y publica en ambos caminos (AM-004); el evento lleva `accountExists`
  solo por el broker y el worker no envía correo si la cuenta no existe. La confirmación valida el
  token antes de Argon2id y, en una sola sentencia, cambia la contraseña, consume el token, anula
  los demás tokens de restablecimiento del usuario y revoca todas las sesiones (AM-005). Reenvío:
  solo admin, solo `pending_verification` (si no, 409), anula invitaciones previas, token de 24 h,
  auditado. Enmiendas en requisitos, OpenAPI, AsyncAPI, amenazas y dos `.feature`.
  - Revisión de Claude antes de commitear (corregido por Codex): se enviaban correos de
    restablecimiento a direcciones no registradas (relé de spam), los tokens hermanos seguían
    vivos tras el cambio, y una prueba de integración era intermitente (hash de refresh token
    derivado del byte bajo de `UnixNano`, colisionaba en macOS).
  - Verificación de Claude: unitarias, lint 0, integración completa dos veces en verde (75,7%),
    paquetes nuevos con `-count=10` en verde.
  - Revisión nativa (alto, 34 archivos, 1641 líneas, 4 lentes): **aprobada** y acusada
    (`review-c911d7b3f0b34000`) con 14 observaciones informativas. Se acepta sin cambio la
    publicación antes del commit en ambos flujos (mismo compromiso de la ADR 0006 que en T5).
    A corregir (T6-fix): matriz de trazabilidad con RF-015 aún "diferido"; rama de saludo
    personalizado muerta en la plantilla de restablecimiento; nombre y mensaje engañosos de la
    prueba de "sin estado de cuenta"; rama muerta de `ErrTokenInvalid` al consumir; renombrar
    `ErrInvalidInput` a algo como `ErrTokenRequired`; límites de contraseña como constante
    compartida con la invitación; log al descartar eventos sin `accountExists`; pruebas de los
    códigos HTTP sin cubrir (reenvío 204/404/503/500, confirmación 204/400/500).
  - T6-fix (2026-09-28, Sonnet por falta de cuota de Codex, `1ceed6e`): D13 y las observaciones
    anteriores. Verificación de Claude: unitarias, lint 0, trazabilidad al día, integración de
    `passwordreset`, `login` y `store` con `-race` en verde. Revisión nativa (alto, 21 archivos,
    579 líneas, 4 lentes): **aprobada** y acusada (`review-c986c308fd926a89`) con 5 observaciones
    menores que van con T7: `previous_user` sin `FOR UPDATE` puede dar `unlocked` falso si un login
    concurrente bloquea la cuenta en medio (R4); la acción `password_reset_completed` repetida como
    literal en Go y SQL sin constante ni comentario que las una (R2); el detalle "12 to 128" sigue
    escrito a mano junto a las constantes (R2); las pruebas de integración no leen la metadata
    `unlocked` ni el registro de auditoría en la cuenta deshabilitada (R3).
- Commit: `7d8b2a6`, `1ceed6e`

### T7 — RF-013/014 MFA obligatorio por correo
- [ ] Estado · Ejecutor: `Codex` · Depende de: T3
- Política D11: todo login exige el código. Desafío `202` con `mfa_token` temporal, código de 6
  dígitos enviado por el worker, vencimiento corto, un solo uso, límite de intentos y auditoría; sin
  paso de activación. Quitar el rechazo explícito de cuentas con MFA de la semana 2.
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
- Integración intermitente: `TestRF002_AceptarPersisteHashArgon2idYActivaLaCuenta` y
  `TestRF006_DosRenovacionesConcurrentesUnaGana` fallaron una vez en T5 (pasaron en tres corridas
  más); investigar junto con la de `TestRF001_UsersAceptaArgon2idYRechazaMD5`.
- `handleBindingError` responde en texto plano y no en RFC 7807 (hallado por GGA en T3);
  `POST /api/v1/auth/login` con `{}` responde 500 en vez de 400; `/readyz` devuelve
  `err.Error()` por dependencia (puede filtrar host/usuario/base); investigar la intermitencia de
  `TestRF001_UsersAceptaArgon2idYRechazaMD5`.
- Commit: —

## Fase 2 — Dominios locales y frontend

### T11 — Dos dominios locales y portabilidad
- [ ] Estado · Ejecutor: `Claude` (Docker y navegadores; Codex no puede) · Depende de: D10
- Primero, la comprobación de D10 con Playwright en Chromium y Firefox: `*.localhost` resuelve sin
  `/etc/hosts` y una cookie `Secure` por HTTP se guarda y se envía.
- Dos `server` en Nginx (`identityhub.localhost` y `contabilidad.localhost`), cabeceras RNF-009 en
  ambos, CSP de cada uno acotada a su propio origen y al Hub donde haga falta.
- Brecha conocida de portabilidad, anotada para la semana 4: hoy un clon limpio no levanta con
  `docker compose up -d` porque el compose vive en `deploy/` (no en la raíz) y exige un `.env` que no
  se versiona (secretos, VULN-001/003). La solución (compose en la raíz y generación de `.env` o de
  secretos de desarrollo sin reabrir esos VULN) se decide en la semana 4.
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
| 0 — Enmiendas de spec y decisiones | T1 a T3 (3) | 3 (T1 a T3) |
| 1 — Backend | T4 a T10 (7) | 3 (T4 a T6) |
| 2 — Dominios locales y frontend | T11 a T13 (3) | 0 |
| 3 — Verificación, DAST y cierre | T14 a T17 (4) | 0 |
| **Total** | **17** | **6** |

## Siguiente paso

T7 (Codex, desde las 12:38): MFA por correo, más las 5 observaciones menores de T6-fix.

## Cambios de spec propuestos

Todos en T3, tras T2.

## Notas de handoff Codex

(vacío)
