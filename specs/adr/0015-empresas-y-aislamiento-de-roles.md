# 0015 — Empresas y aislamiento de roles

Estado: aceptada · 2026-10-19 · Añade RF-022; enmienda RF-020, RF-021 y el modelo de dominio

## Contexto

En la revisión final, el profesor observó que la grilla de roles (ADR 0013) solo servía a Contabilidad
y preguntó qué ocurre con una empresa tercera. El estado actual explica la limitación:

- No existe la entidad empresa: `users` no tiene organización y `applications` (migración
  `000007`) contiene una sola fila, `contabilidad`.
- Los roles ya son por aplicación (`roles.application_id`, migración `000009`; nombre
  `<client_id>.<slug>`) y el token de una aplicación solo lleva sus roles, con `aud` igual a su
  `client_id`. Esa parte del aislamiento ya existe.
- El cliente OAuth es único y está fijo en el código (`backend/cmd/api/main.go:159`,
  `backend/internal/config/config.go:75`); RF-020 lo declara «único cliente». CORS, `redirect_uri` y
  `aud` se comparan contra ese cliente.
- Un administrador puede asignar a cualquier usuario roles de cualquier aplicación
  (`UserDrawer.tsx`), sin validar a qué empresa pertenece cada parte.
- Tras el login, el administrador va a `/inicio` y el resto a `/me`; el menú muestra «Contabilidad» a
  todos (`frontend/src/App.tsx:185`).
- `/oauth/authorize` no rechaza a un usuario sin rol en la aplicación: emite un token sin roles y la
  aplicación muestra «sin acceso» (`specs/06-acceptance/control-de-acceso.feature`).

- La consola del Hub mezcla empresas: la página de Roles y el listado de Usuarios no distinguen a qué
  empresa pertenece cada aplicación, rol o cuenta.

El Hub, tal como está, no impide que una segunda empresa vea o reciba roles de la primera.

## Decisión

Modelo de **empresa ligera**: una entidad `organizations` y una regla de pertenencia única, sin
administración delegada por empresa.

1. **Pertenencia única.** Cada aplicación y cada usuario pertenecen a exactamente una empresa
   (`applications.organization_id` y `users.organization_id`, ambos `NOT NULL`).
2. **Coherencia de roles.** Un rol de aplicación solo puede asignarse a un usuario de la misma empresa
   que la aplicación del rol. Lo impone el backend (rechazo con `400` y auditoría) y la base de datos
   (trigger o restricción), de modo que ninguna ruta de código ni consulta directa la evada. Los roles
   de directorio (`admin`, `user`) no pertenecen a ninguna aplicación y no se restringen.
3. **Clientes desde la base de datos.** Los clientes OAuth se cargan de `applications`, cada uno con su
   `redirect_uri`, su origen permitido y su `aud` (`client_id`). Dejan de ser un valor único fijo en
   configuración; `redirect_uri` sigue exigiendo coincidencia exacta y CORS sigue limitado a
   `/oauth/token` y `/.well-known/jwks.json`, ahora con el origen del cliente solicitado.
4. **Rechazo en `/oauth/authorize`.** Si la empresa del usuario difiere de la de la aplicación, o el
   usuario no tiene ningún rol en esa aplicación, el Hub rechaza la solicitud sin emitir código ni
   token y registra el rechazo en auditoría. Esto sustituye el comportamiento actual de emitir un
   token sin roles.
5. **Enrutamiento por empresa.** Tras el login, el Hub lista las aplicaciones de la empresa del usuario
   (aquellas en las que tiene al menos un rol) y, si hay exactamente una, lo lleva directo a ella. El
   menú deja de mostrar «Contabilidad» fijo.
6. **La consola separa las empresas (objetivo principal).** El administrador elige primero una empresa
   (selector o sección por empresa) y trabaja en una vista acotada a ella: la página de Roles lista
   solo las aplicaciones y roles de esa empresa (crear un rol nunca muestra roles de dos empresas
   juntos); el listado de Usuarios muestra solo sus usuarios; y al asignar un rol a un usuario solo se
   ofrecen roles de la empresa del usuario. Cambiar de empresa muestra otra vista sin nada de la
   anterior. El filtrado lo aplica también la API (parámetro de empresa en los listados de
   administración), no solo la interfaz.
7. **El administrador del Hub sigue siendo global.** Es el operador de la plataforma: puede cambiar
   entre empresas, pero en cada vista trabaja con una sola. No hay administrador por empresa.
8. **Empresas y aplicaciones por migración.** Se declaran en una migración, como hoy Contabilidad; la
   consola no ofrece alta de empresas ni de aplicaciones. Datos sembrados: «Contabilidad S.A.» (con
   Contabilidad y los usuarios existentes) y «Empresa B» (con la aplicación mínima Inventario, sus
   roles y permisos).

### Reglas de validación

| Situación | Resultado |
|---|---|
| Asignar un rol de la aplicación X a un usuario de otra empresa | `400`, evento `role_assignment_rejected` con el motivo de empresa distinta |
| `/oauth/authorize` de un usuario de otra empresa | rechazo, sin código, auditado |
| `/oauth/authorize` de un usuario de la empresa sin rol en la aplicación | rechazo, sin código, auditado |
| `/oauth/authorize` de un usuario con rol en la aplicación de su empresa | flujo actual (código, PKCE, token con `aud` y solo sus roles) |
| Usuario sin roles de aplicación | ve una lista vacía de aplicaciones |
| Grilla de roles o listado de usuarios con una empresa seleccionada | solo recursos de esa empresa |

## Alternativas consideradas

| Alternativa | Por qué se descarta |
|---|---|
| **Multi-tenencia completa** con administrador por empresa y auditoría separada | Exige delegar la autoridad, partir el audit log y probar el aislamiento entre administradores. El problema planteado (que una empresa no vea ni use roles de otra) se resuelve sin eso; queda como evolución. |
| **Solo aplicaciones**, sin entidad empresa (agrupar por lista de usuarios en cada aplicación) | No expresa «los usuarios de la empresa B»: la pertenencia quedaría repetida por aplicación, sin una invariante verificable y sin forma de decidir qué aplicaciones mostrar tras el login. |
| **Un realm por empresa**, como en Keycloak | Aísla del todo (usuarios, claves, sesiones), pero obliga a duplicar la configuración de seguridad por empresa. Es desproporcionado para una plataforma con un operador único; ADR 0002 ya descartó adoptar Keycloak. |

## Consecuencias

- Buenas: la consola ofrece una vista por empresa, sin mezclar aplicaciones, roles ni cuentas; el aislamiento queda garantizado por una invariante de datos, no solo por la interfaz; las
  aplicaciones nuevas se incorporan por migración sin tocar el código del flujo OAuth; Contabilidad
  conserva su comportamiento para sus usuarios.
- Malas: una persona que trabaja para dos empresas necesita dos cuentas (correo distinto, porque el
  correo es único); el administrador global ve todas las empresas, por lo que la auditoría sigue siendo
  única; empresas y aplicaciones no se gestionan desde la consola, de modo que una nueva empresa
  requiere una migración y un despliegue.
- La consola y la API de administración ganan un contexto de empresa: los listados de usuarios,
  aplicaciones y roles se filtran por ella y la interfaz exige elegir una antes de mostrarlos. Es
  más trabajo de consola (R4) y un paso extra para el operador.
- El rechazo en `/oauth/authorize` cambia el contrato observable de RF-020: un usuario sin rol en la
  aplicación ya no recibe un token vacío. Los escenarios existentes que lo describían se ajustan
  (`control-de-acceso.feature`).
- La aplicación demostrativa Inventario (`inventario.localhost:8080`) tiene su propio cliente, roles y
  permisos; no añade backend de negocio, igual que Contabilidad (ADR 0013).
- Cambiar la empresa de un usuario con roles de aplicación asignados queda fuera de alcance: se
  rechaza mientras existan roles de la empresa anterior.

## Escenarios de aceptación

Viven en `specs/06-acceptance/empresas.feature` y, cuando se implementen, cada uno tendrá su prueba
E2E en `e2e/tests/` (correspondencia uno a uno verificada por `scripts/traceability.py`).
