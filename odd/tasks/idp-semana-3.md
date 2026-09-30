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
- **D14 · Aviso de restablecimiento (2026-09-28).** Todo restablecimiento completado envía un
  correo al titular avisando que su contraseña cambió (y que se cerraron sus sesiones); si además
  desbloqueó la cuenta (D13), el correo lo dice. Se envía siempre, no solo al desbloquear: si
  alguien tomó el control del buzón y cambió la contraseña, el dueño se entera. Va en T7.
- **D15 · Parámetros del desafío MFA (2026-09-28).** El código (y su `mfa_token`) vence a los
  **5 minutos**, admite **5 intentos** por desafío y el reenvío se permite cada **60 segundos**.
  RF-014 y la ADR 0010 pedían vida corta y límites sin fijar números; NIST SP 800-63B admite hasta
  10 minutos para códigos por canal externo. Consultado por Codex al empezar T7.
- **D16 · Publicar después del commit en restablecimiento y MFA (2026-09-28).** El aviso de D14 y el
  código MFA se publican después de confirmar la transacción. Si falla el aviso de D14, el
  restablecimiento se mantiene y el fallo queda en el log con el id del usuario; si falla el código
  MFA, el login responde 503 y el desafío vence solo. Evita que una caída del broker impida
  recuperar cuentas (incluido el desbloqueo de D13), que un broker lento retenga conexiones y que se
  envíe un aviso de un cambio revertido. Costo aceptado: un aviso puede perderse. Enmienda la
  ADR 0006 para estos dos flujos; el outbox transaccional se descarta por tamaño (mejora posible).

## Preguntas abiertas

Ninguna: P1 a P4 resueltas en D8 a D11; D12 y D13 salieron de revisiones; D14, del usuario al cerrar T6; D15, consulta de Codex en T7; D16, de la revisión nativa de T7.

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
- [x] Estado · Ejecutor: `Codex` · Depende de: T3
- Política D11: todo login exige el código. Desafío `202` con `mfa_token` temporal, código de 6
  dígitos enviado por el worker, vencimiento corto, un solo uso, límite de intentos y auditoría; sin
  paso de activación. Quitar el rechazo explícito de cuentas con MFA de la semana 2.
- D15: 5 min de vigencia, 5 intentos, reenvío cada 60 s.
- D14: correo de aviso en todo restablecimiento completado, con línea condicional si desbloqueó.
- Observaciones menores de T6-fix (ver T6): `FOR UPDATE` en `previous_user`; constante o comentario
  que una `password_reset_completed` en Go y SQL; "12 to 128" derivado de las constantes; pruebas
  de integración que lean la metadata `unlocked` y la auditoría en la cuenta deshabilitada.
- Handoff Codex (2026-09-28, sin commit): MFA obligatorio completo: desafío 202 sin cookie,
  token/código SHA-256, comparación constante, consumo único, expiración 5 min, 5 intentos y
  reenvío cada 60 s; `verifyMfa` emite el par normal y la cookie refresh. La verificación lleva
  la IP resuelta por el middleware `ClientIP` y el `User-Agent` a `CreateRefreshToken`; no lee
  cabeceras de reenvío directamente. Añadidos migración `000006`, SQLc, eventos/correos MFA y de
  restablecimiento completado (D14), `FOR UPDATE` de `previous_user`, detalle de contraseña
  derivado de constantes y aserciones PostgreSQL para auditorías `unlocked=true/false` de D13.
  RED observado: `GOCACHE=/tmp/identity-hub-go-build go test ./internal/auth/mfa -run
  TestRF014_VerificacionConservaIPConfiableYAgenteEnLaSesionRefresh -count=1` falló al no existir
  `VerifyInput` ni los campos IP/User-Agent de la sesión. GREEN: el mismo escenario y
  `go test ./internal/auth/mfa ./internal/api -count=1` pasaron. `make test-go`, `make lint`,
  `go vet -tags=integration ./...` y trazabilidad pasaron. No se ejecutó PostgreSQL/Docker:
  `mfa_integration_test.go` está presente y `git check-ignore` confirmó que no está ignorado.
  Codex se quedó sin cuota al cerrar (reanuda 19:23); consultó D15 antes de empezar.
- Revisión de Claude (corregido por Sonnet, delegación directa: 2+ archivos no triviales): el
  código MFA se podía adivinar por fuerza bruta, porque el bloqueo de RF-017 solo se evaluaba al
  fallar la contraseña y con la contraseña correcta se pedían desafíos sin límite (5 intentos cada
  uno); el límite por IP no contaba códigos rechazados y el intento que agotaba el desafío no
  contaba. Ahora cada código rechazado cuenta, la verificación bloquea al umbral con la misma
  política (paquete `lockout` compartido) y el límite por IP los incluye. Además: código guardado
  como HMAC-SHA256 con el token crudo como clave; `login_succeeded` y `last_login_at` al aceptar
  el código, no al validar la contraseña; fuera la rama muerta del login y `ErrMFAUnavailable`;
  código sin sesgo (`crypto/rand.Int`); `token.AccessTokenExpiresIn` compartido; reenvío rechaza
  cuentas no activas; nombre en el desafío; comentarios cruzados Go/`audit.sql`. Las pruebas de
  integración de login, logout, refresh y auditlog fallaban (construían `login.New` sin MFA):
  pasan a iniciar sesión por el flujo real con el helper `testsession`.
  - Claude (inline, TDD): la cookie refresh de `verifyMfa` perdía `MaxAge` (el login anterior lo
    ponía); RED/GREEN en `TestRF014_VerificarMFAEntregaTokensYCookieRefresh`.
  - GGA (pre-commit) bloqueó el commit: `Verify`/`Resend` convertían todo error del store en
    `ErrChallengeInvalid` (una caída de la base daba 401). Claude (inline, TDD): el store traduce
    solo `pgx.ErrNoRows`; el resto se envuelve y da 500. RED/GREEN en
    `TestRF014_FalloDeAlmacenamientoNoSeConfundeConDesafioInvalido`.
  - Verificación de Claude: unitarias, lint 0, `go vet -tags=integration`, integración completa con
    PostgreSQL real en verde (75,6%); `mfa` y `passwordreset` con `-race -count=3` y `login` con `-race -count=2` en verde (`login`
    tarda ~5 min por corrida con `-race`; con `-count=3` supera el timeout por defecto de 10 min).
    Ojo: sin `TEST_DATABASE_URL` las pruebas de base se saltan y `make test-integration` sale 0
    (56,4%); una primera verificación de Claude cayó en eso.
  - Quedan para después (fuera de alcance, sin id): `login.Service` aún recibe token/TTL y su
    `Writer` expone métodos que ya no usa; `RefreshToken` y `MFAEnabled` sin uso en `login`;
    `accountLockedNotification` en `cmd/api` duplica `events.AccountLocked`; `POST /auth/refresh`
    reemite la cookie sin `MaxAge` (previo a T7); carrera `duplicate key pg_authid_rolname_index` al
    crear roles en la migración 000002 con paquetes en paralelo (vista una vez por Sonnet).
  - Revisión nativa (alto, 44 archivos, 2405 líneas con código generado, 4 lentes): **aprobada** y
    acusada (`review-4f42750c40a59712`) con 16 observaciones informativas. Las que valen una
    T7-fix: `Issue` no tiene límite (con la contraseña correcta se crean desafíos y correos sin
    tope, se saltea el reenvío de 60 s y `mfa_challenges` crece sin purga); el aviso de D14 y el
    código MFA se publican dentro de la transacción (con el broker caído falla todo
    restablecimiento, incluido el desbloqueo de D13; con el broker lento se agota el pool; si el
    commit falla, el correo ya salió); si falla publicar `security.account_locked` desde `Verify`
    no se registra en el log; se perdió la prueba de que el access token solo lleva roles de
    directorio (D8, RF-009); faltan pruebas de vencimiento del desafío y de la ventana de reenvío en
    el servicio (además `last_sent_at` usa el reloj de la base y la comparación el del servicio);
    `CreateChallenge` del store ignora `AttemptsLeft`. Menores: dependencias muertas en
    `login.Service`, `refreshTTL` compartido entre `SetLoginService` y `SetMFAService`, largo del
    código y `"active"` como literales, nombre genérico de `WithinTransaction` en el store MFA,
    `0` sin nombre en `refreshCookie`.
- Commit: `1bacc20`

### T7-fix — Observaciones de la revisión nativa de T7
- [x] Estado · Ejecutor: `Sonnet` (Codex sin cuota hasta las 19:23) · Depende de: T7
- D16: aviso de D14 y código MFA publicados después del commit; enmienda a la ADR 0006.
- Límite de emisión: cada login nuevo anula los desafíos abiertos de la cuenta y hay un tope de 5
  desafíos por cuenta cada 15 min (429). La purga de desafíos vencidos queda para después.
- Log al fallar la publicación de `security.account_locked` desde `Verify`; `CreateChallenge` del
  store respeta `AttemptsLeft`.
- Pruebas: roles de directorio en el access token (D8, RF-009), vencimiento del desafío y ventana
  de reenvío en el servicio (misma fuente de reloj para `last_sent_at` y la comparación).
- Menores: dependencias muertas en `login.Service`, `refreshTTL` compartido entre setters, largo
  del código y `"active"` como constantes, nombre del `WithinTransaction` del store MFA, `0` con
  nombre en `refreshCookie`.
- Hecho por Sonnet (delegación directa, 2+ archivos no triviales), revisado por Claude. Ventana de
  emisión con constantes propias (`IssuanceWindow`, `MaxIssuancesPerWindow`), no la del bloqueo,
  que es configurable y pensada para credenciales erróneas; un `pg_advisory_xact_lock` por usuario
  evita superar el tope con logins concurrentes. `last_sent_at` y `created_at` salen del reloj del
  servicio. `login.New` ya no recibe el servicio de tokens ni el TTL. Las pruebas de roles de
  directorio y de vencimiento cubren comportamiento existente: se probaron mutando `mfa.go`.
  `make gen` regeneró `frontend/src/api/schema.d.ts`, que estaba desactualizado.
  - Verificación de Claude: unitarias, lint 0, `go vet -tags=integration`, trazabilidad al día e
    integración completa con PostgreSQL real en verde (76,4%).
  - Quedan anotados: el 6.º desafío rechazado no se audita (el callback se revierte, solo da 429);
    si un reenvío confirma pero falla la publicación, el código anterior ya no sirve y hay que
    esperar 60 s para reenviar; con la contraseña, alguien puede agotar el tope de 5 desafíos y
    dejar al titular 15 min sin entrar (mismo compromiso que el bloqueo de RF-017).
  - Revisión nativa (alto, 28 archivos, 1083 líneas, 4 lentes): **aprobada** y acusada
    (`review-97ada3d05cfd27fc`) con 8 observaciones informativas. Para una tarea futura: los
    desafíos cuyo código no se pudo publicar cuentan para el tope, así que 5 reintentos durante una
    caída del broker dejan al usuario 15 min en 429 (no contar o anular esos desafíos); en un
    reenvío que falla al publicar, no adelantar `last_sent_at` para permitir reintentar ya, y
    documentarlo en la ADR y OpenAPI; ninguna prueba ejercita el `login.Service` real con un emisor
    que devuelva `ErrIssuanceLimited`/`ErrDeliveryUnavailable` (solo stubs del handler). Menores:
    `SupersedeOpenMfaChallenges` usa el `now()` de la base; el comentario de la prueba de
    integración de desafío anulado promete más de lo que prueba.
- Commit: `4b63728`

### T8 — RF-016 Sesiones activas
- [x] Estado · Ejecutor: `Codex` · Depende de: T3
- Listar sesiones (IP, user-agent, creación, último uso) y revocar una concreta.
- Con T8 (decisión del usuario, 2026-09-28), observaciones de la revisión nativa de T7-fix: un
  desafío cuyo código no se pudo publicar no cuenta para el tope de emisión (anularlo o
  excluirlo); un reenvío que falla al publicar no adelanta `last_sent_at` (documentarlo en la
  ADR 0006 y OpenAPI); prueba con el `login.Service` real y un emisor que devuelva
  `ErrIssuanceLimited`/`ErrDeliveryUnavailable` hasta el 429/503; `SupersedeOpenMfaChallenges` con el
  reloj del servicio; comentario de la prueba de desafío anulado ajustado a lo que prueba.
- Verificación de Claude (2026-09-28): revisión del diff (listado y revocación filtran por
  `user_id`; ajena, vencida o revocada dan el mismo 404; revocación y auditoría en una transacción;
  sesión actual por claim firmado `sid`), unitarias, lint 0, trazabilidad al día e integración
  completa con PostgreSQL real en verde (74,3%). Queda anotado: el access token de una sesión
  revocada sigue valiendo hasta su vencimiento (15 min, JWT sin estado); `make gen` usa
  openapi-typescript 7.13.0 frente al 6.7.6 que documenta `AGENTS.md`.
- Revisión nativa (alto, 24 archivos, 922 líneas, 4 lentes): **aprobada** y acusada
  (`review-5dbfa7da1c845f4b`) con 12 observaciones informativas. Para una T8-fix: la compensación
  tras un fallo de publicación (borrar el desafío o restaurar el reenvío) corre con el mismo
  contexto de la petición, así que si el fallo fue por cancelación o timeout la compensación
  también falla (usar un contexto desacoplado con plazo propio); `RestoreResend` descarta
  `RowsAffected` y no deja rastro si no restauró nada; faltan pruebas HTTP de 404/500/503 en
  sesiones y de integración de la compensación en PostgreSQL; comentario de `CreateMfaChallenge`
  con los `$n` corridos. Menores: parámetros sin nombre en `RestoreResend`, guardas duplicadas en
  los handlers de sesiones, parámetros sin uso en `testSessionToken`, y si el broker acepta pero
  responde error, el correo sale sin cobrar el tope.
- Commit: `f537243`

### T8-fix — Observaciones de la revisión nativa de T8
- [ ] Estado · Ejecutor: `Codex` (GPT-5.6-Sol, esfuerzo medium; GPT-6.1-Sol no está disponible con la cuenta de ChatGPT) · Depende de: T8
- Compensación tras fallo de publicación del código MFA (borrar el desafío o restaurar el reenvío)
  con un contexto desacoplado de la petición (`context.WithoutCancel` + plazo propio), para que
  una cancelación o timeout de la petición no deje el desafío contando para el tope de 5.
- `RestoreResend` revisa `RowsAffected` y deja rastro (log) si no restauró nada; parámetros con nombre.
- Pruebas: HTTP de 404/500/503 en listado y revocación de sesiones; unitaria de compensación con
  contexto cancelado; integración en PostgreSQL de la compensación (borrado y restauración).
- Menores: comentario de `CreateMfaChallenge` con los `$n` correctos, guardas duplicadas en los
  handlers de sesiones, parámetros sin uso en `testSessionToken`. Si el broker acepta pero
  responde error: documentar el comportamiento (no se cobra el tope) o decidirlo, sin cambiarlo en silencio.
- Ruta: delegado a Codex (2+ archivos no triviales). Codex no tiene Docker: la integración con
  PostgreSQL real la corre Claude al revisar.
- Hecho por Codex (GPT-5.6-Sol medium; GPT-6.1-Sol falló al arrancar: la cuenta de ChatGPT no
  lo admite). La compensación de `Issue` y `Resend` corre con `context.WithoutCancel` y un plazo
  propio de 5 s, y registra el fallo si la compensación falla. RED observado:
  `TestRF014_CompensacionDeEmisionSobreviveContextoCancelado` falló con `context canceled` antes
  del cambio. Caso del broker que acepta pero responde error: sin cambios (el correo puede salir y
  la compensación no cobra el tope); queda como riesgo aceptado, a decidir por el usuario.
- Verificación de Claude (2026-09-29): revisión del diff; `sqlc generate` (v1.31.1) para llevar
  el comentario `$7` al código generado; corregida una falta de ortografía que marcó el linter;
  `golangci-lint` 0; unitarias en verde; integración con PostgreSQL real 75,9%, con las pruebas
  nuevas de compensación (eliminación y restauración) en verde. Fallaron 3 pruebas de otros
  paquetes (`audit`, `employee`, `invitation`) por la carrera ya anotada en T10; pasan en serie (`-p 1`).
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
  Causa hallada en T8-fix (2026-09-29): la migración `000002` crea el rol `identity_app` con
  "comprobar y después crear" (`IF NOT EXISTS ... CREATE ROLE`). Los roles son globales del
  clúster y los paquetes de integración corren en paralelo sobre el mismo servidor, así que dos
  paquetes crean el rol a la vez y uno falla con `23505 pg_authid_rolname_index`. Se reprodujo en
  `audit`, `employee` e `invitation`; en serie (`-p 1`) pasan.
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
| 1 — Backend | T4 a T10 (7) | 5 (T4 a T8) |
| 2 — Dominios locales y frontend | T11 a T13 (3) | 0 |
| 3 — Verificación, DAST y cierre | T14 a T17 (4) | 0 |
| **Total** | **17** | **8** |

## Siguiente paso

T8-fix (en curso, Codex GPT-5.6-Sol medium); luego T9 (authorization code + PKCE, Codex con revisión reforzada).

## Cambios de spec propuestos

Todos en T3, tras T2.

## Notas de handoff Codex

- T8: se implementaron `GET/DELETE /api/v1/me/sessions` sobre familias de refresh, con propiedad del titular, `404` indistinguible para familia ajena/desconocida, auditoría `session_revoked` y marca `current` mediante el `sid` firmado del access token; se añadieron servicio, adaptador SQLc y composición.
- T7-fix: un fallo al publicar el desafío inicial lo anula para no agotar el cupo; un fallo al publicar un reenvío restaura código y `last_sent_at`; la superación usa el reloj del servicio. ADR 0006 y OpenAPI documentan ambos casos; la prueba real de `login.Service` confirma 429/503.
- RED observado: `cd backend && GOCACHE=/tmp/identity-hub-go-build go test ./internal/auth/session ./internal/auth/mfa ./internal/api -run 'TestRF016_|TestRF014_FalloDeEntrega' -count=1` falló por el paquete de sesiones inexistente, desafío no anulado y `last_sent_at` adelantado. GREEN: la misma selección y `go test -race ./internal/auth/session ./internal/auth/mfa ./internal/auth/refresh ./internal/api -count=1` pasaron.
- Verificación: `make gen`, `make test-go`, `make lint`, `go vet -tags=integration ./...`, validación OpenAPI y trazabilidad en verde; T8 corrige además el generador para que RF-016 ya no figure diferido. Claude debe ejecutar `make test-integration` con PostgreSQL real; el sandbox no tiene `TEST_DATABASE_URL`.
