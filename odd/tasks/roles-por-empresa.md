# Roles separados por empresa

## Objetivo

Que el Hub sirva a varias empresas sin mezclar sus datos ni sus roles: cada aplicación y cada usuario
pertenecen a una sola empresa, a un usuario solo se le asignan roles de aplicaciones de su empresa, tras
iniciar sesión el Hub lo lleva a las aplicaciones de su empresa, y el Hub rechaza el acceso a una
aplicación de otra empresa.

## Problema y por qué

La grilla de roles solo servía a Contabilidad y no había respuesta para una empresa tercera. Mapa del estado actual (2026-10-19, exploración con evidencia path:line):

- No existe la entidad empresa: `users` no tiene organización; `applications` (migración 000007) tiene una
  sola fila, `contabilidad`.
- Los roles ya son por aplicación (`roles.application_id`, migración 000009; nombre `<client_id>.<slug>`) y
  el token de una aplicación solo lleva sus roles (`aud` = `client_id`).
- Un único cliente OAuth fijo en el código (`backend/cmd/api/main.go:159`, `config.go:75`; RF-020 dice
  «único cliente»); CORS, `redirect_uri` y `aud` comparan contra ese cliente.
- Un administrador puede asignar a un usuario roles de cualquier aplicación (`UserDrawer.tsx`), sin
  validación por empresa.
- Tras el login: admin a `/inicio`, resto a `/me`; el menú muestra «Contabilidad» fijo a todos
  (`frontend/src/App.tsx:185`).
- `/oauth/authorize` no rechaza a un usuario sin rol en la aplicación: emite un token vacío y la aplicación
  muestra «sin acceso» (`control-de-acceso.feature:19`).

## Decisiones del usuario (2026-10-19)

- **Modelo «empresa ligera»:** tabla de empresas; cada aplicación y cada usuario pertenecen a una sola
  empresa; el backend impide asignar roles de otra empresa; el Hub enruta por empresa y rechaza en
  `/oauth/authorize`. El administrador del Hub sigue siendo global (operador). Clientes OAuth cargados desde
  la base de datos. Descartados: multi-tenencia completa con administrador por empresa, y «solo
  aplicaciones» sin entidad empresa.
- **Demostración con una segunda aplicación mínima:** «Inventario» de «Empresa B» en
  `inventario.localhost:8080`, servida por el mismo Nginx, con su propio cliente OAuth, roles y permisos.

- **Aclaración del usuario (2026-10-19), objetivo principal:** que la consola del Hub separe las empresas.
  El administrador elige primero la empresa y trabaja en una vista solo de ella: la grilla de roles muestra
  solo sus aplicaciones y roles (crear un rol nunca muestra roles de dos empresas juntos), el listado de
  usuarios solo sus usuarios, y al asignar un rol solo se ofrecen roles de la empresa del usuario. Inventario
  sigue en alcance como aplicación de la segunda empresa, pero la separación en la consola es lo central.
  Confirmado por el usuario: se construye la app básica de Inventario.

## Alcance autorizado

- ADR nuevo, requisitos (RF-020 y RF-021 ajustados o RF nuevo), modelo de dominio, escenarios de aceptación.
- Migración: empresas, pertenencia de usuarios y aplicaciones, datos semilla (empresa de Contabilidad con los
  usuarios existentes; «Empresa B» con Inventario, sus roles y permisos).
- Backend: clientes OAuth desde la base; rechazo en `/oauth/authorize` por empresa o falta de rol;
  validación de asignación de roles por empresa; endpoint con las aplicaciones del usuario; empresa del
  usuario al invitarlo.
- Consola del Hub: empresa en alta y detalle de usuario; roles asignables solo de su empresa; menú de
  aplicaciones dinámico; redirección tras el login a la aplicación de su empresa.
- Aplicación `inventario/` mínima, su bloque de Nginx, build en la imagen `web` y prueba E2E del cruce.
- Documentación: guía de integración, manual de usuario, README y matriz.

Fuera de alcance: administrador por empresa, separar la auditoría por empresa, alta de empresas o
aplicaciones desde la consola (se declaran por migración, como hoy Contabilidad).

## Restricciones

- Sin push ni PR mientras dure el incidente de GitHub Actions (decisión del usuario, 2026-10-19).
- Prueba primero (RED → GREEN) donde haya pruebas deterministas; Postgres real para integración
  (`TEST_DATABASE_URL`, ver CLAUDE.md); `golangci-lint` instalado y corrido antes de cerrar tareas de Go.
- Handlers compuestos de verdad en `Server.Routes()` (CLAUDE.md).
- `make gen` tras cambiar OpenAPI o SQL; la deriva entre specs y código la revisa CI.
- Mantenimiento posterior a la entrega: sin fichas VULN.

## Tareas

- [x] **R1. Especificación.** ADR 0015, requisitos, modelo de dominio y escenarios `empresas.feature`.
- [x] **R2. Datos.** Migración de empresas y semillas; consultas sqlc; trigger o validación de que un rol
  asignado pertenece a la empresa del usuario; pruebas de integración.
- [x] **R3. OAuth multi-cliente.** Clientes desde `applications`; `redirect_uri`, CORS y `aud` por cliente;
  rechazo en `/oauth/authorize` por empresa o sin rol; pruebas.
- [x] **R4. Administración y enrutamiento.** API: empresa del usuario, roles asignables, aplicaciones del
  usuario; consola: alta con empresa, `UserDrawer` filtrado, menú dinámico y redirección tras el login.
- [ ] **R5. Inventario.** App mínima, Nginx, build en `web`, compose, E2E del cruce entre empresas.
- [ ] **R6. Documentación y cierre.** Guía de integración, manual de usuario, README, matriz; checks completos.

## Criterios de aceptación

- Un usuario de Contabilidad S.A. entra a Contabilidad; uno de Empresa B entra a Inventario; ninguno puede
  abrir la aplicación de la otra empresa (el Hub rechaza en `/oauth/authorize`).
- El backend rechaza asignar a un usuario un rol de una aplicación de otra empresa.
- Tras el login, cada usuario ve solo las aplicaciones de su empresa.
- Contabilidad sigue funcionando igual para sus usuarios; todas las pruebas existentes pasan.

## Entrega

Estrategia `ask-on-risk`; previsión muy por encima de 400 líneas (migración, backend, consola, app nueva).
Estrategia de cadena (decisión del usuario, 2026-10-19): **PRs encadenados a `main`** (`stacked-to-main`),
un PR por bloque coherente: R1+R2, R3, R4, R5+R6. Mientras GitHub Actions siga degradado, quedan como
ramas locales y se suben después.

## Progreso

- 2026-10-19: exploración hecha; modelo y demostración decididos con el usuario; rama
  `feat/roles-por-empresa` creada desde `main` (local).
- 2026-10-19 (R1, redactado, sin marcar): `specs/adr/0015-empresas-y-aislamiento-de-roles.md` (con la
  separación por empresa en la consola como objetivo principal), índice en `specs/adr/README.md`,
  RF-022 nuevo y RF-020/RF-021 ajustados en `specs/01-requirements.md`, entidad `organizations` e
  invariantes 12 a 14 en `specs/02-domain-model.md`, `specs/06-acceptance/empresas.feature` (11
  escenarios) y un escenario ajustado en `control-de-acceso.feature` (rechazo en authorize en lugar de
  token vacío). Comprobaciones: `traceability.py --check` y `make spec-drift` fallan solo por los 11
  escenarios de RF-022 sin E2E (esperado hasta R5; la matriz no se regenera hasta entonces);
  `traceability_test.py` y los demás `*_test.py` de `spec-drift` pasan.

- 2026-10-19, revisión de R1 (sesión principal): ADR 0015 refleja la separación en la consola como
  objetivo principal (punto 6) y el filtrado también en la API. Decisiones aceptadas: cambiar la empresa de
  un usuario se rechaza mientras tenga roles de la anterior. Pendientes para R2–R4: nombre de la acción de
  auditoría del rechazo en `/oauth/authorize` (R3), forma del parámetro de empresa en la API (R4), y a qué
  empresa pertenece el administrador global (R2; `users.organization_id` es `NOT NULL`).
- **Consecuencia para la entrega:** `empresas.feature` se guarda fuera del repo hasta tener sus E2E (el hook de pre-commit también corre la matriz). `scripts/traceability.py` y `make spec-drift` exigen una E2E por cada
  escenario con tag `@RF`, así que en esta rama fallan hasta que existan. Cada escenario entra a `main` en
  el mismo PR que su E2E; el corte de PR se ajusta al subir (el primero será más grande que R1+R2).
- 2026-10-19, revisión nativa de R1 (aprobada, 4 avisos, todos ya en la lista de pendientes): referencia del
  ADR a `empresas.feature` (entra con sus E2E); contrato del rechazo en `/oauth/authorize` (R3); empresa del
  administrador global (decidida: «Identity Hub (operador)», reflejar en `specs/02-domain-model.md` en R2 o
  R6); el escenario ajustado de `control-de-acceso.feature` describe el rechazo antes de existir: su E2E
  sigue probando el comportamiento viejo hasta R3, que debe actualizarla.
- 2026-10-19 (R2, escrito por el ejecutor, sin marcar; la revisión y el commit son del orquestador):
  - Migración `000011_organizations` (up/down; ida, vuelta e ida verificada en un Postgres desechable):
    tabla `organizations` con tres empresas de UUID fijo (`hub` «Identity Hub (operador)»
    `...0a1`, `contabilidad-sa` `...0a2`, `empresa-b` `...0a3`); `applications.organization_id` y
    `users.organization_id` `NOT NULL` con respaldo (admins → hub, resto → contabilidad-sa) e índice en
    usuarios; aplicación `inventario` de Empresa B con 3 permisos (`productos.ver`, `productos.editar`,
    `movimientos.registrar`, sin prefijo como en 000009) y 2 roles (`inventario.consulta`,
    `inventario.bodeguero`). Triggers con SQLSTATE propio `IH001`: `user_roles` INSERT/UPDATE (rol de
    aplicación de otra empresa; los roles de directorio quedan exentos) y `users` UPDATE OF
    `organization_id` (rechaza mientras tenga roles de aplicaciones de la empresa anterior).
  - sqlc: `CreateUser` recibe la empresa; consultas nuevas en `db/queries/organizations.sql`
    (`ListOrganizations`, `GetUserOrganization`, `ListOrganizationApplications`, `ListOrganizationUsers`
    con el mismo filtro y cursor que `ListAdminUsers`).
  - Store (`backend/internal/store/organizations.go`): constantes `HubOrganizationID`,
    `ContabilidadOrganizationID`, `EmpresaBOrganizationID`; errores tipados `ErrCrossOrganizationRole`
    (mapea `IH001` en `ReplaceRoles` y en `AddUserRole` del alta de empleados) y `ErrOrganizationRequired`
    (`CreateUser` exige empresa). Sin cambios al contrato OpenAPI.
  - **Transitorio (se retira en R4):** el alta de empleados sin empresa usa `contabilidad-sa`
    (`employee.Input.OrganizationID` cero); el administrador de arranque (`BOOTSTRAP_ADMIN_EMAIL`) va a `hub`.
    La grilla de roles (`ListApplications`) sigue sin filtrar por empresa hasta R4, por lo que ahora lista
    también Inventario.
  - Pruebas existentes ajustadas por el cambio de datos (no de comportamiento): altas de usuario con
    empresa; catálogo de roles de 000004 y listado de aplicaciones de la grilla incluyen Inventario; las
    aplicaciones ajenas insertadas por SQL crudo llevan empresa.
  - Pruebas nuevas `TestRF022_*` (7, `organizations_integration_test.go`). RED observado quitando los
    triggers de la migración: fallaron las 3 que dependen de ellos (asignación vía store, SQL crudo y
    cambio de empresa); GREEN con los triggers. Las de siembra, listados por empresa, roles de
    directorio y «alta exige empresa» describen datos/consultas que no compilan ni existen sin la
    implementación, así que no tienen RED separado.
  - `make gen` regeneró `specs/07-traceability.md` (RF-022 pasa de parcial a completo por las 7 pruebas
    Go; los escenarios siguen sin E2E hasta R5 porque `empresas.feature` aún no está en el repo).

- 2026-10-19 (R3, escrito por el ejecutor, sin marcar; revisión y commit del orquestador):
  - Clientes desde `applications`: consultas nuevas `ListOAuthApplications`, `GetOAuthAuthorizationAccess`
    y `ListApplicationRolesForUser` (sustituye a `GetOAuthApplication`); `Store.ListOAuthClients`;
    `oauth.New` recibe `[]Client` (con `OrganizationID`); `oauth.ValidateClients` falla al arrancar si no hay
    clientes o alguno está mal formado (id, `redirect_uri` http(s) absoluto sin fragmento, origen igual al de
    la URI, ids únicos). Decisión: la tabla se lee una vez al arrancar (las aplicaciones entran por
    migración + despliegue, ADR 0015 punto 8); `main.go` ya no depende de un cliente único. Se retiraron
    `ValidateOAuthClient`, `ValidateRegisteredClient` y las variables `OAUTH_CLIENT_ID`,
    `OAUTH_REDIRECT_URI`, `OAUTH_CLIENT_ORIGIN` (config y sus pruebas); `SetOAuthService` ya no recibe cliente.
  - CORS de `/oauth/token` y `/.well-known/jwks.json`: eco exacto del origen si coincide con el
    `allowed_origin` de cualquier cliente registrado; nunca comodín. `aud` = `client_id` del código
    canjeado (`ExchangeResult.ClientID`). Roles del token: ahora por `roles.application_id` (consulta
    `ListApplicationRolesForUser`), ya no por prefijo del nombre.
  - Rechazo en `/oauth/authorize`: empresa distinta (`organization_mismatch`, se evalúa primero) o sin rol
    de la aplicación (`no_application_role`): sin código, auditoría `oauth_authorization_rejected`
    (metadata `reason`, `client_id`; actor = usuario) y `302` a la `redirect_uri` registrada con
    `error=access_denied` y el `state`. Si falla la auditoría, error 500 (no se emite nada). Cliente o URI
    inválidos conservan el comportamiento anterior. OpenAPI: solo texto de la descripción de
    `/oauth/authorize` (sin cambio de forma); `make gen` regeneró `schema.d.ts`, sqlc y la matriz.
  - Contabilidad: `completeCallback` distingue `error=access_denied` (`CallbackError` `no_access`) y `App`
    muestra «Sin acceso a Contabilidad» sin pedir token; los demás errores siguen igual.
  - RED/GREEN: pruebas escritas antes; RED observado como fallo de compilación en `internal/auth/oauth`
    (`undefined: AuthorizationAccess`) y en `internal/api` (firma de `SetOAuthService`), y como fallo de
    comportamiento en Vitest (`access_denied` daba `denied`; la pantalla «Sin acceso» no aparecía).
    GREEN tras implementar. Pruebas nuevas: `TestRF020_VariosClientesRegistrados...`,
    `TestRF020_OrigenPermitido...`, `TestRF020_ValidateClients...`, `TestRF020_CORSPermiteElOrigenDeCada...`,
    `TestRF020_PostgresListaLosClientesRegistrados...`, `TestRF022_AutorizacionRechazadaNoEmiteCodigoYAudita`,
    `TestRF022_FalloDeAuditoriaDelRechazo...`, `TestRF022_CanjeDeInventarioLlevaSoloSusRoles...`,
    `TestRF022_AuthorizeRechazadoRedirigeConAccessDenied...` (+ variante con cliente/URI inválidos),
    `TestRF022_AuthorizeRechazaUsuarioDeOtraEmpresa...` (Postgres real, vía `Server.Routes()`),
    `TestRF022_PostgresAutorizaSoloAUsuariosDeLaEmpresaConRol...`, y dos de Contabilidad. Se retiraron 2
    pruebas de `config` (la validación pasó a `ValidateClients`) y la de `ValidateRegisteredClient`.
    Pruebas existentes ajustadas: usuarios que piden código ahora necesitan un rol de la aplicación.
  - E2E: «Un empleado sin rol de aplicación no accede a Contabilidad» ahora comprueba `error=access_denied`
    en la redirección de authorize, ningún `code`, ninguna petición a `/oauth/token` y la auditoría; el
    test de reutilización de código usa un usuario con `contabilidad.analista`.
  - Fuera de mis superficies, pendiente para el orquestador: `.env.example` (comentarios `OAUTH_*`),
    `docs/guia-desarrollo.md`, `docs/manuales/*` (integración, despliegue), `docs/informe/informe-tecnico.md`,
    `infraestructura/terraform/README.md` y ADR 0012 aún citan esas variables (R6); `e2e/support/accounts.ts`
    inserta el administrador por SQL sin `organization_id` y rompe todo el E2E tras R2 (ver verificación).

  - Verificación de R3: `make gen` (solo cambios esperados: sqlc, `schema.d.ts`, matriz); `make test-go` verde;
    `make test-integration` con Postgres desechable verde, cobertura total 81.2 %, y `-v -run 'TestRF02[02]'`
    muestra las `TestRF020_*`/`TestRF022_*` nuevas en PASS (no SKIP); `golangci-lint` v2.13.2 sin hallazgos
    (también con `--build-tags=integration`; se añadió a `backend/.golangci.yml` una exclusión de `misspell`
    para el client_id `inventario`); Vitest de Contabilidad 150/150, lint y typecheck limpios;
    `traceability.py --check` al día. E2E: stack `make up-app` levantado (la API arrancó leyendo los clientes
    de la tabla) y `control-de-acceso.spec.ts` + `autorizacion-oauth.spec.ts` 19/19 en verde, pero solo tras
    `ALTER TABLE users ALTER COLUMN organization_id SET DEFAULT` en la base local del stack: el
    `seedAdmin()` de `e2e/support/accounts.ts` inserta por SQL sin empresa y falla con `NOT NULL` desde R2
    (fuera de mis superficies; hay que corregirlo antes de la suite completa). Ese DEFAULT quedó en el
    volumen local de Postgres del stack (se conserva al hacer `make down`).

- 2026-10-19 (R4, escrito por el ejecutor, sin marcar; revisión y commit del orquestador):
  - API (OpenAPI primero, `make gen`): `GET /admin/organizations` (id, slug, name, `operator` = empresa `hub`);
    `organizationId` obligatorio en `GET /admin/users` y `GET /admin/applications` (la API filtra: `ListInput.OrganizationID`,
    `ListRoleGridApplications` con `WHERE organization_id`); `organizationId` obligatorio en el alta de empleados (se retira el valor
    transitorio de R2; falta → 400, empresa desconocida (FK) → 400); `GET /me/applications` (consulta `ListUserApplications`:
    aplicaciones de la empresa del usuario con al menos un rol; URL = `allowed_origin`; vacía para el admin global).
    Decisión: crear/editar/borrar rol no recibe parámetro de empresa, porque la aplicación de la ruta pertenece a una sola empresa.
  - `ErrCrossOrganizationRole` ahora vive en `admin` y `store` lo alias-a; `ReplaceRoles` y el alta responden 400
    (`role-organization-mismatch`) y auditan `role_assignment_rejected` con `reason=organization_mismatch` en una transacción
    aparte (la principal hace rollback: el usuario conserva sus roles y la invitación no crea cuenta). Handlers compuestos en
    `Server.Routes()` y probados por el router (`organizations_test.go`, `organizations_integration_test.go` con Postgres real).
  - Consola: `company.tsx` (CompanyProvider/CompanyScope/selector; selección por visor en localStorage con try/catch; por defecto la
    primera empresa no operadora; la página se remonta al cambiar de empresa). Usuarios, Roles e Inicio acotados; `UserDrawer` ofrece
    solo roles de la empresa del usuario y, al invitar, pide la empresa (prefijada) y descarta los roles al cambiarla. El bloque
    «Aplicaciones conectadas» sale de `/me/applications` (oculto si está vacío). Tras el login: admin → `/inicio`; una aplicación →
    redirección a ella; varias o ninguna → `/me`.
  - RED/GREEN: pruebas escritas junto con la implementación; RED comprobado por mutación (desactivar la rama de auditoría/filtro
    en admin, employee y store hizo fallar `TestRF022_RolDeOtraEmpresa*`, `...Consola...`, `...AsignarRol...`, `...Invitar...`; quitar
    `organizationId` de `listApplications` y la redirección única hizo fallar 5 pruebas de Vitest); restauradas y en verde.
  - Pruebas existentes ajustadas: peticiones a listados con `organizationId` (un parámetro obligatorio ausente da 400 antes de
    evaluar la autenticación, porque lo valida el binding generado); altas con empresa; Vitest de `App` con `stubApi`.
  - `specs/06-acceptance/empresas.feature` vuelve al repo con 8 escenarios (grilla, usuarios, roles ofrecidos, rechazo del backend,
    Contabilidad S.A. entra, Empresa B rechazada en Contabilidad, sin rol rechazado, lista tras el login). Quedan en el scratchpad
    para R5: «Un usuario de Empresa B entra a Inventario», «Con una sola aplicación el login lleva directamente a ella» y «El token
    de Inventario contiene solo roles de Inventario». `e2e/tests/empresas.spec.ts` (8 pruebas); `seedCompanyUser` en
    `e2e/support/accounts.ts`; `traceability.py --check` al día.
  - E2E: `demo.spec.ts` ahora espera la redirección a Contabilidad tras el login; `roles-y-permisos` usa un admin de Contabilidad
    como titular del rol (un admin global del `hub` ya no puede tener roles de Contabilidad); `manual/capturas.ts` manda la empresa.
  - Verificación: `make gen` limpio; `make test-go` y `go test -race -short` verdes; `make test-integration` con Postgres desechable
    verde (cobertura 81.8 %; `-run TestRF02[12]` con 46 PASS y 0 SKIP); `golangci-lint` v2.13.2 sin hallazgos (también con
    `--build-tags=integration`); front: lint y typecheck limpios, Vitest 285/285; `make e2e` completo 71/71 (incluye empresas,
    roles-y-permisos, control-de-acceso y autorizacion-oauth).
  - Dudas: (1) el escenario «Tras el login se listan…» choca con la regla de una sola aplicación (el usuario cae directo en
    Contabilidad); la E2E comprueba ambas cosas (redirección y luego el menú en `/me`). (2) El menú «Aplicaciones conectadas» se
    oculta en móvil (CSS previo). (3) `docs/manuales/*` y capturas no se tocaron (R6).
  - Ajuste posterior a la revisión: `organizationId` pasó a opcional en OpenAPI (tipo string, la descripción dice que el servidor lo
    exige) en `GET /admin/users` y `GET /admin/applications`; el handler lo valida después del wrapper admin (401, luego 403, luego
    400 si falta o es inválido). Se revirtieron los ajustes de pruebas que solo existían por el 400 previo a la autenticación y se
    añadió `TestRF022_AutenticacionYRolPrecedenAlParametroDeEmpresa`. Reverificado: `make gen`, `make test-go`, golangci-lint (ambos
    tags), Vitest 285/285, lint/typecheck, integración (128 PASS, 0 SKIP en api/auth/store), `traceability.py --check` y `make e2e` 71/71.
- 2026-10-19, revisión nativa de R3 (el rango R1–R3 completo excedía el presupuesto de contexto; se revisó
  solo R3 con `--base-ref` en el commit de R2; aprobada). Avisos para R6: (a) la migración 000011 se editó
  (`FOR SHARE`) después de aplicarse en la base local del stack: golang-migrate no la reaplica; en CI y bases
  nuevas no afecta; actualizar la base local al final; (b) la prueba de `backend/internal/api/oauth_test.go`
  sobre redirección a un URI ajeno choca antes con la guarda de parámetros duplicados y no prueba lo que dice.
- 2026-10-19, revisión de R4 (sesión principal): `/me/applications` exige sesión y la consulta solo devuelve
  aplicaciones de la propia empresa con rol del usuario. Pedido y verificado el orden 401/403 antes que 400.
  Comportamiento aceptado: un usuario con una sola aplicación va directo a ella y solo ve la lista si vuelve
  al Hub.

## Próximo paso

R5 (Inventario: app mínima, Nginx, build en `web`, E2E de los escenarios de Inventario) y R6 (documentación
y cierre).
