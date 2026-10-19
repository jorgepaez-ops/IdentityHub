# Roles separados por empresa

## Objetivo

Que el Hub sirva a varias empresas sin mezclar sus datos ni sus roles: cada aplicación y cada usuario
pertenecen a una sola empresa, a un usuario solo se le asignan roles de aplicaciones de su empresa, tras
iniciar sesión el Hub lo lleva a las aplicaciones de su empresa, y el Hub rechaza el acceso a una
aplicación de otra empresa.

## Problema y por qué

En la revisión final, el profesor observó que la grilla de roles solo servía a Contabilidad y preguntó qué
pasa con una empresa tercera. Mapa del estado actual (2026-10-19, exploración con evidencia path:line):

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

- [ ] **R1. Especificación.** ADR 0015, requisitos, modelo de dominio y escenarios `empresas.feature`.
- [ ] **R2. Datos.** Migración de empresas y semillas; consultas sqlc; trigger o validación de que un rol
  asignado pertenece a la empresa del usuario; pruebas de integración.
- [ ] **R3. OAuth multi-cliente.** Clientes desde `applications`; `redirect_uri`, CORS y `aud` por cliente;
  rechazo en `/oauth/authorize` por empresa o sin rol; pruebas.
- [ ] **R4. Administración y enrutamiento.** API: empresa del usuario, roles asignables, aplicaciones del
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

## Próximo paso

R1 (writer delegado).
