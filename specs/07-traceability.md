# 07 — Matriz de trazabilidad

> **Archivo generado.** No editar a mano.
> `python3 scripts/traceability.py` lo regenera; el job `spec-drift` de CI
> falla si el resultado difiere de lo commiteado.

Un requisito sin prueba que lo verifique se considera no implementado. Esta
tabla existe para que esa afirmación sea comprobable de un vistazo y no una
declaración de buenas intenciones.

| Requisito | Título | Pri | Operaciones de la API | Escenarios | Pruebas Go | E2E | Controles CI | Estado |
|---|---|---|---|---|---|---|---|---|
| **RF-001** | Alta de empleados por administración | P0 | `createEmployee`, `resendInvitation` | 4 | 35 | 4 | — | ✅ completo |
| **RF-002** | Aceptación de invitación y verificación de correo | P0 | `acceptInvitation` | 4 | 16 | 3 | — | ✅ completo |
| **RF-003** | Inicio de sesión | P0 | `login` | 2 | 14 | 2 | — | ✅ completo |
| **RF-004** | Emisión y validación de JWT | P0 | `getJwks` | 1 | 8 | 1 | — | ✅ completo |
| **RF-005** | Renovación de sesión | P0 | `refreshSession` | 1 | 7 | 1 | — | ✅ completo |
| **RF-006** | Detección de reuso de refresh token | P0 | — | 1 | 5 | 1 | — | ✅ completo |
| **RF-007** | Cierre de sesión | P0 | `logout` | 1 | 9 | 1 | — | ✅ completo |
| **RF-008** | Perfil propio | P0 | `getCurrentUser`, `updateCurrentUser` | 2 | 7 | 2 | — | ✅ completo |
| **RF-009** | Control de acceso por roles | P0 | — | 5 | 17 | 5 | — | ✅ completo |
| **RF-010** | Administración de usuarios | P0 | `listUsers`, `getUser`, `updateUser` | 3 | 13 | 3 | — | ✅ completo |
| **RF-011** | Registro de auditoría | P0 | `listAuditLog` | 1 | 9 | 1 | — | ✅ completo |
| **RF-012** | Notificaciones asíncronas | P0 | — | 1 | 25 | — | — | ✅ completo |
| **RF-013** | Segundo factor obligatorio por correo | P1 | — | 1 | 5 | — | — | ✅ completo |
| **RF-014** | Inicio de sesión con segundo factor | P1 | `verifyMfa`, `resendMfaCode` | 5 | 36 | 4 | — | ✅ completo |
| **RF-015** | Restablecimiento de contraseña | P1 | `requestPasswordReset`, `confirmPasswordReset` | 3 | 33 | 3 | — | ✅ completo |
| **RF-016** | Sesiones activas | P1 | `listSessions`, `revokeSession` | 1 | 8 | 1 | — | ✅ completo |
| **RF-017** | Bloqueo por fuerza bruta | P1 | — | 4 | 23 | 1 | — | ✅ completo |
| **RF-018** | Claves de servicio | P2 | `listServiceKeys`, `createServiceKey`, `revokeServiceKey` | 5 | 29 | 5 | — | ✅ completo |
| **RF-019** | Exportación del audit log | P2 | `exportAuditLog` | 2 | 13 | 2 | — | ✅ completo |
| **RF-020** | Autorización de aplicaciones cliente | P1 | `authorizeClient`, `exchangeAuthorizationCode` | 13 | 34 | 8 | — | ✅ completo |
| **RF-021** | Roles y permisos configurables por aplicación | P1 | `listApplications`, `listApplicationRoles`, `createApplicationRole`, `updateApplicationRole`, `deleteApplicationRole` | 9 | 31 | 6 | — | ✅ completo |
| **RF-022** | Empresas y aislamiento de roles | P1 | `listMyApplications`, `listOrganizations` | 9 | 34 | 8 | — | ✅ completo |
| **RNF-001** | Contenerización total | P0 | — | 1 | — | — | `ci.yml#e2e`, `.github/actions/stack-up/action.yml` | ✅ completo |
| **RNF-002** | Pipeline de ciclo completo | P0 | — | 1 | — | — | `ci.yml#secrets`, `ci.yml#sast`, `ci.yml#sca`, `ci.yml#image`, `ci.yml#dast` | ✅ completo |
| **RNF-003** | Cero secretos en el repositorio | P0 | — | — | 10 | — | `ci.yml#secrets` | ✅ completo |
| **RNF-004** | Imágenes sin vulnerabilidades corregibles | P0 | — | — | 3 | — | `ci.yml#image`, `release.yml#gate` | ✅ completo |
| **RNF-005** | Cobertura de pruebas ≥ 70 % | P0 | — | — | 25 | — | `ci.yml#test-integration` | ✅ completo |
| **RNF-006** | Procedencia verificable | P0 | — | 1 | — | — | `release.yml#publish` | ✅ completo |
| **RNF-007** | Observabilidad | P0 | — | — | 5 | 1 | — | ✅ completo |
| **RNF-008** | Contenedores endurecidos | P0 | — | 1 | — | — | `docker-compose.yml#api`, `docker-compose.yml#worker`, `docker-compose.yml#web`, `backend/Dockerfile`, `frontend/Dockerfile`, `ci.yml#iac-scan`, `ci.yml#lint` | ✅ completo |
| **RNF-009** | Cabeceras de seguridad | P0 | — | — | — | 6 | `ci.yml#dast` | ✅ completo |
| **RNF-010** | Latencia | P1 | — | 1 | — | — | `ci.yml#e2e`, `Makefile#load-test` | ✅ completo |
| **RNF-011** | Los specs son la fuente de verdad | P0 | — | — | 5 | — | `ci.yml#spec-drift` | ✅ completo |
| **RNF-012** | Logs sin datos sensibles | P0 | — | — | 5 | 1 | — | ✅ completo |

**Resumen:** 34 requisitos · 34 completos · 0 parciales · 0 sin cubrir · 0 diferidos (2026-10-13).

Leyenda de estado: *completo* = verificado por al menos dos de las
cuatro columnas de evidencia (Escenarios, Pruebas Go, E2E, Controles CI) · *parcial* = una sola ·
*sin cubrir* = ninguna · *diferido (2026-10-13)* = planificado para el 2026-10-13 por decisión del usuario (2026-10-12).

*Controles CI* lista los jobs, pasos o ajustes que hacen cumplir el requisito de forma
automática (`archivo#sección`). Se declaran en `specs/07-controls.yaml` y `--check` falla
si alguno apunta a algo que no existe; varios controles en la misma celda cuentan como
una sola fuente de evidencia.

## Escenarios de aceptación por requisito

- **RF-001**
  - registro-y-verificacion.feature — Alta administrativa con datos válidos
  - registro-y-verificacion.feature — El autorregistro público no está disponible
  - registro-y-verificacion.feature — Un correo no puede darse de alta dos veces
  - registro-y-verificacion.feature — Un admin reenvía una invitación pendiente
- **RF-002**
  - registro-y-verificacion.feature — Aceptar la invitación verifica el correo y fija la contraseña
  - registro-y-verificacion.feature — La invitación es de un solo uso
  - registro-y-verificacion.feature — Una cuenta que no aceptó la invitación no puede iniciar sesión
  - registro-y-verificacion.feature — Un admin reenvía una invitación pendiente
- **RF-003**
  - autenticacion.feature — Todo inicio de sesión exige MFA por correo
  - autenticacion.feature — Contraseña incorrecta
- **RF-004**
  - autenticacion.feature — Un token con el algoritmo alterado se rechaza
- **RF-005**
  - rotacion-de-sesion.feature — La renovación rota la cookie refresh
- **RF-006**
  - rotacion-de-sesion.feature — Reutilizar un refresh token rotado revoca toda la familia
- **RF-007**
  - rotacion-de-sesion.feature — Cierre de sesión
- **RF-008**
  - perfil-propio.feature — Consultar el perfil propio devuelve al usuario autenticado
  - perfil-propio.feature — Actualizar el nombre para mostrar
- **RF-009**
  - autorizacion-oauth.feature — El token contiene solo los roles de Contabilidad
  - control-de-acceso.feature — Un usuario corriente no accede a la administración
  - control-de-acceso.feature — Añadir el rol admin al token no concede privilegios
  - control-de-acceso.feature — Un empleado sin rol de aplicación no accede a Contabilidad
  - control-de-acceso.feature — El rol base "user" no se puede quitar
- **RF-010**
  - control-de-acceso.feature — Un administrador deshabilita una cuenta
  - control-de-acceso.feature — Un administrador no puede deshabilitarse a sí mismo
  - control-de-acceso.feature — Un administrador no puede asignarse roles a sí mismo
- **RF-011**
  - control-de-acceso.feature — El registro de auditoría no se puede alterar
- **RF-012**
  - registro-y-verificacion.feature — Alta administrativa con datos válidos
- **RF-013**
  - autenticacion.feature — Todo inicio de sesión exige MFA por correo
- **RF-014**
  - autenticacion.feature — Todo inicio de sesión exige MFA por correo
  - autenticacion.feature — Reenviar el código invalida el anterior
  - autenticacion.feature — El reenvío respeta la ventana mínima
  - autenticacion.feature — Agotar los intentos MFA anula el desafío
  - autenticacion.feature — Adivinar códigos MFA en desafíos sucesivos bloquea la cuenta
- **RF-015**
  - restablecimiento-contrasena.feature — La solicitud no enumera cuentas
  - restablecimiento-contrasena.feature — Restablecer revoca las sesiones anteriores
  - restablecimiento-contrasena.feature — Restablecer desbloquea la cuenta sin que un fallo la vuelva a bloquear
- **RF-016**
  - rotacion-de-sesion.feature — Revocar una sesión concreta desde otro dispositivo
- **RF-017**
  - autenticacion.feature — Agotar los intentos MFA anula el desafío
  - autenticacion.feature — Adivinar códigos MFA en desafíos sucesivos bloquea la cuenta
  - autenticacion.feature — Bloqueo tras intentos fallidos repetidos
  - restablecimiento-contrasena.feature — Restablecer desbloquea la cuenta sin que un fallo la vuelva a bloquear
- **RF-018**
  - claves-de-servicio.feature — Admin emite una clave, la usa para leer el audit log y la revoca
  - claves-de-servicio.feature — El listado de claves no expone el secreto
  - claves-de-servicio.feature — Una clave de servicio solo abre las operaciones que declaran su alcance
  - claves-de-servicio.feature — Una clave inválida, expirada o revocada recibe la misma respuesta
  - claves-de-servicio.feature — Solo un administrador gestiona claves de servicio
- **RF-019**
  - control-de-acceso.feature — Un administrador exporta el registro de auditoría filtrado
  - control-de-acceso.feature — Un usuario corriente no puede exportar el registro de auditoría
- **RF-020**
  - autorizacion-oauth.feature — Flujo OAuth correcto con PKCE S256
  - autorizacion-oauth.feature — Un segundo acceso usa la sesión SSO del Hub
  - autorizacion-oauth.feature — Una URI de retorno distinta se rechaza
  - autorizacion-oauth.feature — El estado es obligatorio
  - autorizacion-oauth.feature — PKCE plain se rechaza
  - autorizacion-oauth.feature — Un código de autorización no puede reutilizarse
  - autorizacion-oauth.feature — El token contiene solo los roles de Contabilidad
  - autorizacion-oauth.feature — Sin sesión en el Hub se pasa por el login y se vuelve a la autorización
  - autorizacion-oauth.feature — Un "continue" absoluto o externo no provoca una redirección abierta
  - control-de-acceso.feature — Un empleado sin rol de aplicación no accede a Contabilidad
  - empresas.feature — Un usuario de Contabilidad S.A. entra a Contabilidad
  - empresas.feature — Un usuario de Empresa B que abre Contabilidad es rechazado
  - empresas.feature — Un usuario sin rol en una aplicación de su empresa es rechazado
- **RF-021**
  - empresas.feature — La grilla de roles muestra solo las aplicaciones y roles de la empresa seleccionada
  - empresas.feature — Al asignar un rol solo se ofrecen roles de la empresa del usuario
  - empresas.feature — El backend rechaza asignar un rol de otra empresa
  - roles-y-permisos.feature — Admin crea auditor de Contabilidad con capacidades acotadas
  - roles-y-permisos.feature — Se rechaza crear un rol de directorio desde la grilla
  - roles-y-permisos.feature — Un administrador no edita permisos de un rol que posee
  - roles-y-permisos.feature — Se rechaza un permiso de otra aplicación
  - roles-y-permisos.feature — No se elimina un rol asignado
  - roles-y-permisos.feature — Cada cambio de rol queda auditado
- **RF-022**
  - control-de-acceso.feature — Un empleado sin rol de aplicación no accede a Contabilidad
  - empresas.feature — La grilla de roles muestra solo las aplicaciones y roles de la empresa seleccionada
  - empresas.feature — El listado de usuarios muestra solo los usuarios de la empresa seleccionada
  - empresas.feature — Al asignar un rol solo se ofrecen roles de la empresa del usuario
  - empresas.feature — El backend rechaza asignar un rol de otra empresa
  - empresas.feature — Un usuario de Contabilidad S.A. entra a Contabilidad
  - empresas.feature — Un usuario de Empresa B que abre Contabilidad es rechazado
  - empresas.feature — Un usuario sin rol en una aplicación de su empresa es rechazado
  - empresas.feature — Tras el login se listan solo las aplicaciones de la empresa del usuario
- **RNF-001**
  - requisitos-no-funcionales.feature — Cada servicio corre en su propio contenedor y todo arranca con un solo docker compose up
- **RNF-002**
  - requisitos-no-funcionales.feature — El pipeline recorre las etapas de seguridad y sus gates rompen la build
- **RNF-006**
  - requisitos-no-funcionales.feature — Una imagen se firma, se atesta y se verifica antes de mover latest
- **RNF-008**
  - requisitos-no-funcionales.feature — Los servicios propios corren endurecidos
- **RNF-010**
  - requisitos-no-funcionales.feature — El p95 de los endpoints de autenticación se mantiene por debajo de 300 ms
