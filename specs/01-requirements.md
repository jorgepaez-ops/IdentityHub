# 01 — Requisitos

Cada requisito tiene un identificador estable. Ese identificador viaja por todo el proyecto:
aparece en el `operationId` de la API, en el nombre de las pruebas (`TestRF003_...`), en los
escenarios Gherkin y en la matriz de `07-traceability.md`. **Si un requisito no es trazable
hasta una prueba que lo verifique, se considera no implementado.**

Prioridad: **P0** núcleo no negociable · **P1** segundo anillo · **P2** si sobra tiempo.

---

## Requisitos funcionales

### RF-001 — Alta de empleados por administración · P0
Un administrador crea la cuenta de un empleado con correo, nombre y roles, sin conocer ni fijar
su contraseña.
- El alta pública está cerrada; solo un `admin` autenticado puede crear cuentas.
- Toda cuenta recibe el rol base `user` y puede recibir roles de aplicación autorizados.
- La cuenta nace en estado `pending_verification` y se envía una invitación asíncrona al correo.
- Un `admin` puede reenviar la invitación solo mientras la cuenta siga en `pending_verification`:
  anula los enlaces anteriores, emite uno nuevo de 24 h y registra la acción en auditoría.
- Un correo ya registrado devuelve `409` sin exponer más información de la cuenta existente.
- **Aceptación:** el alta devuelve `201`, registra al administrador como actor y encola la
  invitación; la cuenta no puede iniciar sesión antes de aceptarla.

> **Enmienda T3 · 2026-09-27 · D6/D9:** se retira el autorregistro y se sustituye por alta
> administrativa con invitación.

> **Enmienda T6 · 2026-09-28 · D12:** el reenvío evita que una invitación perdida o vencida
> deje una cuenta pendiente sin una vía de activación.

### RF-002 — Aceptación de invitación y verificación de correo · P0
El empleado demuestra que controla el correo al aceptar la invitación y definir su contraseña.
- El token tiene 32 bytes de `crypto/rand`, se guarda hasheado, expira en 24 h y es de un solo uso.
- La contraseña se almacena con Argon2id y debe tener entre 12 y 128 caracteres.
- Consumir el token activa la cuenta; un token expirado o reutilizado devuelve `410`.
- **Aceptación:** el correo llega a Mailpit; aceptar el enlace fija la contraseña y pasa la cuenta
  a `active`; reutilizar el enlace falla.

> **Enmienda T3 · 2026-09-27 · D6/D9:** aceptar la invitación sustituye la verificación separada
> y prueba la posesión del correo.

### RF-003 — Inicio de sesión · P0
Una cuenta activa inicia un desafío MFA presentando correo y contraseña.
- Credenciales inválidas devuelven `401` con un mensaje genérico, sin revelar si el correo
  existe.
- Una cuenta en `pending_verification`, `locked` o `disabled` no puede iniciar sesión.
- **Aceptación:** credenciales correctas devuelven `202` con `mfaToken`; contraseña incorrecta
  devuelve `401`; el intento queda en el audit log.

> **Enmienda T3 · 2026-09-27 · D11/ADR 0010:** todo login continúa obligatoriamente con MFA.

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
Los roles de directorio y de aplicación gobiernan ámbitos distintos.
- `admin` y `user` son roles de directorio fijos; `user` es el rol base de toda cuenta.
- Los roles de aplicación son datos configurables con nombre `<aplicacion>.<nombre>`; solo pertenecen
  a una aplicación y se asignan junto con el rol base.
- Un token dirigido a una aplicación incluye únicamente sus roles de aplicación en `roles` y sus
  permisos resueltos en `permissions`; los privilegios se verifican **en el servidor**, nunca
  confiando en el cliente.
- Un `user` que llama a un endpoint de administración recibe `403`.
- **Aceptación:** las rutas `/admin/*` responden `200` a un admin y `403` a un user.

> **Enmienda T3 · 2026-09-27 · D8:** se separan roles de directorio y roles de aplicación.

> **Enmienda T11 · 2026-10-05 · D9/ADR 0013:** los roles de aplicación dejan de ser un catálogo
> fijo; `admin` y `user` permanecen roles del sistema no editables.

### RF-010 — Administración de usuarios · P0
Un admin lista, busca, habilita, deshabilita y asigna roles a cuentas.
- No puede deshabilitarse a sí mismo (evita dejar el sistema sin administrador).
- No puede asignarse roles a sí mismo; otro administrador debe hacerlo y la acción se audita.
- **Aceptación:** un admin deshabilita a un usuario y ese usuario ya no puede iniciar sesión.

> **Enmienda T3 · 2026-09-27 · D8:** se añade separación de funciones para la autoasignación.

### RF-011 — Registro de auditoría · P0
Todo evento de seguridad queda registrado de forma inmutable.
- Eventos: alta, invitación aceptada, login y MFA exitosos/fallidos, reenvío o agotamiento del
  desafío, logout, rotación y reuso de refresh, cambio de contraseña, cambio de rol, autorización,
  canje y reuso de código OAuth y bloqueo de cuenta.
- Cada entrada guarda actor, acción, recurso, IP, user-agent y marca temporal.
- La tabla es *append-only*: sin `UPDATE` ni `DELETE` concedidos al rol de la aplicación.
- **Aceptación:** un login fallido crea una fila; intentar modificarla con el usuario de la
  aplicación falla por permisos.

> **Enmienda T3 · 2026-09-27 · D8/D9/ADR 0009/ADR 0010:** se actualiza el catálogo mínimo de
> eventos de seguridad de los nuevos flujos.

### RF-012 — Notificaciones asíncronas · P0
Los correos se envían fuera del ciclo de la petición HTTP.
- La API publica un mensaje en RabbitMQ; el worker lo consume y entrega por SMTP.
- Reintentos con retroceso exponencial; tras 3 fallos, el mensaje va a la *dead-letter queue*.
- **Aceptación:** el alta o un desafío MFA encolan mensajes visibles en RabbitMQ y el correo
  aparece en Mailpit; con el worker detenido, la petición HTTP sigue respondiendo en menos de 500 ms.

> **Enmienda T3 · 2026-09-27 · D5/D6/D9:** las notificaciones pasan a ser invitaciones y códigos MFA.

### RF-013 — Segundo factor obligatorio por correo · P1
Toda cuenta usa como segundo factor un código de un solo uso enviado al correo verificado.
- No existe enrolamiento: aceptar la invitación establece el canal verificado.
- No se admiten TOTP, códigos QR ni códigos de recuperación.
- **Aceptación:** ningún login emite una sesión sin completar el desafío por correo.

> **Enmienda T3 · 2026-09-27 · D5/D11/ADR 0010:** el MFA por correo obligatorio sustituye TOTP.

### RF-014 — Inicio de sesión con segundo factor · P1
La contraseña correcta inicia un desafío, pero no crea una sesión.
- El login devuelve `202` con un `mfaToken` temporal y envía un código de 6 dígitos generado con
  `crypto/rand`, guardado solo como hash, de vida de **5 minutos** y de un solo uso.
- Cada desafío admite como máximo **5 intentos** y el reenvío se permite una vez cada **60 segundos**.
- Cada desafío limita intentos; agotarlos lo anula. **Cada código rechazado** (también el que agota
  el desafío) cuenta como un intento fallido para RF-017, de modo que adivinar códigos a través de
  desafíos sucesivos bloquea la cuenta igual que adivinar la contraseña.
- El reenvío está limitado por frecuencia, anula el código anterior y queda auditado.
- Desafío, fallos, reenvío, agotamiento y acierto quedan auditados.
- **Aceptación:** un código válido devuelve `200`, el access token y la cookie refresh; reutilizarlo
  falla, y el código anterior falla después de un reenvío.

> **Enmienda T3 · 2026-09-27 · ADR 0010:** se concreta el desafío MFA por correo.

> **Enmienda T7-fix · 2026-09-28 · D16:** el código se publica después de confirmar la transacción
> (ADR 0006, Enmienda D16), al emitir y al reenviar. Si el broker falla en ese punto, el login (o
> el reenvío) responde `503` y el desafío no usado vence solo. Cada desafío nuevo anula los
> desafíos abiertos de la cuenta y se emiten como máximo **5 desafíos por cuenta cada 15 minutos**;
> el sexto responde `429` (`application/problem+json`) sin crear desafío ni enviar correo.

> **Enmienda T7 · 2026-09-28 · D15:** se fijan los límites de 5 minutos, 5 intentos y 60 segundos.
> El código se guarda como HMAC-SHA256 con clave en el token del desafío, y el éxito del inicio de
> sesión (`login_succeeded`, `last_login_at`) se registra al aceptar el código, no al verificar la contraseña.

### RF-015 — Restablecimiento de contraseña · P1
Quien olvida su contraseña la restablece por correo.
- La respuesta es siempre `202`, exista o no el correo (evita enumeración).
- Token de un solo uso con 1 h de vigencia; al usarlo se revocan todas las sesiones activas.
- **Aceptación:** completar el flujo permite entrar con la nueva contraseña y las sesiones
  anteriores dejan de funcionar.

> **Enmienda T3 · 2026-09-27 · D6:** comparte con la invitación el mecanismo de token de cuenta
> hasheado, expirable y de un solo uso, manteniendo propósitos y vigencias independientes.

> **Enmienda T6-fix · 2026-09-28 · D13:** un restablecimiento completado sobre una cuenta
> `locked` (RF-017) la pasa a `active` y limpia su bloqueo en la misma sentencia; no reactiva
> cuentas `disabled` ni `pending_verification`. Los intentos fallidos anteriores al
> restablecimiento dejan de contar para el conteo de bloqueo de RF-017, así que un solo fallo justo
> después no vuelve a bloquear la cuenta. Queda auditado como `password_reset_completed`, con
> metadatos que indican si desbloqueó la cuenta.

> **Enmienda T7 · 2026-09-28 · D14:** todo restablecimiento completado envía un aviso de contraseña
> cambiada y sesiones cerradas; indica el desbloqueo solo cuando aplica.

> **Enmienda T7-fix · 2026-09-28 · D16:** el aviso se publica después de confirmar el
> restablecimiento. Si el broker falla, el restablecimiento se mantiene, `confirm` responde `204` y
> el fallo queda en el log (ADR 0006, Enmienda D16).

### RF-016 — Sesiones activas · P1
El titular ve sus sesiones y puede revocarlas individualmente.
- Se muestran IP, user-agent, creación y último uso.
- **Aceptación:** revocar una sesión invalida su refresh token y deja el resto intactas.

### RF-017 — Bloqueo por fuerza bruta · P1
Tras 5 intentos fallidos en 15 minutos, la cuenta se bloquea 15 minutos.
- El bloqueo se registra en auditoría y se notifica al titular.
- Cuentan como fallo tanto la contraseña incorrecta (`login_failed`) como el código MFA rechazado
  (`mfa_code_rejected`), con el mismo umbral y ventana; el límite por IP cuenta ambos.
- **Aceptación:** seis intentos fallidos devuelven `423 Locked`; la contraseña correcta también
  falla mientras dure el bloqueo.

### RF-018 — Claves de servicio · P2
Un admin emite claves de API para integraciones máquina a máquina.

### RF-019 — Exportación del audit log · P2
Un admin exporta el registro de auditoría filtrado en CSV o JSON.

### RF-020 — Autorización de aplicaciones cliente · P1
Una aplicación cliente obtiene un access token mediante authorization code con PKCE S256.
- Existe un único cliente público, Contabilidad, con `client_id` y `redirect_uri` fijos en
  configuración; la URI de retorno exige coincidencia exacta y `state` es obligatorio.
- El código de autorización es aleatorio, se guarda solo como hash, vence en aproximadamente un
  minuto, es de un solo uso y queda ligado al cliente, la URI y el `code_challenge` S256.
- El Hub mantiene una sesión propia en cookie `HttpOnly`, `Secure`, `SameSite=Lax`, separada de la
  cookie refresh de la consola, que conserva `SameSite=Strict`; la crea el canje exitoso del desafío
  MFA (RF-014).
- Sin sesión SSO, `/oauth/authorize` redirige al login del Hub con un parámetro `continue` que solo
  admite rutas relativas que empiecen por `/oauth/authorize`; cualquier otro valor se ignora (sin
  redirección abierta).
- El access token tiene `aud` igual al cliente, solo sus roles de aplicación y los permisos
  resueltos para esa audiencia; no se entrega refresh token a la aplicación.
- CORS se permite únicamente en `/oauth/token` y `/.well-known/jwks.json`, sin credenciales y solo
  para el origen configurado del cliente.
- **Aceptación:** el flujo correcto redirige con `code` y el mismo `state`; una segunda entrada usa
  la sesión SSO sin contraseña; URI incorrecta, `state` ausente, PKCE `plain`, reuso del código
  o un `continue` absoluto o externo se rechazan; el JWT contiene solo los roles de Contabilidad.

> **Enmienda T3 · 2026-09-27 · ADR 0009:** se añade el contrato mínimo de SSO entre dominios.

> **Enmienda T11 · 2026-10-05 · D9/ADR 0013:** el token de Contabilidad incorpora `permissions`
> resueltos para su audiencia además de `roles`; el cliente público único y sus valores fijos se
> mantienen.

### RF-021 — Roles y permisos configurables por aplicación · P1
Cada aplicación declara sus permisos y un administrador configura los roles que podrán asignarse
para esa aplicación.
- Los permisos se registran por aplicación; un administrador crea, edita o elimina roles de
  aplicación con el nombre `<aplicacion>.<nombre>` y solo puede marcar permisos de esa aplicación.
- `admin` y `user` son roles de directorio del sistema: no se crean, editan ni eliminan desde la
  grilla.
- Un token emitido para una aplicación lleva los permisos resueltos de sus roles para esa audiencia
  en el claim `permissions`. Un cambio rige al emitir el siguiente token, con una espera máxima de
  15 minutos por la vigencia del access token.
- **Aceptación:** (1) solo se crean roles de aplicación; (2) un administrador no edita los permisos
  de un rol que posee; (3) un permiso desconocido o de otra aplicación devuelve `400`; (4) no se
  elimina un rol asignado; (5) crear, actualizar y eliminar un rol deja auditoría.

> **Decisión T11 · 2026-10-05 · D9/ADR 0013:** se define la grilla configurable y el alcance del
> token por aplicación.

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
