# 07 — Matriz de trazabilidad

> **Archivo generado.** No editar a mano.
> `python3 scripts/traceability.py` lo regenera; el job `spec-drift` de CI
> falla si el resultado difiere de lo commiteado.

Un requisito sin prueba que lo verifique se considera no implementado. Esta
tabla existe para que esa afirmación sea comprobable de un vistazo y no una
declaración de buenas intenciones.

| Requisito | Título | Pri | Operaciones de la API | Escenarios | Pruebas Go | E2E | Estado |
|---|---|---|---|---|---|---|---|
| **RF-001** | Registro de cuenta | P0 | `register` | 3 | 6 | — | ✅ completo |
| **RF-002** | Verificación de correo | P0 | `verifyEmail` | 2 | — | — | 🟡 parcial |
| **RF-003** | Inicio de sesión | P0 | `login` | 3 | — | — | 🟡 parcial |
| **RF-004** | Emisión y validación de JWT | P0 | `getJwks` | 2 | 7 | — | ✅ completo |
| **RF-005** | Renovación de sesión | P0 | `refreshSession` | 1 | — | — | 🟡 parcial |
| **RF-006** | Detección de reuso de refresh token | P0 | — | 1 | — | — | 🟡 parcial |
| **RF-007** | Cierre de sesión | P0 | `logout` | 1 | — | — | 🟡 parcial |
| **RF-008** | Perfil propio | P0 | `getCurrentUser`, `updateCurrentUser` | — | — | — | 🔴 sin cubrir |
| **RF-009** | Control de acceso por roles | P0 | — | 2 | — | — | 🟡 parcial |
| **RF-010** | Administración de usuarios | P0 | `listUsers`, `getUser`, `updateUser` | 2 | — | — | 🟡 parcial |
| **RF-011** | Registro de auditoría | P0 | `listAuditLog` | 1 | 3 | — | ✅ completo |
| **RF-012** | Notificaciones asíncronas | P0 | — | — | 3 | — | 🟡 parcial |
| **RF-013** | Alta de segundo factor (TOTP) | P1 | `enrollMfa`, `activateMfa`, `disableMfa` | 1 | — | — | 🟡 parcial |
| **RF-014** | Inicio de sesión con segundo factor | P1 | `verifyMfa` | 1 | — | — | 🟡 parcial |
| **RF-015** | Restablecimiento de contraseña | P1 | `requestPasswordReset`, `confirmPasswordReset` | — | — | — | 🔴 sin cubrir |
| **RF-016** | Sesiones activas | P1 | `listSessions`, `revokeSession` | 1 | — | — | 🟡 parcial |
| **RF-017** | Bloqueo por fuerza bruta | P1 | — | 1 | 1 | — | ✅ completo |
| **RF-018** | Claves de servicio | P2 | — | — | — | — | 🔴 sin cubrir |
| **RF-019** | Exportación del audit log | P2 | — | — | — | — | 🔴 sin cubrir |
| **RNF-001** | Contenerización total | P0 | — | — | — | — | 🔴 sin cubrir |
| **RNF-002** | Pipeline de ciclo completo | P0 | — | — | — | — | 🔴 sin cubrir |
| **RNF-003** | Cero secretos en el repositorio | P0 | — | — | 4 | — | 🟡 parcial |
| **RNF-004** | Imágenes sin vulnerabilidades corregibles | P0 | — | — | — | — | 🔴 sin cubrir |
| **RNF-005** | Cobertura de pruebas ≥ 70 % | P0 | — | — | — | — | 🔴 sin cubrir |
| **RNF-006** | Procedencia verificable | P0 | — | — | — | — | 🔴 sin cubrir |
| **RNF-007** | Observabilidad | P0 | — | — | — | — | 🔴 sin cubrir |
| **RNF-008** | Contenedores endurecidos | P0 | — | — | — | — | 🔴 sin cubrir |
| **RNF-009** | Cabeceras de seguridad | P0 | — | — | — | — | 🔴 sin cubrir |
| **RNF-010** | Latencia | P1 | — | — | — | — | 🔴 sin cubrir |
| **RNF-011** | Los specs son la fuente de verdad | P0 | — | — | 3 | — | 🟡 parcial |
| **RNF-012** | Logs sin datos sensibles | P0 | — | — | 2 | — | 🟡 parcial |

**Resumen:** 31 requisitos · 4 completos · 14 parciales · 13 sin cubrir.

Leyenda de estado: *completo* = verificado por al menos dos de las tres
columnas de prueba · *parcial* = una sola · *sin cubrir* = ninguna.

## Escenarios de aceptación por requisito

- **RF-001**
  - registro-y-verificacion.feature — Registro con datos válidos
  - registro-y-verificacion.feature — Contraseñas que no cumplen la política
  - registro-y-verificacion.feature — El registro no revela qué correos existen
- **RF-002**
  - registro-y-verificacion.feature — Verificación del correo con el enlace recibido
  - registro-y-verificacion.feature — El enlace de verificación es de un solo uso
- **RF-003**
  - autenticacion.feature — Inicio de sesión correcto
  - autenticacion.feature — Contraseña incorrecta
  - registro-y-verificacion.feature — Una cuenta sin verificar no puede iniciar sesión
- **RF-004**
  - autenticacion.feature — Inicio de sesión correcto
  - autenticacion.feature — Un token con el algoritmo alterado se rechaza
- **RF-005**
  - rotacion-de-sesion.feature — La renovación rota el refresh token
- **RF-006**
  - rotacion-de-sesion.feature — Reutilizar un refresh token rotado revoca toda la familia
- **RF-007**
  - rotacion-de-sesion.feature — Cierre de sesión
- **RF-009**
  - control-de-acceso.feature — Un usuario corriente no accede a la administración
  - control-de-acceso.feature — Añadir el rol admin al token no concede privilegios
- **RF-010**
  - control-de-acceso.feature — Un administrador deshabilita una cuenta
  - control-de-acceso.feature — Un administrador no puede deshabilitarse a sí mismo
- **RF-011**
  - control-de-acceso.feature — El registro de auditoría no se puede alterar
- **RF-013**
  - autenticacion.feature — Inicio de sesión con segundo factor activo
- **RF-014**
  - autenticacion.feature — Inicio de sesión con segundo factor activo
- **RF-016**
  - rotacion-de-sesion.feature — Revocar una sesión concreta desde otro dispositivo
- **RF-017**
  - autenticacion.feature — Bloqueo tras intentos fallidos repetidos
