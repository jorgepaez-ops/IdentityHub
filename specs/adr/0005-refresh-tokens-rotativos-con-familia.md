# 0005 — Refresh tokens opacos, rotativos y agrupados en familias

Estado: aceptada · 2026-09-05 · Enmienda C2 · 2026-09-19

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
- No hay ventana de gracia: contradice el escenario «reutilizar un refresh
  rotado revoca la familia», que es inmediato, y devolver el mismo par ya
  emitido exigiría almacenar el token en claro, cuando solo se guarda su
  SHA-256. Se acepta el riesgo residual de que dos pestañas o clientes que
  renueven simultáneamente con el mismo token provoquen un falso positivo y
  cierren la sesión, pues el segundo uso se interpreta como reuso. Como
  mitigación para el frontend de la semana 3, el cliente tendrá un único
  refrescador compartido entre pestañas: solo habrá una renovación en vuelo por
  sesión —por ejemplo, mediante Web Locks API o `BroadcastChannel`— y las demás
  pestañas esperarán y reutilizarán el resultado. Se descartó la alternativa de
  devolver el mismo par ya emitido durante una ventana de gracia.
- Un cliente que pierda la respuesta de la renovación pierde la sesión. Es el
  precio de detectar el robo, y se prefiere pecar por ese lado.
