# 01 — Requisitos

Cada requisito tiene un identificador estable. Ese identificador viaja por todo el proyecto:
aparece en el `operationId` de la API, en el nombre de las pruebas (`TestRF003_...`), en los
escenarios Gherkin y en la matriz de `07-traceability.md`. **Si un requisito no es trazable
hasta una prueba que lo verifique, se considera no implementado.**

Prioridad: **P0** núcleo no negociable · **P1** segundo anillo · **P2** si sobra tiempo.

---

## Requisitos funcionales

### RF-001 — Registro de cuenta · P0
Una persona puede crear una cuenta con correo y contraseña.
- La contraseña se almacena con Argon2id; jamás en claro ni con hash rápido.
- Se rechaza una contraseña de menos de 12 caracteres.
- Un correo ya registrado devuelve `409`, **con el mismo tiempo de respuesta** que uno nuevo
  (mitiga enumeración de usuarios).
- La cuenta nace en estado `pending_verification` y no puede iniciar sesión.
- **Aceptación:** registrarse devuelve `201`; un segundo intento con el mismo correo devuelve
  `409`; la fila en `users` tiene un hash con prefijo `$argon2id$`.

### RF-002 — Verificación de correo · P0
La cuenta se activa mediante un enlace enviado por correo.
- El token es de 32 bytes de `crypto/rand`, se guarda hasheado y expira en 24 h.
- Es de un solo uso; un segundo intento devuelve `410`.
- **Aceptación:** tras registrarse, el correo llega a Mailpit; abrir el enlace pasa la cuenta a
  `active`; reutilizar el enlace falla.

### RF-003 — Inicio de sesión · P0
Una cuenta activa obtiene un par de tokens presentando correo y contraseña.
- Credenciales inválidas devuelven `401` con un mensaje genérico, sin revelar si el correo
  existe.
- Una cuenta en `pending_verification`, `locked` o `disabled` no puede iniciar sesión.
- **Aceptación:** credenciales correctas devuelven `200` con `access_token` y `refresh_token`;
  contraseña incorrecta devuelve `401`; el intento queda en el audit log.

### RF-004 — Emisión y validación de JWT · P0
La API emite JWT firmados con Ed25519 y publica su clave pública.
- Claims: `iss`, `sub`, `aud`, `exp`, `iat`, `jti`, `roles`.
- Vigencia del access token: 15 minutos.
- `GET /.well-known/jwks.json` expone la clave pública; el `kid` del token coincide.
- La validación rechaza explícitamente `alg: none` y cualquier algoritmo distinto de `EdDSA`.
- **Aceptación:** el token decodifica con la clave del JWKS; un token con `alg` alterado se
  rechaza con `401`.

### RF-005 — Renovación de sesión · P0
Un refresh token válido produce un par nuevo y se invalida a sí mismo.
- El refresh token es opaco (no un JWT), de 32 bytes aleatorios, guardado como SHA-256.
- Rotación obligatoria: cada uso emite un refresh nuevo y marca el anterior como `rotated`.
- **Aceptación:** renovar devuelve tokens distintos; el refresh anterior ya no funciona.

### RF-006 — Detección de reuso de refresh token · P0
Usar un refresh token ya rotado se interpreta como robo de credencial.
- Revoca **toda la familia** de tokens descendientes de esa sesión.
- Emite un evento de auditoría `refresh_reuse_detected` y una notificación al titular.
- **Aceptación:** rotar, luego reintentar con el token viejo → `401` y todas las sesiones de esa
  familia quedan revocadas.

### RF-007 — Cierre de sesión · P0
El titular puede cerrar la sesión actual.
- Revoca el refresh token; el access token expira solo por su vigencia corta.
- **Aceptación:** tras cerrar sesión, el refresh devuelve `401`.

### RF-008 — Perfil propio · P0
El titular consulta y actualiza su nombre para mostrar.
- **Aceptación:** `GET /me` con token válido devuelve el usuario autenticado; sin token, `401`.

### RF-009 — Control de acceso por roles · P0
Los roles `admin` y `user` gobiernan el acceso a los recursos.
- El rol viaja en el claim `roles` y se verifica **en el servidor**, nunca confiando en el cliente.
- Un `user` que llama a un endpoint de administración recibe `403`.
- **Aceptación:** las rutas `/admin/*` responden `200` a un admin y `403` a un user.

### RF-010 — Administración de usuarios · P0
Un admin lista, busca, y habilita o deshabilita cuentas.
- No puede deshabilitarse a sí mismo (evita dejar el sistema sin administrador).
- **Aceptación:** un admin deshabilita a un usuario y ese usuario ya no puede iniciar sesión.

### RF-011 — Registro de auditoría · P0
Todo evento de seguridad queda registrado de forma inmutable.
- Eventos: registro, verificación, login exitoso/fallido, logout, rotación y reuso de refresh,
  cambio de contraseña, alta y baja de MFA, cambio de rol, bloqueo de cuenta.
- Cada entrada guarda actor, acción, recurso, IP, user-agent y marca temporal.
- La tabla es *append-only*: sin `UPDATE` ni `DELETE` concedidos al rol de la aplicación.
- **Aceptación:** un login fallido crea una fila; intentar modificarla con el usuario de la
  aplicación falla por permisos.

### RF-012 — Notificaciones asíncronas · P0
Los correos se envían fuera del ciclo de la petición HTTP.
- La API publica un mensaje en RabbitMQ; el worker lo consume y entrega por SMTP.
- Reintentos con retroceso exponencial; tras 3 fallos, el mensaje va a la *dead-letter queue*.
- **Aceptación:** registrarse encola un mensaje visible en RabbitMQ y el correo aparece en
  Mailpit; con el worker detenido, la petición HTTP sigue respondiendo en menos de 500 ms.

### RF-013 — Alta de segundo factor (TOTP) · P1
El titular activa TOTP y recibe códigos de recuperación.
- El secreto se entrega una sola vez como URI `otpauth://` y código QR.
- La activación exige confirmar un código válido.
- Se generan 10 códigos de recuperación de un solo uso, guardados hasheados.
- **Aceptación:** activar MFA y confirmar con un código correcto marca `mfa_enabled = true`.

### RF-014 — Inicio de sesión con segundo factor · P1
Con MFA activo, la contraseña no basta.
- El login devuelve `202` y un `mfa_token` de corta vida en lugar de los tokens de sesión.
- Se acepta un código TOTP o uno de recuperación (que se consume).
- **Aceptación:** login sin código devuelve `202`; con código válido devuelve `200` y los tokens.

### RF-015 — Restablecimiento de contraseña · P1
Quien olvida su contraseña la restablece por correo.
- La respuesta es siempre `202`, exista o no el correo (evita enumeración).
- Token de un solo uso con 1 h de vigencia; al usarlo se revocan todas las sesiones activas.
- **Aceptación:** completar el flujo permite entrar con la nueva contraseña y las sesiones
  anteriores dejan de funcionar.

### RF-016 — Sesiones activas · P1
El titular ve sus sesiones y puede revocarlas individualmente.
- Se muestran IP, user-agent, creación y último uso.
- **Aceptación:** revocar una sesión invalida su refresh token y deja el resto intactas.

### RF-017 — Bloqueo por fuerza bruta · P1
Tras 5 intentos fallidos en 15 minutos, la cuenta se bloquea 15 minutos.
- El bloqueo se registra en auditoría y se notifica al titular.
- **Aceptación:** seis intentos fallidos devuelven `423 Locked`; la contraseña correcta también
  falla mientras dure el bloqueo.

### RF-018 — Claves de servicio · P2
Un admin emite claves de API para integraciones máquina a máquina.

### RF-019 — Exportación del audit log · P2
Un admin exporta el registro de auditoría filtrado en CSV o JSON.

---

## Requisitos no funcionales

### RNF-001 — Contenerización total · P0
Cada servicio corre en su propio contenedor y el sistema entero se levanta con un solo
`docker compose up`. No se requiere ninguna dependencia instalada en el host más allá de Docker.

### RNF-002 — Pipeline de ciclo completo · P0
Un push ejecuta build → test → SAST → SCA → secretos → escaneo de imagen → SBOM → firma →
publicación → despliegue → DAST, sin pasos manuales. Los gates de seguridad **rompen la build**.

### RNF-003 — Cero secretos en el repositorio · P0
Ninguna credencial vive en el código. Gitleaks analiza el historial completo en cada push.
La configuración falla al arrancar si falta un secreto obligatorio, en lugar de usar un
valor por defecto inseguro.

### RNF-004 — Imágenes sin vulnerabilidades corregibles · P0
Las imágenes publicadas desde `main` no contienen CVE HIGH o CRITICAL con parche disponible.
Las excepciones se documentan en `security/.trivyignore` con justificación y fecha de expiración.

### RNF-005 — Cobertura de pruebas ≥ 70 % · P0
Medida sobre `backend/internal/`. El gate rompe la build por debajo del umbral.

### RNF-006 — Procedencia verificable · P0
Cada imagen se publica con un SBOM en CycloneDX y una firma Cosign *keyless*. El despliegue
verifica la firma antes de arrancar; si no valida, no despliega.

### RNF-007 — Observabilidad · P0
La API y el worker exponen `/metrics` en formato Prometheus y emiten logs JSON estructurados.
Grafana presenta un panel con la tasa de logins fallidos, la latencia p95, la profundidad de la
DLQ y los reusos de refresh detectados.

### RNF-008 — Contenedores endurecidos · P0
Todos los contenedores corren como usuario no-root, con sistema de archivos raíz en solo
lectura, todas las capacidades descartadas y `no-new-privileges`. Las imágenes base se fijan
por digest.

### RNF-009 — Cabeceras de seguridad · P0
Nginx emite `Content-Security-Policy`, `Strict-Transport-Security`, `X-Content-Type-Options`,
`X-Frame-Options` y `Referrer-Policy`. Las cookies de sesión son `HttpOnly`, `Secure` y
`SameSite=Strict`. `server_tokens off`.

### RNF-010 — Latencia · P1
El p95 de los endpoints de autenticación se mantiene por debajo de 300 ms en local, excluido
el coste deliberado de Argon2id.

### RNF-011 — Los specs son la fuente de verdad · P0
El código de contratos se genera desde `specs/`. El job `spec-drift` regenera en CI y falla si
el resultado difiere de lo commiteado.

### RNF-012 — Logs sin datos sensibles · P0
Los logs nunca contienen contraseñas, tokens, secretos TOTP ni códigos de recuperación. Los
identificadores de token se registran truncados a 8 caracteres.
