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
- **D17 · Primer admin por invitación al arrancar (2026-10-01, usuario).** Sin autorregistro (T5)
  ni semilla, un sistema recién levantado no tiene quien entre a la consola (lo detectó la
  verificación de T12a, que tuvo que crear cuentas a mano en la base local). Variable opcional
  `BOOTSTRAP_ADMIN_EMAIL`: al arrancar, si no existe ningún admin, la API crea esa cuenta como
  pendiente con rol `admin` y le envía la invitación por el mismo camino de T5 (llega a Mailpit). Sin
  contraseñas en variables ni en el repo (RNF-003); si ya hay un admin, no hace nada y lo registra
  en el log. Se descartaron un comando manual (paso extra en la demo y en CI) y una semilla con
  cuentas demo (credenciales conocidas). Lo implementa T12d.

## Preguntas abiertas

Ninguna: P1 a P4 resueltas en D8 a D11; D12 y D13 salieron de revisiones; D14, del usuario al cerrar T6; D15, consulta de Codex en T7; D16, de la revisión nativa de T7; D17, del usuario al verificar T12a.

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
- [x] Estado · Ejecutor: `Codex` (GPT-5.6-Sol, esfuerzo medium; GPT-6.1-Sol no está disponible con la cuenta de ChatGPT) · Depende de: T8
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
- Revisión nativa (alto, 10 archivos, 297 líneas, 4 lentes): **aprobada** y acusada
  (`review-311ed4df12551d88`) con 6 observaciones informativas, pasadas a T10.
- Commit: `06e5f7d`

### T9 — Autorización para aplicaciones cliente (authorization code + PKCE)
- [x] Estado · Ejecutor: `Codex` (GPT-5.6-Sol medium, con revisión reforzada de Claude) · Depende de: T3, T7
- Sesión del Hub en su dominio; `GET /oauth/authorize` (valida cliente, `redirect_uri` exacto,
  `state`, `code_challenge` S256; si no hay sesión, lleva al login del Hub con MFA) y
  `POST /oauth/token` (canjea código + `code_verifier` por access token; código de un solo uso).
  Cliente Contabilidad fijado en configuración. Una prueba por cada control de "Restricciones".
- Hecho por Codex (GPT-5.6-Sol medium): migración `000007` (`applications`, `hub_sessions`,
  `authorization_codes`, solo hashes), paquete `internal/auth/oauth`, handlers en `api/oauth.go`
  compuestos en `Server.Routes()` (sin los stubs `501`), cookie `hub_session` desde `verifyMfa`,
  `IssueForAudience` (`aud` = cliente, solo roles de la aplicación, sin `sid`) y CORS limitado a
  `/oauth/token` y el JWKS. Canje en una transacción: bloquear, validar cliente, `redirect_uri` y
  PKCE, consumir, confirmar (un verificador incorrecto no consume el código). TDD parcial según el
  propio Codex: algunas correcciones de su verificador interno se hicieron antes que sus pruebas.
- Revisión reforzada de Claude, cuatro hallazgos corregidos por Sonnet (Codex sin cuota hasta las
  23:53) con TDD y RED observado: (1) el logout y la confirmación del restablecimiento no revocaban
  `hub_sessions`, así que el SSO seguía emitiendo códigos 30 días tras cerrar sesión, contra la
  ADR 0009; ahora se revocan en la misma transacción y el logout borra la cookie; (2) un error de
  base de datos en el canje daba 400 y cualquier código inválido dejaba una auditoría
  `authorization_code_reused` sin actor (inundable sin autenticarse); ahora solo el reuso real se
  audita, con el dueño como actor, y los fallos de infraestructura dan 500; (3) `Cache-Control:
  no-store` en `/oauth/token` (RFC 6749 §5.1); (4) `Vary: Origin` siempre en las rutas con CORS.
- Verificación de Claude (2026-09-29): `golangci-lint` 0, unitarias en verde, `sqlc generate` sin
  deriva, trazabilidad al día, integración completa con PostgreSQL real 78,5% sin pruebas omitidas
  (`-p 1` por la carrera de roles anotada en T10).
- Tras la nota de GGA en el pre-commit: el `redirect_uri` del cliente se valida una vez en
  `SetOAuthService` (si no es absoluto, OAuth queda en 503) en vez de ignorar el error de
  `url.Parse` en cada petición, y `main.go` arma el cliente una sola vez.
  GGA detectó que ese cambio abría un `nil` en el camino de error de enlace de `/oauth/authorize`
  cuando OAuth no está configurado (500 por pánico recuperado); corregido con prueba primero
  (`TestRF020_AutorizacionSinOAuthConfiguradoNoRedirigeNiFalla`, RED 500, GREEN).
- Para T11: `hub_session` es `Secure` y el Hub va por HTTP en `*.localhost` (D10); comprobar en
  Chromium y Firefox que el navegador la guarda. El cliente queda fijado dos veces (constantes de
  configuración y fila sembrada en la migración) y deben coincidir.
- Revisión nativa (alto, 30 archivos, 1.721 líneas, 4 lentes): **aprobada** y acusada
  (`review-c3eb980b2c81235c`) con 15 observaciones informativas, pasadas a T9-fix.
- Commit: `9c3b070`

### T9-fix — Observaciones de la revisión nativa de T9
- [x] Estado · Ejecutor: `Codex` (implementa) + Claude (integración con PostgreSQL real, lint, revisión) · Depende de: T9
- Decisión (2026-09-30, con el usuario): el bloqueo de cuenta **no** revoca `hub_sessions` (ya lo
  cubre `GetHubSessionUser` con `u.status = 'active'`; revocar daría a un atacante un modo de
  cerrar el Hub de la víctima con intentos fallidos). RF-016 y la detección de reuso de refresh
  revocan **solo** la sesión del Hub de su familia: migración `000008` agrega `family_id` a
  `hub_sessions` (se crea junto a la familia en `mfa.go`). Logout y reseteo de contraseña siguen
  revocando todas las del usuario.
- Riesgo: revisar qué otros caminos que cortan sesiones (revocación de RF-016, reuso de refresh,
  bloqueo de cuenta) deben revocar también `hub_sessions` (hallazgo R1 en `mfa.go`, creación de la
  sesión del Hub).
- Fiabilidad y resiliencia: un error de base de datos al leer la sesión del Hub se trata como "sin
  sesión" y redirige al login en vez de dar 5xx; el canje consume el código antes de leer roles y
  auditar, así que un fallo posterior deja el código gastado sin token; los 500 de OAuth no se
  registran en el log; las tablas `authorization_codes` y `hub_sessions` crecen sin purga.
- Legibilidad: cliente duplicado en constantes y en la fila sembrada (un desfase da 500), validación
  de autorización duplicada entre handler y servicio, rama muerta al armar `continue`, centinela
  engañoso en `GetHubSessionUser` (usa `ErrAuthorizationCodeInvalid`), cotas de PKCE sin nombre,
  estado de RF-020 en la matriz de trazabilidad por revisar; prueba unitaria de MFA de la sesión Hub.
- Resultado (2026-09-30): migración `000008_hub_session_refresh_family` (`family_id` + índice
  parcial); `RevokeActiveRefreshSession` y `RevokeRefreshFamily` revocan en la misma sentencia la
  sesión del Hub de esa familia; `ErrHubSessionInvalid` propio y error de base → 500 RFC 7807;
  canje, lectura de roles y auditoría en una sola transacción (`WithinAuthorizationCodeTransaction`);
  `writeOAuthServerError` registra los 500 sin códigos ni tokens; `ValidateOAuthClient` hace fallar
  el arranque si el cliente configurado no coincide con la fila sembrada; validación de autorización
  única en `oauth.Service`; constantes PKCE con nombre; rama muerta de `continue` eliminada; RF-020
  pasa a completo en la matriz. Purga: consultas `PurgeAuthorizationCodes`/`PurgeHubSessions` y
  métodos del store con prueba de integración; **no se cablearon** porque no existe un mecanismo
  periódico (queda pendiente decidir dónde correrlas).
- Ejecución: el job de Codex murió a mitad (se cortó al llegar el reenviador a su límite de 30 min)
  y no entregó informe final; el código lo dejó completo, pero **no hay evidencia RED registrada**
  de TDD. Claude verificó: `go build`, `go vet` (con y sin tag), `go test ./...`, integración real
  `go test -race -tags=integration -p 1 -count=1 ./...` contra PostgreSQL desechable (todo `ok`,
  incluidas `TestRF017_CuentaBloqueadaNoAutorizaConSesionHubExistente`,
  `TestRF020_PostgresPurgaCodigosYSesionesHubInutilizables`,
  `TestRF016_ListarYRevocarUnaFamiliaSinAfectarOtra` y `TestRF006_ReusoDeTokenRotadoRevocaLaFamilia`),
  `make gen` sin diferencias, `golangci-lint` 0 issues.
- Residual anotado: la firma del access token ocurre después del commit del canje; si fallara, el
  código queda gastado (firma en memoria, fallo improbable). Las sesiones del Hub previas a `000008`
  tienen `family_id` nulo y solo las revocan logout y reseteo.
- Fuera de alcance: `golangci-lint --build-tags=integration` marca `errcheck` en
  `internal/testdb/testdb.go:52` (`dropDatabase` sin comprobar), previo a esta tarea (`6954cba`).
- Revisión nativa: evaluación **alta** (`review_due: high_risk`, 21 rutas). **No disponible**: el
  envío de la selección de archivos sin seguimiento pierde `--base-ref`/`--committed-only` y el
  START ofrecido cubría toda la rama (137 rutas). No se ejecutó START. Defecto de Gentle AI 3.7.0
  ya reportado (#4890, abierto, sin arreglo publicado); con consentimiento del usuario se agregó un
  comentario de ocurrencia. El commit queda sin revisión nativa; se sigue con la política normal.
- Commit: `d1c566d`

### T10 — Pendientes chicos de la semana 2
- [x] Estado · Ejecutor: `Codex` (implementa) + Claude (integración con PostgreSQL real, lint, revisión)
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
- Resultado (2026-09-30): `testdb.New` toma un advisory lock de sesión
  (`migrationAdvisoryLockID`) en una conexión de admin durante la creación y migración de la base
  temporal, sin tocar la migración `000002`; `handleBindingError` responde RFC 7807 400 sin
  detalle interno; `POST /api/v1/auth/login` con `{}` da 400 RFC 7807 antes de mirar si el
  servicio está disponible; `/readyz` solo expone `up`/`down` y registra el detalle con el logger
  inyectado; `Store.WithLogger` inyecta el logger en `mfaWriter` (`RestoreResend` ya no usa el
  `slog` global); pruebas del caso sin fila y del fallo de la compensación; comentario del plazo de
  5 s. Los parámetros de `RestoreResend` ya tenían nombre.
- TDD: RED/GREEN de Codex para binding, login, readiness y compensación. RED de la carrera
  observado por Claude con el `testdb.go` anterior sobre un clúster nuevo
  (`23505 pg_authid_rolname_index`); GREEN en tres clústeres nuevos. La carrera solo se reproduce
  mientras el rol `identity_app` no existe todavía en el clúster.
- Verificación (Claude): suite de integración completa **en paralelo** (sin `-p 1`) con
  `-race` sobre un clúster nuevo, todo `ok`; `go test -race ./...` `ok`; `golangci-lint` 0 issues
  (con tag `integration` sigue solo el `errcheck` previo de `testdb.go`, ahora línea 76).
- Revisión nativa: evaluación **alta** (16 rutas, 252 líneas). **No disponible** por el mismo
  defecto de Gentle AI 3.7.0 (#4890: la selección de archivos sin seguimiento pierde
  `--base-ref`). El usuario eligió continuar sin reportar otra vez; no se ejecutó START.
- Commit: `20ad92b`
- Observaciones de la revisión nativa de T8-fix: `RestoreResend` del store registra con el
  `slog` global y no con un logger inyectado, y el caso en que no restaura nada no tiene prueba;
  falta prueba del camino en que la compensación misma falla (`logCompensationFailure`); documentar
  por qué el plazo de compensación es de 5 s; nombres de parámetros de `RestoreResend` en la interfaz.

## Fase 2 — Dominios locales y frontend

### T11 — Dos dominios locales y portabilidad
- [x] Estado · Ejecutor: `Claude` (Docker y navegadores; Codex no puede) · Depende de: D10
- Primero, la comprobación de D10 con Playwright en Chromium y Firefox: `*.localhost` resuelve sin
  `/etc/hosts` y una cookie `Secure` por HTTP se guarda y se envía.
- Dos `server` en Nginx (`identityhub.localhost` y `contabilidad.localhost`), cabeceras RNF-009 en
  ambos, CSP de cada uno acotada a su propio origen y al Hub donde haga falta.
- Brecha conocida de portabilidad, anotada para la semana 4: hoy un clon limpio no levanta con
  `docker compose up -d` porque el compose vive en `deploy/` (no en la raíz) y exige un `.env` que no
  se versiona (secretos, VULN-001/003). La solución (compose en la raíz y generación de `.env` o de
  secretos de desarrollo sin reabrir esos VULN) se decide en la semana 4.
- Ruta: delegada (Sonnet; Docker, navegadores y red). Revisó y commiteó Claude.
- D10 comprobado (2026-10-01) con un script de Playwright desechable fuera del repo. Chromium
  (host): `identityhub.localhost` resuelve sin `/etc/hosts`, la cookie `Secure` por HTTP se guarda y
  se envía, y no llega a `contabilidad.localhost` (sitios distintos). Firefox: los builds de
  Playwright no arrancan en macOS 27 (perfil/sandbox), así que se probó en
  `mcr.microsoft.com/playwright:v1.63.0-noble` (Firefox 155.0, servidor y navegador en el mismo
  contenedor, decisión del usuario): las mismas cuatro comprobaciones pasan.
- Nginx: el Hub (`identityhub.localhost`) es `default_server`, así que `localhost:8080` lo sigue
  sirviendo (CI, ZAP); nuevo proxy `/oauth/` hacia la API (antes `/oauth/authorize` caía en la SPA).
  `contabilidad.localhost` sirve un marcador estático hasta T13. Las cinco cabeceras de RNF-009
  viven en `security-headers.conf` con la CSP por `server` en `$csp` (Contabilidad agrega el Hub en
  `connect-src`); el include en `/assets/` corrige además que esa `location` perdía las cabeceras.
- `PUBLIC_BASE_URL` y `JWT_ISSUER` pasan a `http://identityhub.localhost:8080` (config, compose,
  OpenAPI, README, guía y Makefile). TDD: RED `go test -race ./internal/config/` con los dos valores
  por defecto viejos; GREEN `ok`.
- Verificación (stack real con `make up`): Hub, Contabilidad y `localhost:8080` responden 200 con
  las cinco cabeceras y su CSP; ruta profunda 200; `/healthz`, `/readyz` y JWKS 200 por el Hub;
  login con credenciales inventadas 401 problem+json; `/oauth/token` 400 y `/oauth/authorize` sin
  parámetros 400. Chromium carga el Hub y el marcador. `make test-go` ok, `make test-integration`
  ok, `npm run test` ok, `golangci-lint` 0 issues, trazabilidad al día, Trivy de `web` 0
  HIGH/CRITICAL corregibles. Nada quedó corriendo.
- Pendiente: `.env.example` (líneas de `API_BASE_URL` y `JWT_ISSUER`) sigue con
  `http://localhost:8080`; los permisos del entorno bloquean editarlo. Sin efecto funcional (nadie
  lee `API_BASE_URL` y el compose fija `JWT_ISSUER`), pero el ejemplo queda desactualizado: lo
  cambia el usuario.
- Revisión nativa: el primer intento chocó con #4890 (la selección de archivos sin seguimiento
  pierde `--base-ref`/`--committed-only`). Arreglo local: los archivos sin seguimiento del usuario
  (`.atl/`, `.codegraph/`, `Claude outputs/`, `ANALISIS-REQUISITOS.md`,
  `ENUNCIADO-TRABAJO-FINAL.md`) van en `.git/info/exclude`, así Gentle AI no pide la selección.
  Con eso, revisión de `edec16f..f582156` (alto, 13 archivos, 187 líneas, 4 lentes) con
  consentimiento del usuario: **aprobada** y acusada (`review-7548bec84859a73f`), sin bloqueantes.
  Observaciones informativas: R3 (WARNING) los cambios de Nginx no tienen prueba automática
  (cabeceras en `/assets/`, proxy `/oauth/`, el `server` de Contabilidad, que CI y ZAP no alcanzan
  porque apuntan a `localhost:8080`): pasa a T14 como smoke test con `Host`; R3 el origen del Hub
  en la CSP de Contabilidad está fijado a mano y repetido en config y compose; R2 el comentario
  del `server` de Contabilidad habla de "la SPA" cuando aún es el marcador de T13; R2 el valor por
  defecto de `JWTIssuer` se comprueba dentro de la prueba de `PublicBaseURL`.
- Commit: `3cb30fc`

### T12 — Consola de Identity Hub (React)
- [x] Estado · Ejecutor: `Codex` · Depende de: T5, T7, T8, T11
- Según el mockup: login con MFA, directorio de usuarios, alta y edición (estado y roles, regla de
  auto-deshabilitado de RF-010), audit log; páginas de cuenta: verificar correo
  (`/verify-email?token=…`, Q15), definir contraseña, restablecer, perfil y sesiones. Refresh
  compartido entre pestañas (Web Locks o `BroadcastChannel`).
- Plan (Claude, 2026-10-01), tres commits de unidad de trabajo, cada uno con sus pruebas Vitest:
  - [x] T12a — Base: enrutado con `react-router-dom`, cliente de API tipado con `schema.d.ts`
    (errores RFC 7807), access token solo en memoria y refresh por la cookie `HttpOnly`; refresh
    serializado entre pestañas con Web Locks (dos pestañas que rotan a la vez dispararían la
    detección de reuso de RF-006 y revocarían la familia); login con correo y contraseña → 202 →
    pantalla de código MFA (verificar y reenviar; 401, 423, 429 y 503 con mensajes propios);
    cerrar sesión. Tras el login, un admin entra a la consola y el resto a "Mi cuenta".
    Hecho (2026-10-01, Codex; revisó, verificó y commiteó Claude). Commit `7c9c3a7`. TDD: RED
    11/11 y luego 4/16 con las correcciones de revisión, GREEN 17/17; lint, tipos y build ok. Claude
    corrigió en la revisión el mensaje de un 401 en el paso MFA (decía "correo o contraseña") y los
    prefijos de requisito de las pruebas; GGA pidió usar el tipo generado `MfaChallenge`.
    Verificación en el stack real (Sonnet, Chromium): login, código desde Mailpit, consola para el
    admin y "Mi cuenta" para el resto, sesión restaurada al recargar, dos pestañas recargando a la
    vez sin evento de reuso (12 rotaciones, ninguna revocación salvo el logout), logout, mensajes de
    código y de credenciales; sin errores de consola ni violaciones de CSP. Cuentas de prueba creadas
    a mano en la base local (de ahí D17). Revisión nativa (medio, 5 archivos, 779 líneas, una
    lente): **aprobada** y acusada (`review-758fe75ab4475b76`), con observaciones que pasan a
    T12a-fix.
  - [x] T12a-fix — Observaciones de la revisión de T12a: un 2xx sin cuerpo rompe `request()`
    (confirmado: el 202 de `mfa/resend` no tiene cuerpo, así que un reenvío correcto muestra "No se
    pudo conectar"); el `BroadcastChannel` del token se reenvía a la misma pestaña (dos instancias
    con el mismo nombre); si `mfa/verify` sale bien y falla `GET /me`, el mensaje culpa al código;
    `TestRF007_LogsOutLocallyWhenRemoteLogoutFails` está fuera del `describe` y no corre su
    `afterEach`; faltan pruebas del refresh fallido, de la sesión restaurada al montar y de la
    guarda de `/usuarios`; el reenvío correcto no muestra confirmación. Además, el menú lateral no
    marca la página activa (visto en la verificación).
    Hecho (2026-10-01, Codex; revisó y commiteó Claude). Commit `8dd3794`. RED observado en el 2xx
    sin cuerpo, el eco del canal, el fallo de `/me` tras el MFA, la confirmación del reenvío y el
    menú activo; las pruebas que faltaban pasaron sin RED (cubrían comportamiento ya correcto).
    GREEN 29/29; lint, tipos y build ok; GGA aprobó. Revisión nativa: `review_due: false`
    (`under_budget`, medio, 287 líneas): queda pendiente en el tramo hasta que un commit posterior
    alcance el presupuesto. Detalle menor anotado: si el código MFA sale bien y falla `GET /me`,
    solo se borra el token local; la sesión del servidor sigue viva y una recarga la restauraría.
  - [x] T12b — Cuenta: aceptar invitación y definir contraseña (`/invitations/accept?token=…`,
    ruta fijada en T5; sustituye la página `/verify-email` de Q15, que se fue con el autorregistro),
    solicitar y confirmar restablecimiento, perfil (`GET`/`PATCH /me`) y sesiones activas
    (listar y revocar).
    Hecho (2026-10-01, Sonnet porque Codex no tenía cuota; revisó y commiteó Claude). Commit
    `8291345`. Rutas de los correos: `/invitations/accept?token=` y `/password-reset?token=`
    (`notify.go`); solicitud en `/forgot-password`. TDD: RED 27/29, GREEN 61/61; lint, tipos y build
    ok; GGA aprobó. E2E con Chromium en `-p t12b-check`, con el primer admin llegado por T12d:
    aceptar invitación (reusarla, 410), MFA, editar el nombre y que persista, revocar la sesión de
    otro contexto (sale en su siguiente refresh), restablecer la contraseña y entrar con la nueva,
    revocar la sesión actual (cierra aquí); sin errores de consola ni de CSP. Revisión nativa
    (medio, 7 archivos, 699 líneas, una lente): **aprobada** y acusada (`review-f619b064ab69f30d`).
    Observaciones que pasan a T12c: un fallo de autenticación al revocar no saca al usuario;
    nombre vacío se envía sin validar; claves de React por texto del mensaje; faltan pruebas del
    límite de 128, del 400 sin campos y de un 500 al confirmar. Además, el user agent de las
    sesiones se ve crudo.
  - [x] T12c — Consola admin (incluye las observaciones de la revisión de T12b): directorio con búsqueda y estados, cajón de alta y edición (roles
    del catálogo; un admin no puede deshabilitarse a sí mismo, RF-010), reenvío de invitación a
    cuentas pendientes y registro de auditoría.
    Hecho (2026-10-01, Sonnet; revisó y commiteó Claude). Commit `24bf24c`. TDD: RED 12/45 (arreglos
    de T12b) y 39/41 (consola); GREEN 118/118; lint, tipos y build ok; GGA aprobó. El usuario
    propio no puede cambiarse estado ni roles (el backend también lo rechaza); `user` va siempre
    marcado (rol base obligatorio); roles con los ids del OpenAPI (`contabilidad.senior`,
    `contabilidad.analista`). E2E con Chromium en `-p t12c-check`: alta de empleada con invitación
    en Mailpit, reenvío (enlace viejo 410), la empleada entra y cae en "Mi cuenta" sin acceso a la
    consola, edición de roles, deshabilitarla (ya no entra), cajón propio bloqueado, auditoría con
    todos los eventos y filtro; sesiones con "Chrome en macOS"; sin errores de consola ni de CSP.
    Huecos del backend anotados (sin cambiar): la auditoría trae solo UUIDs (sin nombre del actor);
    mensajes de error en inglés; autodeshabilitarse, autoasignarse roles y quitar el último admin
    devuelven el mismo 400 genérico; el listado de usuarios ordena por id y no trae total.
    Revisión nativa (medio, 15 archivos, 1.358 líneas, una lente): **aprobada** y acusada
    (`review-96aa97adcdb33fa5`). Observaciones → T12c-fix.
  - [x] T12c-fix — "Cargar más" del directorio y de la auditoría no descarta respuestas viejas: si
    cambia la búsqueda o el filtro mientras carga, agrega filas de la consulta anterior y deja el
    cursor viejo; además, el `console.error` espiado en `account.test.tsx` solo se restaura si la
    prueba pasa.
    Hecho (2026-10-01, Sonnet; commiteó Claude). Commit `982240a`. Contador de generación compartido
    por la carga inicial y "Cargar más"; al cambiar la búsqueda o el filtro se limpia el cursor y el
    botón se oculta hasta la primera página nueva. RED 5/123 (tres carreras con promesas diferidas y
    dos de botón oculto), GREEN 123/123; lint, tipos y build ok; GGA aprobó. Revisión nativa:
    `review_due: false` (`under_budget`, medio, 185 líneas): queda en el tramo con el siguiente
    commit.
- Restricciones: sin dependencias nuevas (Codex no tiene red); CSP sin `unsafe-inline` ni orígenes
  externos, así que nada de estilos inline ni Google Fonts: las familias del mockup se declaran con
  pila de respaldo del sistema. Sistema visual del mockup "Identity Hub Console" (acento `#2d4f8f`,
  modo oscuro). Verificación contra el stack real (Docker y Chromium): Claude, al cerrar.
- Commit: `7c9c3a7` (T12a), `8dd3794` (T12a-fix), `8291345` (T12b), `24bf24c` (T12c), `982240a` (T12c-fix)

### T12d — Primer admin por invitación al arrancar (D17)
- [x] Estado · Ejecutor: `Codex` (Go, sin red) · Depende de: T5
- Al arrancar la API, con `BOOTSTRAP_ADMIN_EMAIL` definido y ningún admin en la base, crear la
  cuenta pendiente con rol `admin` y encolar su invitación (mismo flujo y auditoría que el alta de
  T5); idempotente y segura con varias réplicas arrancando a la vez (sin dos invitaciones ni dos
  cuentas). Variable en `config.go`, compose, `.env.example` (lo edita el usuario) y la guía.
- Pruebas: sin variable no hace nada; con variable y sin admin crea y encola una sola vez; con un
  admin existente no hace nada; arranques concurrentes (integración con PostgreSQL real).
- Ruta: delegada. Codex escribió casi todo y se quedó sin cuota al final; Sonnet terminó, verificó
  y Claude revisó y commiteó. Se borró el archivo de tareas aparte que había creado Codex.
- Hallazgo de la revisión de Claude: contar cualquier admin (también uno pendiente) trababa la
  instalación si el primero no aceptaba la invitación en 24 h. Ahora: admin activo → no hace nada;
  el correo configurado es un admin pendiente con invitación vigente → no hace nada; con la
  invitación vencida o anulada → la reemite como el reenvío de T6 (anula la anterior, auditoría
  `bootstrap_admin_invitation_reissued`); el correo es de otra cuenta → aviso; si no, la crea. La
  guarda de invitación vigente salió de la prueba concurrente (8 arranques mandaban 8
  invitaciones).
- TDD: RED en reemisión, error del broker en la reemisión e invitación vigente; GREEN. Verificación:
  `make test-go` ok, `make test-integration` ok (arranque concurrente: una cuenta, una invitación,
  un evento), `golangci-lint` 0 issues (con tag `integration` sigue solo el `errcheck` previo de
  `testdb.go`), trazabilidad al día, `sqlc` sin deriva. E2E en un proyecto de compose aparte
  (`-p t12d-check`, sin tocar el volumen del usuario): log de creación sin el correo, invitación en
  Mailpit, aceptación 204 (segunda vez 410), login + MFA y `/me` con `admin`; reinicio sin efecto;
  invitación vencida a mano → reemitida, token viejo 410 y nuevo 204.
- Pendiente del usuario: agregar `BOOTSTRAP_ADMIN_EMAIL` a `.env.example`.
- Revisión nativa de T12a-fix + T12d (`7c9c3a7..6ddfea8`, alto, 18 archivos, 1.311 líneas, 4
  lentes): **aprobada** y acusada (`review-8124b51b54325c52`). Observaciones, pasan a T12d-fix.
- Commit: `6ddfea8`

### T12d-fix — Observaciones de la revisión de T12a-fix y T12d
- [x] Estado · Ejecutor: `Sonnet` (Codex sin cuota hasta la tarde; necesita PostgreSQL real) · Depende de: T12d
- R1/R3 (WARNING): `ActiveAdminExists` solo cuenta admins `active`; si todos los admins reales están
  bloqueados (RF-017) o deshabilitados, un reinicio crea un segundo admin. D17 dice "si no existe
  ningún admin": contar todo admin que no sea la cuenta pendiente del propio arranque.
- R3/R4 (WARNING): `user.invited` (con el token) se publica dentro de la transacción y antes del
  commit, con el candado tomado: si el broker se cuelga, las demás réplicas esperan; si el commit
  falla, el correo ya salió con un enlace muerto. Es el mismo patrón de T5, `invitationresend` y
  T6. Decisión del usuario (2026-10-01): en el arranque se publica **después del commit** (como
  D16). Implementado así: si la publicación falla, se anula ese token en una transacción corta y el
  arranque siguiente la reemite de inmediato; si la anulación también falla, el token sigue vigente
  sin correo hasta vencer (24 h) y se avisa en el log. T5 y el reenvío de invitaciones quedan con el
  patrón actual, anotados para decidir más adelante.
- R2: la fila de `BOOTSTRAP_ADMIN_EMAIL` en la guía dice "una sola invitación" (hay reemisión);
  tamaño del token sin constante con nombre; comprobación redundante de `@` en `config.go`; clase
  falsa de `BroadcastChannel` duplicada en `client.test.ts`.
- R3 (frontend): `clearSession` tras fallar `/me` anuncia `null` a las demás pestañas; `request()`
  devuelve `undefined` para cualquier 2xx sin `application/json`, incluso en endpoints que esperan
  cuerpo.
- Hecho (2026-10-01, Sonnet; revisó y commiteó Claude). TDD: RED en el orden publicar/commit, en
  admins bloqueado y deshabilitado (creaban una segunda cuenta), en el fallo de publicación y en el
  frontend (`clearSession` anunciaba, cuerpo no JSON aceptado); GREEN. `make test-go` ok,
  `make test-integration` ok, `golangci-lint` 0 issues (con tag solo el `errcheck` previo), frontend
  32/32 + lint + tipos + build, trazabilidad al día, `sqlc` sin deriva, GGA aprobó. E2E
  (`-p t12dfix-check`): con el único admin bloqueado y otro correo configurado, el reinicio no crea
  una segunda cuenta.
- Revisión nativa (alto, 13 archivos, 530 líneas, 4 lentes): **aprobada** y acusada
  (`review-129c43661467fbe6`). Observaciones informativas, pendientes de decidir: R4/R2/R3
  `publishAfterCommit` descarta las causas del error de publicación y de la anulación, así que el
  log no distingue un broker caído de un fallo de base (registrar la causa saneada); R4 ya no hay
  reintento tras un fallo de publicación (antes el arranque fallaba y el orquestador reintentaba);
  R2 el nombre `OutcomeInvitationUndeliveredLive` no se explica solo; R3 `request()` ahora rechaza
  un 2xx con texto plano no vacío (hoy ningún endpoint lo devuelve).
- Commit: `5f66c28`


### T13 — Aplicación Contabilidad (React, otro dominio)
- [x] Estado · Ejecutor: `Codex` → `Sonnet` (Codex sin cuota; decisión del usuario) · Depende de: T9, T11
- Según el mockup: inicio de sesión por redirección con PKCE, verificación del JWT con el JWKS del
  Hub, vistas por rol (Resumen, Transacciones, Cierre contable, Administración) con datos de
  ejemplo. Sin usuarios propios.
- Hecho (2026-10-01, Sonnet; revisó y commiteó Claude). Commit `eaa4670`. App aparte en
  `contabilidad/` (Vite + React + TS, sin dependencias nuevas): PKCE S256 (verifier y `state` en
  `sessionStorage` solo durante la ida y vuelta), canje en `/oauth/token`, token solo en memoria,
  firma Ed25519 verificada con WebCrypto contra el JWKS del Hub (`kid`, `iss`, `aud`, `exp`),
  vistas por rol con datos de ejemplo y nueva redirección a `/oauth/authorize` al vencer (sin
  refresh, ADR 0009). Hueco del Hub corregido: tras el MFA la consola no volvía al
  `/oauth/authorize`; ahora acepta `?continue=` solo si es exactamente `/oauth/authorize` del
  mismo origen (sin redirección abierta; 21 pruebas). La imagen `web` compila las dos apps con
  contexto en la raíz (`frontend/Dockerfile.dockerignore` excluye `.env`, `.git`, backend, etc.);
  se quitó el marcador de T11; CI y Makefile corren para Contabilidad los mismos gates.
- TDD: RED por comportamiento y GREEN: Contabilidad 43/43 (vector RFC 7636, firma y claims
  inválidos, `alg none`/HS256, `state` cambiado, nada en almacenamiento), consola 144/144; lint,
  tipos y build ok en las dos; Trivy de `web` 0 HIGH/CRITICAL; trazabilidad al día; GGA aprobó.
- E2E SSO en Chromium (`-p t13-check`, 30/30): cada usuario entra por el Hub con MFA y vuelve sin
  `code` en la URL ni token en almacenamiento; una segunda pestaña vuelve sin pedir contraseña;
  analista sin Cierre y solo con sus filas; senior aprueba; cambiar el rol en la consola se ve en
  el siguiente login. Pendiente: Firefox y la redirección real a los 15 min (solo prueba unitaria).
- Decisión del usuario (2026-10-01): se mantiene D8. El token de Contabilidad solo lleva roles
  `contabilidad.*`; un admin del Hub sin esos roles ve "Sin acceso". La vista "Administración" de
  la app sobra (T13-fix). El backend emite `"roles": null` con el conjunto vacío.
- Revisión nativa de T12c-fix + T13 (`24bf24c..eaa4670`, alto, 48 archivos, 6.758 líneas, 4
  lentes): **aprobada** y acusada (`review-7ae0824e02b3c4d0`). Observaciones → T13-fix.
- Commit: `eaa4670`

### T13-fix — Decisión D8 en la app y observaciones de la revisión de T13
- [x] Estado · Ejecutor: `Codex` · Depende de: T13
- D8: quitar "Administración" de Contabilidad; "Sin acceso" con enlace al Hub.
- R2/R3 (WARNING): el contador de generación de T12c-fix reemplazó la limpieza al desmontar en
  `UsersPage` y `AuditLogPage`: una respuesta tardía tras salir de la página todavía llama
  `setState` u `onSessionEnded`. Incrementar la generación al desmontar.
- R4 (WARNING): `exp`/`nbf` sin tolerancia y temporizador armado con el mismo `exp`: con el reloj
  del navegador adelantado, bucle de reautenticación. Tolerancia de reloj y freno al bucle.
- R3 (WARNING): la prueba del token que vence no es determinista (reloj real); usar reloj falso.
- R2 (WARNING): `.eslintrc.cjs` de Contabilidad apaga `react/no-danger` con un comentario que dice
  lo contrario; la prohibición real es el `no-restricted-syntax`.
- R4: un fallo al bajar el JWKS se muestra como credencial inválida; distinguirlo.
- R1: CI cae de `npm ci` a `npm install` (también en la consola); usar `npm ci` a secas.
- R2: constantes de prueba duplicadas (`ISSUER`, `AUDIENCE`, `HUB`); `nextId` con número mágico.
- Hecho (2026-10-01, Codex; revisó y commiteó Claude). Commit `89ac1cb`. Contabilidad 49/49, consola
  147/147; lint, tipos y build ok en las dos; GGA aprobó. RED observado en guardas al desmontar,
  límite de reloj, fallo del JWKS y folio malformado; sin RED aislado para quitar Administración y
  para la prueba determinista (el primer RED se colgó por el reloj falso). CI remoto sin correr.
- Revisión nativa (alto, 18 archivos, 295 líneas, 4 lentes): **aprobada** y acusada
  (`review-14ddf284cef4132f`). Observaciones → T13-fix2.

### T13-fix2 — Observaciones de la revisión de T13-fix
- [x] Estado · Ejecutor: `Codex` · Depende de: T13-fix
- R2/R3/R4 (WARNING, coinciden tres lentes): `resend()` de `UsersPage` usa el mismo contador de
  generación que recarga la lista; si la lista se recarga durante un reenvío, se pierden el aviso y
  el cierre de sesión y `resending` queda trabado. Separar "desmontado" de "carga vigente".
- R2 (WARNING): un 4xx o un JSON inválido del JWKS se sigue mostrando como credencial inválida.
- R2 (WARNING): `MINIMUM_SESSION_LIFETIME_MS` y `CLOCK_LEEWAY_SECONDS` están acoplados sin
  decirlo; derivar uno del otro.
- R3 (WARNING): las pruebas del callback con reloj falso hacen un solo `advanceTimersByTimeAsync(0)`
  antes de afirmar; WebCrypto no resuelve como microtarea, pueden fallar al azar.
- R2: mensaje del reloj fuera de `CALLBACK_MESSAGES`; la prueba de `nextId` lleva la etiqueta RF009
  y un `as never`.
- Hecho (2026-10-01, Codex con la lista de archivos permitidos de Gentle AI 4.0; revisó y commiteó
  Claude). Commit `d0283df`. RED/GREEN por cambio; Contabilidad 53/53 cinco corridas seguidas,
  consola 148/148; lint, tipos y build ok; GGA aprobó.
- Revisión nativa (alto, 8 archivos, 130 líneas, 4 lentes): **aprobada** y acusada
  (`review-8e77444c1bb38413`). Observaciones sobre calidad de pruebas, pendientes (no bloquean el
  corte): R3 `advanceUntil` solo reduce la fragilidad de WebCrypto con reloj falso (presupuesto de
  ~100 ms); R3 la prueba de la vida mínima no discrimina el umbral (el token ya está vencido) y la
  aserción de la constante es tautológica; R2 títulos de `it.each` con espacios.
- Commit: `d0283df`



## Fase 3 — Verificación, DAST y cierre

### T14 — E2E con Playwright
- [ ] Estado · Ejecutor: `Claude` (instala Playwright y navegadores) + `Codex` (pruebas)
- Una prueba por escenario Gherkin, lectura de correos por la API de Mailpit, el guion de la demo
  como prueba; `make e2e` real y job en CI; `spec-drift` comprueba la correspondencia.
- Commit: —
- Mapa (explorador, 2026-10-02): 37 escenarios en 6 features (`specs/06-acceptance/`, etiquetas
  `@RF-NNN`); `scripts/traceability.py` solo cuenta pruebas por RF (no falla si falta una);
  `make spec-drift` no existe; ningún job de CI levanta el stack; el admin inicial no tiene
  contraseña (solo invitación por correo); `/api/v1/auth/` limitado a 5r/s burst 5 (429).
- Decisión del usuario (2026-10-02): los escenarios sin camino E2E puro (auditoría inmutable,
  invitación vencida, encolado) se cubren con acceso directo controlado (`docker exec … psql`) para
  que los 37 tengan prueba E2E real, no con exenciones. Las pruebas crean su propio admin
  (`e2e-admin-*@example.test`, invitación sembrada por SQL y aceptada por la API real), sin tocar
  las cuentas del usuario.
- Convención: un `test()` por escenario titulado `RF-NNN <nombre exacto del escenario>` (primera
  etiqueta RF), en `e2e/tests/<feature>.spec.ts`.
- Subtareas:
  - [x] T14a — Andamiaje: `e2e/package.json` + lockfile (`@playwright/test` 1.63.0), config,
    helpers (Mailpit, API, admin sembrado, correos únicos, ritmo bajo el límite de 5r/s),
    `make e2e`, smoke test de Nginx con cabecera `Host` (deuda de T11) y el escenario "aceptar
    invitación" como prueba del arnés. Ruta: delegada (Sonnet: red y Docker).
    Hecho (2026-10-02): `e2e/` con `@playwright/test` 1.63.0 fijo, helpers (`support/`: Mailpit,
    API con ritmo de 300 ms, psql por `execFile` con variables `-v`, admin sembrado), 6 pruebas
    RNF-009 de Nginx por `Host` y RF-002 "Aceptar la invitación…" por la UI. Revisión de Claude:
    se endureció la prueba de `/oauth/` (exige 400 `application/problem+json` de la API, no solo
    "no es HTML"). Evidencia: `make e2e` 7/7 tres veces seguidas (idempotente), `tsc --noEmit` ok,
    gitleaks sobre `e2e/` sin hallazgos, matriz regenerada (`traceability.py --check` al día).
  - [x] T14b — Escenarios restantes de las 6 features. Ruta: delegada (Codex escribe, Claude corre).
    - Lote 1 (2026-10-02, Codex escribió, Claude corrió y revisó): autenticación (8), rotación de
      sesión (4) y restablecimiento (3). Revisión de Claude: "Agotar los intentos" ahora prueba que
      el `mfaToken` agotado rechaza el código **correcto** y que los 5 rechazos quedan en
      `audit_log` (antes repetía un código incorrecto, que pasaría igual con el desafío vivo); la URL
      del Hub sale de `support/config.ts` en vez de 22 literales. Codex endureció `mailpit.ts`
      (`full.ok`, reintento ante errores de red), advertencias de la revisión de T14a.
    - Problema hallado: el límite por IP de RF-017 (20 fallos en 15 min, contados en `audit_log`, que
      no se puede borrar) bloqueaba la segunda corrida con 423. Decisión del usuario (opción a):
      `make e2e` recrea la API con `LOGIN_IP_MAX_FAILURES=1000` mientras corre y la restaura al
      terminar (compose pasa la variable con default 20); el límite por cuenta no se toca; chequeo
      previo (`support/global-setup.ts`) que falla con un mensaje claro si la IP ya está bloqueada.
    - Deriva spec↔código (sin id, a decidir): `autenticacion.feature` espera el evento de auditoría
      `mfa_succeeded`; el backend emite `mfa_code_accepted` y `login_succeeded`. La prueba afirma lo
      real (`login_succeeded`). Hallazgos de la revisión de T14a/rango aún abiertos: timeout en
      `psql` de `db.ts:32` y la prueba débil de `nginx.spec.ts:63-72`.
    - Evidencia: `make e2e` 22/22 en tres corridas seguidas (antes del ajuste, la segunda daba 423);
      tras cada corrida la API vuelve a `LOGIN_IP_MAX_FAILURES=20`; `tsc --noEmit` ok.
    - Correcciones tras la revisión de 4 lentes de `7b45057` (Codex escribió; Claude corrió y añadió
      lo último): los correos se esperan excluyendo los ID que ya existían antes de la acción (las
      carreras de login doble, reenvío y desafíos sucesivos); "Agotar los intentos" deja 1 intento
      por SQL para aislar el agotamiento del bloqueo de la cuenta (ambos umbrales son 5) y comprueba
      que la cuenta sigue `active`; `LoginRejectedError` tipado para el chequeo previo; `make e2e`
      restaura el límite con `trap` en EXIT/INT/TERM; `psql` con timeout de 30 s; tipos desde
      `@playwright/test` (no de `playwright-core`, que no está declarado). Evidencia: `make e2e` 22/22
      dos veces seguidas; interrumpido con SIGINT a mitad de la suite, el límite vuelve de 1000 a 20.
    - Lote 2 (2026-10-02, Codex escribió, Claude corrió y revisó): registro (6 restantes) y control de
      acceso (8), incluida "sin acceso a Contabilidad" por navegador. Revisión de Claude: la prueba
      de auditoría inalterable ahora hace `SET ROLE identity_app` (el escenario habla de la
      aplicación; Codex la corría como dueño de la tabla) y espera `insufficient_privilege` en UPDATE
      y DELETE; "conserva sus roles anteriores" compara la lista exacta. Codex añadió además lo que
      pidió la revisión de `e319485` (comentario del atajo SQL, `attempts_left` inicial = 5, reintento
      del snapshot de Mailpit). Evidencia: `make e2e` 36/36 dos veces; matriz al día.
    - Advertencias abiertas de la revisión del rango (para la limpieza al cerrar T14): `fetch` sin
      timeout en `api.ts`/`mailpit.ts`, helpers de login duplicados entre `api.ts` y `auth.ts`, URL
      base del Hub en dos lugares (`playwright.config.ts` y `support/config.ts`), prueba débil de
      `nginx.spec.ts:63-72`.
    - Más advertencias de la revisión del rango con el lote 2 (aprobada): `make e2e` no avisa si falla
      la restauración del límite; el 429 de "ventana mínima" no distingue RF-014 del limitador de
      Nginx (afirmar el tipo de problema); falta un timeout por prueba acorde a los flujos con varios
      correos (hoy, 30 s por defecto).
    - Lote 3 (2026-10-02, Codex escribió, Claude corrió y revisó): OAuth (9), cuatro por navegador
      (PKCE completo, segundo acceso por SSO sin contraseña ni correo nuevo, login con `continue`,
      `continue` externo sin redirección abierta) y cinco de protocolo. Al correrlo, 2 fallaban por
      exigir `aud` como texto: el backend lo emite como lista de un elemento, que RFC 7519 admite y la
      spec ("identifica a Contabilidad") también; la prueba ahora exige exactamente `[contabilidad]`.
      Evidencia: `make e2e` 45/45 (39 escenarios Gherkin + 6 RNF-009 de Nginx); matriz al día.
    - Cierre de T14b: los 39 escenarios Gherkin tienen exactamente una prueba con su título (el mapa
      inicial decía 37; recuento real con `grep`/`comm`: 39, sin faltantes ni sobrantes).
  - [ ] T14c — Guion de la demo como prueba (criterio de aceptación 1). Ruta: delegada.
  - [x] T14d — `spec-drift` comprueba la correspondencia escenario↔prueba (falla si falta o sobra)
    y `make spec-drift`. Ruta: delegada (Codex, sin red).
    Hecho (2026-10-02): Codex escribió casi todo y se cortó por su límite de uso antes de regenerar
    la matriz; Claude lo revisó y terminó. `scripts/traceability.py` exige uno a uno escenario↔prueba
    (falta, sobra o duplicada → error con archivo:línea) y, añadido por Claude, también falla si un
    título RF/RNF no es literal (antes quedaba invisible). `nginx.spec.ts` con títulos literales: la
    matriz cuenta 6 en RNF-009 (antes 4). `scripts/traceability_test.py` (stdlib) y `make
    spec-drift`. Evidencia: RED sobre el repo real (un título pasado a template → exit 1 con el
    escenario faltante y el título no literal), GREEN tras restaurarlo; `make spec-drift` ok; 6/6
    de Nginx. El job de CI ya corre `traceability.py`, así que lo exige sin renombrarse; sumar
    `traceability_test.py` al job queda para T14e.
  - [x] Limpieza de advertencias de las revisiones de T14 (2026-10-02, Sonnet porque Codex estaba sin
    cupo; revisó y commiteó Claude): timeouts en los `fetch` de `api.ts`/`mailpit.ts`; un solo flujo
    de login con MFA (`passwordThenMfa`); `hubUrl` como única fuente de la URL; timeout de 120 s por
    prueba; `make e2e` avisa y sale con error si falla la restauración del límite (rama escrita pero
    no ejercitada); el 429 de "ventana mínima" exige el problema `mfa-resend-rate-limited` de la app;
    "no aceptó la invitación" ya no es vacía (contraseña conocida y cuenta devuelta a pendiente por
    SQL → 401 `invalid-credentials` sin revelar el estado); el reenvío acepta el token nuevo (204);
    la espera de la redirección OAuth solo acepta el 302 hacia el `redirect_uri`; Nginx por hosts con
    nombre y Contabilidad identificada por su `<title>`; un solo parser de Gherkin, etiquetas que no
    se arrastran entre bloques, escenarios duplicados como error y pruebas de esas ramas; comentario de
    `advanceUntil` corregido. Evidencia: `make e2e` 45/45 dos veces, límite de vuelta en 20,
    `make spec-drift` ok (3 pruebas + matriz idéntica), Contabilidad 53/53, `tsc` ok.
    Revisión de `e28d3cf` aprobada; se quitó la constante muerta `GHERKIN_SCOPE` (el reinicio real
    de etiquetas es la rama `else` del parser). Decisión consciente: "no aceptó la invitación"
    fabrica el estado por SQL (contraseña conocida + `pending_verification`), porque un invitado
    real nunca tiene una contraseña conocida y solo así se prueba que el 401 viene del estado.
  - [ ] T14e — Job E2E en CI (stack con `.env` generado), sin renombrar los 17 checks
    obligatorios; verificarlo requiere push (decisión del usuario).

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
| 1 — Backend | T4 a T10 (7) | 7 (T4 a T10) |
| 2 — Dominios locales y frontend | T11 a T13 + T12d (4) | 4 (T11 a T13 y T12d) — fase cerrada |
| 3 — Verificación, DAST y cierre | T14 a T17 (4) | 0 |
| **Total** | **18** | **14** |

## Siguiente paso

Fase 1 cerrada el 2026-09-30 (T9-fix y T10). PR de corte de las fases 0 y 1: #6, mergeado el
2026-10-01 en `ece7e7f` (merge commit; CI de `main` en verde). Incluye `7bfc125`, falso positivo de
gitleaks en `accept_invitation.go` agregado a `.gitleaksignore`.

Fase 2 cerrada el 2026-10-01 (T11, T12 con T12d y T13, con sus fixes; última revisión
`review-8e77444c1bb38413`). PR de corte de la fase 2: #7, mergeado el 2026-10-01 en `2806a37`
(merge commit; CI del PR 17/17, run 36939947063). Antes del merge: tres falsos positivos de
gitleaks a `.gitleaksignore` (`85b8cc0`) y `0cd9cd1` (revisión nativa aprobada,
`review-f4cb1ae541fa4cb2`): CVE-2026-103111 (HIGH, `pcre2` 10.48-r0 de la base Alpine, sin imagen
nueva de upstream) parcheado con `apk upgrade --no-cache pcre2` en la etapa final (decisión del
usuario; quitar esa capa cuando el digest lo incluya; Trivy de CI la rechaza si no corrige), `npm ci
--ignore-scripts` sin fallback (SonarCloud S6505/S8543) y nombre explícito del job de imágenes para
conservar los checks obligatorios del ruleset. Tras el merge, el CI de `main` falló en una prueba
frágil de Contabilidad (WebCrypto con reloj falso, la que marcó R3 en T13-fix2); arreglada en
`89d6051` (el helper cede un macrotask real en cada vuelta; 53/53 diez veces y doce bajo carga) y
mergeada por el PR #8 en `795b30d`; CI de `main` en verde (run 36941918054).

Bugs reportados por el usuario al probar a mano (2026-10-01, `make up`, base local), a resolver
antes de T14 como **BUG-1** y **BUG-2** (evidencia de los logs de Nginx/API tomada en el momento):
- **BUG-1 — aceptar invitación falla con "No se pudo conectar con el servicio".** El admin creó la
  cuenta (`POST /api/v1/admin/users` 201, 00:23:08 UTC), llegó el correo, el enlace abrió
  `/invitations/accept?token=…` (00:23:30) y al enviar la contraseña salió ese mensaje. En los logs
  de Nginx **nunca llega** un `POST /api/v1/auth/invitations/accept`: el fallo ocurre en el
  navegador antes o al hacer el `fetch` (ese mensaje es el de un error que no es RFC 7807). En
  cambio, restablecer contraseña sí funciona (`request` 202, `confirm` 204).
- **BUG-2 — `t12a-user@example.test` (analista) no entra a Contabilidad** tras restablecer la
  contraseña (el restablecimiento funcionó). El Hub emite el código: hay tres
  `/oauth/authorize` → `/oauth/callback?code=…` (00:21:19, 00:21:30, 00:28:56), pero **nunca llega**
  un `POST /oauth/token` (ni un `OPTIONS` previo) a Nginx: Contabilidad falla en el navegador antes
  de canjear el código. La E2E de T13 pasó en Chromium headless con el mismo flujo, así que puede
  depender del navegador o del estado (pestañas, `sessionStorage`, extensiones). Dato aparte: esa
  cuenta se creó a mano por SQL en T12a y no tiene el rol base `user` (solo
  `contabilidad.analista`); no explica que falte el canje, pero conviene descartarlo.
- Plan: reproducir con Playwright en Chromium y Firefox mirando consola, red y CSP; preguntar al
  usuario el navegador y lo que muestra la consola; prueba que falle primero (TDD) y arreglo.
- Avance (2026-10-02): **no se reproducen** con un script de Playwright desechable (scratchpad)
  contra el stack del usuario, ni en Chromium ni en WebKit 26.6 headless, ni con el admin logueado en
  otra pestaña del mismo navegador: `invitations/accept` responde 204 con "Tu cuenta quedó activada";
  Contabilidad canjea el código (`/oauth/token` 200) y carga el panel del analista. El usuario
  confirmó que usó **Safari** y que Contabilidad mostró un "error de sesión" (`state` o `exchange` en
  `contabilidad/src/App.tsx`). En sus logs el Hub emitió códigos, así que Safari sí envió
  `hub_session`; el fallo ocurre en Contabilidad antes del `fetch`. Hipótesis: Safari pierde
  `state`/`verifier` de `sessionStorage`, o rechaza el `fetch` cruzado antes de enviarlo. Siguiente
  paso, aplazado por el usuario: reproducir en Safari real con `safaridriver` (requiere `sudo
  safaridriver --enable` y "Permitir automatización remota"). Datos de prueba que quedan en la base
  local: usuarios `bug-*@example.test` y la contraseña cambiada de `t34a-check3@example.com` (el rol
  `admin` temporal ya se quitó).
- Hallazgo aparte (no es un VULN, sin id): WebKit registra en la consola de
  `/invitations/accept` "Refused to apply a stylesheet because its hash, its nonce, or
  'unsafe-inline' does not appear in the style-src directive". La CSP del Hub bloquea un estilo
  inline; falta ver qué lo inyecta y decidir en qué tarea se corrige.

Siguiente: fase 3 empezando por T14 (BUG-1/BUG-2 quedan pendientes de la prueba en Safari real) (E2E con Playwright, incluido el smoke test de Nginx
con cabecera `Host` pendiente de T11). Para delegar con Gentle AI 4.0, cada tarea a un agente que
escribe lleva su `## Allowed edit surfaces`.

- `main` protegida desde el 2026-10-01 (ruleset "Protect main", decisión del usuario): PR
  obligatorio sin aprobaciones requeridas, los 17 checks del PR #6 obligatorios, sin force push ni
  borrado, solo merge commit (squash y rebase deshabilitados en el repo) y sin bypass.
- Evidencias (decisión del usuario, 2026-10-01): se toman al cambiar de semana, no por fase. El
  cambio a Alpine no lleva captura por ahora.
- Code scanning tras el merge: 85 alertas abiertas (antes 417; 332 eran de la imagen `web` Debian).
- Hallazgo nuevo fuera de alcance: CodeQL reporta 33 `js/remote-property-injection` en los tres HTML
  de `docs/diagramas/` (diagramas generados, no código de la app). El plan de la semana 2 decía no
  commitear esa carpeta y está versionada. Pendiente decidir si se sacan del repo o se excluyen del
  análisis.

- Hallazgo nuevo fuera de alcance (CI del PR #6, run 36863251783, job "9-10 · Construir y escanear
  imágenes (web)"): Trivy encuentra 13 HIGH corregibles en la imagen base de `web`
  (`nginxinc/nginx-unprivileged`, Debian 13.7, fijada por digest en `8f3d461`): `libheif1` y sus
  plugins (CVE-2026-84450, CVE-2026-84451), `openssl`/`libssl3t64`/`openssl-provider-legacy` y
  `libpcre2-8-0` (CVE-2026-103111, CVE-2026-75804, CVE-2026-84782). No lo introduce este PR
  (`frontend/Dockerfile` sin cambios desde el 2026-09-26): son avisos publicados después de fijar el
  digest. Remediado antes del merge (decisión del usuario, 2026-10-01): el digest más reciente
  de `stable` (2026-09-28) seguía con las 13, así que la base pasó a
  `nginx-unprivileged:stable-alpine` (Alpine 3.24.2, `sha256:ed04ec1f...`). Verificado en local:
  Trivy con los parámetros de CI da 0 HIGH/CRITICAL corregibles; el contenedor corre como uid 101,
  sirve la SPA (ruta profunda 200) y responde las cinco cabeceras de RNF-009. Anotado en la ficha
  de VULN-017. Sin ficha VULN nueva: queda a decisión del usuario si la lleva.

## Cambios de spec propuestos

Todos en T3, tras T2.

## Notas de handoff Codex

- T8: se implementaron `GET/DELETE /api/v1/me/sessions` sobre familias de refresh, con propiedad del titular, `404` indistinguible para familia ajena/desconocida, auditoría `session_revoked` y marca `current` mediante el `sid` firmado del access token; se añadieron servicio, adaptador SQLc y composición.
- T7-fix: un fallo al publicar el desafío inicial lo anula para no agotar el cupo; un fallo al publicar un reenvío restaura código y `last_sent_at`; la superación usa el reloj del servicio. ADR 0006 y OpenAPI documentan ambos casos; la prueba real de `login.Service` confirma 429/503.
- RED observado: `cd backend && GOCACHE=/tmp/identity-hub-go-build go test ./internal/auth/session ./internal/auth/mfa ./internal/api -run 'TestRF016_|TestRF014_FalloDeEntrega' -count=1` falló por el paquete de sesiones inexistente, desafío no anulado y `last_sent_at` adelantado. GREEN: la misma selección y `go test -race ./internal/auth/session ./internal/auth/mfa ./internal/auth/refresh ./internal/api -count=1` pasaron.
- Verificación: `make gen`, `make test-go`, `make lint`, `go vet -tags=integration ./...`, validación OpenAPI y trazabilidad en verde; T8 corrige además el generador para que RF-016 ya no figure diferido. Claude debe ejecutar `make test-integration` con PostgreSQL real; el sandbox no tiene `TEST_DATABASE_URL`.
