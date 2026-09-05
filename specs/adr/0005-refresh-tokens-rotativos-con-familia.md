# 0005 — Refresh tokens opacos, rotativos y agrupados en familias

Estado: aceptada · 2026-09-05

## Contexto

El access token es un JWT de vida corta y no se puede revocar antes de que
expire. La sesión larga necesita un mecanismo distinto que sí sea revocable.

## Decisión

- El refresh token es **opaco**, no un JWT: 32 bytes de `crypto/rand` en
  base64url. No transporta información; su único significado es la fila que
  apunta en la base de datos, y por tanto es revocable de inmediato.
- Se almacena como **SHA-256**, no en claro. A diferencia de una contraseña no
  necesita Argon2: ya tiene 256 bits de entropía, así que no hay nada que
  precomputar y un hash rápido basta.
- **Rotación obligatoria**: cada uso emite uno nuevo y marca el anterior como
  `rotated`.
- Todos los tokens derivados de un mismo login comparten `family_id`. Presentar
  uno ya rotado significa que existen dos copias de la credencial —una robada—,
  así que se revoca la familia entera y se avisa al titular (RF-006, AM-002).

## Consecuencias

- Cada renovación es una escritura en base de datos. Aceptable a esta escala; en
  otra habría que pensar en particionar o en Redis.
- Una condición de carrera legítima (dos pestañas renovando a la vez) puede
  disparar un falso positivo y cerrar la sesión. Se acota con una ventana de
  gracia de 10 segundos durante la cual se acepta reutilizar el token recién
  rotado devolviendo el mismo par ya emitido.
- Un cliente que pierda la respuesta de la renovación pierde la sesión. Es el
  precio de detectar el robo, y se prefiere pecar por ese lado.
