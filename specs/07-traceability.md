# 07 — Matriz de trazabilidad

> **Archivo generado.** No editar a mano.
> `python3 scripts/traceability.py` lo regenera; el job `spec-drift` de CI
> falla si el resultado difiere de lo commiteado.

Un requisito sin prueba que lo verifique se considera no implementado. Esta
tabla existe para que esa afirmación sea comprobable de un vistazo y no una
declaración de buenas intenciones.

| Requisito | Título | Pri | Operaciones de la API | Escenarios | Pruebas Go | E2E | Estado |
|---|---|---|---|---|---|---|---|
| **RF-001** | Alta de empleados por administración | P0 | `createEmployee`, `resendInvitation` | 4 | 34 | — | ✅ completo |
| **RF-002** | Aceptación de invitación y verificación de correo | P0 | `acceptInvitation` | 4 | 16 | — | ✅ completo |
| **RF-003** | Inicio de sesión | P0 | `login` | 2 | 9 | — | ✅ completo |
| **RF-004** | Emisión y validación de JWT | P0 | `getJwks` | 1 | 8 | — | ✅ completo |
| **RF-005** | Renovación de sesión | P0 | `refreshSession` | 1 | 7 | — | ✅ completo |
| **RF-006** | Detección de reuso de refresh token | P0 | — | 1 | 5 | — | ✅ completo |
| **RF-007** | Cierre de sesión | P0 | `logout` | 1 | 8 | — | ✅ completo |
| **RF-008** | Perfil propio | P0 | `getCurrentUser`, `updateCurrentUser` | — | 7 | — | 🟡 parcial |
| **RF-009** | Control de acceso por roles | P0 | — | 5 | 20 | — | ✅ completo |
| **RF-010** | Administración de usuarios | P0 | `listUsers`, `getUser`, `updateUser` | 3 | 12 | — | ✅ completo |
| **RF-011** | Registro de auditoría | P0 | `listAuditLog` | 1 | 9 | — | ✅ completo |
| **RF-012** | Notificaciones asíncronas | P0 | — | 1 | 9 | — | ✅ completo |
| **RF-013** | Segundo factor obligatorio por correo | P1 | — | 1 | 5 | — | ✅ completo |
| **RF-014** | Inicio de sesión con segundo factor | P1 | `verifyMfa`, `resendMfaCode` | 5 | 33 | — | ✅ completo |
| **RF-015** | Restablecimiento de contraseña | P1 | `requestPasswordReset`, `confirmPasswordReset` | 3 | 32 | — | ✅ completo |
| **RF-016** | Sesiones activas | P1 | `listSessions`, `revokeSession` | 1 | 8 | — | ✅ completo |
| **RF-017** | Bloqueo por fuerza bruta | P1 | — | 4 | 21 | — | ✅ completo |
| **RF-018** | Claves de servicio | P2 | — | — | — | — | ⏳ diferido (semana 3) |
| **RF-019** | Exportación del audit log | P2 | — | — | — | — | ⏳ diferido (semana 3) |
| **RF-020** | Autorización de aplicaciones cliente | P1 | `authorizeClient`, `exchangeAuthorizationCode` | 10 | — | — | ⏳ diferido (semana 3) |
| **RNF-001** | Contenerización total | P0 | — | — | — | — | 🔴 sin cubrir |
| **RNF-002** | Pipeline de ciclo completo | P0 | — | — | — | — | 🔴 sin cubrir |
| **RNF-003** | Cero secretos en el repositorio | P0 | — | — | 4 | — | 🟡 parcial |
| **RNF-004** | Imágenes sin vulnerabilidades corregibles | P0 | — | — | 3 | — | 🟡 parcial |
| **RNF-005** | Cobertura de pruebas ≥ 70 % | P0 | — | — | 25 | — | 🟡 parcial |
| **RNF-006** | Procedencia verificable | P0 | — | — | — | — | 🔴 sin cubrir |
| **RNF-007** | Observabilidad | P0 | — | — | — | — | 🔴 sin cubrir |
| **RNF-008** | Contenedores endurecidos | P0 | — | — | — | — | 🔴 sin cubrir |
| **RNF-009** | Cabeceras de seguridad | P0 | — | — | — | — | 🔴 sin cubrir |
| **RNF-010** | Latencia | P1 | — | — | — | — | 🔴 sin cubrir |
| **RNF-011** | Los specs son la fuente de verdad | P0 | — | — | 4 | — | 🟡 parcial |
| **RNF-012** | Logs sin datos sensibles | P0 | — | — | 3 | — | 🟡 parcial |

**Resumen:** 32 requisitos · 16 completos · 6 parciales · 7 sin cubrir · 3 diferidos (semana 3).

Leyenda de estado: *completo* = verificado por al menos dos de las tres
columnas de prueba · *parcial* = una sola · *sin cubrir* = ninguna · *diferido (semana 3)* = backlog de semana 3 por decisión Q14.

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
