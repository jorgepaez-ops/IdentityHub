# 0004 — Argon2id con parámetros calibrados

Estado: aceptada · 2026-09-05

## Contexto

Hay que elegir función de derivación para las contraseñas. Las candidatas serias
son bcrypt, scrypt y Argon2id.

## Decisión

**Argon2id** (`golang.org/x/crypto/argon2`) con estos parámetros iniciales:

| Parámetro | Valor |
|---|---|
| memoria | 64 MiB |
| iteraciones | 3 |
| paralelismo | 2 |
| longitud de sal | 16 bytes de `crypto/rand` |
| longitud de clave | 32 bytes |

Ganador del Password Hashing Competition y recomendación actual de OWASP. La
variante `id` combina resistencia a canales laterales (Argon2i) y a GPU
(Argon2d). bcrypt trunca en 72 bytes y su coste solo escala en CPU, no en
memoria, que es donde muerde el atacante con hardware dedicado.

Los parámetros viven en la configuración y se registran en el prefijo del hash,
de modo que se pueden subir sin invalidar los hashes existentes: al iniciar
sesión, si el hash almacenado usa parámetros antiguos, se recalcula.

## Consecuencias

- Cada verificación cuesta ~250 ms y 64 MiB. Es intencional contra la fuerza
  bruta, pero convierte el login en un amplificador de denegación de servicio
  (AM-017): se mitiga con `limit_req` en Nginx **antes** de llegar a la API.
- 64 MiB por verificación concurrente obliga a limitar la concurrencia del
  endpoint de login. Documentado en el runbook.
- Cuando el usuario no existe se ejecuta igualmente una verificación contra un
  hash señuelo, para que el tiempo de respuesta no revele nada (AM-004).
