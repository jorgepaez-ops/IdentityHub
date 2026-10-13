# 0014 — Claves de servicio para integraciones máquina a máquina

Estado: aceptada · 2026-10-12 · Implementa RF-018

## Contexto

RF-018 pide que un admin emita claves de API para integraciones máquina a máquina. El caso de
uso que lo justifica es un SIEM o sistema de reportes que consulta el audit log (RF-011) sin una
sesión humana: un access token JWT dura minutos y exige login con contraseña y MFA, así que no sirve
para un proceso desatendido. Reutilizar la cuenta de un admin con su contraseña sería peor: daría
a la integración todos los permisos de ese admin.

## Decisión

- **Formato:** `ihk_<publicId>_<secret>`. `publicId` son 16 caracteres hexadecimales (8 bytes de
  `crypto/rand`), único y no secreto; sirve para buscar la fila y para identificar la clave en el
  listado y en la auditoría. `secret` son 32 bytes de `crypto/rand` en base64url sin relleno (43
  caracteres). El prefijo `ihk_` hace la clave reconocible (y detectable por escáneres de secretos).
- **Almacenamiento:** tabla `service_keys` con `public_id` único y `secret_hash` (SHA-256 del secreto).
  Un hash rápido es suficiente porque el secreto tiene 256 bits de entropía: no es una contraseña
  elegida por una persona y no hay diccionario que atacar; es la misma convención que los refresh
  tokens y los tokens de invitación del proyecto. No se usa pepper porque el proyecto no tiene una
  configuración de pepper y, con esa entropía, no añadiría defensa real. La clave completa se devuelve
  una sola vez en el `201` y nunca se vuelve a poder leer.
- **Verificación:** se busca por `public_id`, se compara el hash con `crypto/subtle` en tiempo
  constante (contra un hash señuelo si el `public_id` no existe) y se rechaza si está revocada o
  vencida. Desconocida, incorrecta, expirada y revocada devuelven el mismo `401` (sin oráculo); el
  motivo real solo queda en la auditoría.
- **Vigencia y revocación:** `expires_at` es obligatorio (1 a 365 días). La revocación es lógica
  (`revoked_at`) e idempotente, y no se puede reactivar una clave.
- **Alcances:** lista cerrada en el servidor, hoy solo `audit-log:read`. Cada operación que admite
  claves lo declara en el código (un envoltorio que pide un alcance concreto) y en OpenAPI mediante un
  esquema de seguridad propio, `serviceKeyAuth`. Ninguna otra operación las acepta: el middleware de
  access token no reconoce el formato y responde `401`. Admitir el export del audit log (RF-019) será
  envolverlo con el mismo alcance.
- **Gestión:** solo un admin con access token emite, lista y revoca claves. Las claves de servicio
  no gestionan claves (no hay autoescalada).
- **Auditoría:** `service_key_created`, `service_key_revoked` y `service_key_rejected`, con el
  `publicId` (si tiene formato válido) y nunca el secreto. `last_used_at` se actualiza como máximo una
  vez por minuto para no convertir cada lectura en una escritura.
- **Límite de peticiones:** no se añade un limitador propio. La adivinanza es inviable (256 bits) y
  Nginx ya limita el tráfico externo; los usos rechazados quedan auditados para detectar abuso.

## Consecuencias

- Buenas: credencial de mínimo privilegio, revocable y auditada para integraciones desatendidas; una
  fuga de la base de datos no revela claves utilizables.
- Malas: la clave es un bearer de larga vida (hasta 365 días) y quien la robe puede leer el audit log
  hasta que se revoque o venza; el audit log contiene IP y direcciones de correo, así que el alcance
  no es trivial. No hay rotación automática: se emite una nueva y se revoca la anterior.
- Un uso rechazado de una clave válida en una operación que no declara su alcance responde `401` pero
  no se audita como `service_key_rejected` (el middleware de access token no la identifica). Se acepta
  porque esa vía no concede ningún acceso.
