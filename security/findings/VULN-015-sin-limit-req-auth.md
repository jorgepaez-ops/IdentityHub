# VULN-015 — Sin `limit_req` en `/api/v1/auth/*`

| | |
|---|---|
| **Severidad** | no informada por el escáner |
| **Estado** | remediado |
| **Detectado por** | ningún gate lo detecta hoy |
| **Componente** | `frontend/nginx/default.conf` (sin línea reportada por el escáner) |
| **Amenaza** | AM-001 (fuerza bruta sobre contraseñas) · AM-017 (Argon2id como amplificador) |
| **Sembrada** | sí |
| **Evidencia antes** | `docs/evidencia/VULN-015/evidencia.json` |
| **Commit de remediación** | `18dea33` (T29) |
| **Evidencia después** | |
| **Run de Actions (antes/después)** | — / — |

## Evidencia

```
30 POST seguidos a /api/v1/auth/login: 30 respuestas 502 y ninguna 429 (sin limit_req)
```

## Por qué importa en esta aplicación

Sin limitación previa al proxy, intentos masivos de login permiten fuerza bruta y agotan el coste de Argon2id.

## Remediación

`limit_req_zone` (5r/s) y `location /api/v1/auth/` con `limit_req zone=auth burst=5 nodelay` y
`limit_req_status 429` (para que coincida con el `429` que ya declara el contrato OpenAPI, en vez
del `503` por defecto de nginx). Verificado con una ráfaga real de 30 `POST` a
`/api/v1/auth/login` contra el stack levantado: las primeras pasan, el resto vuelve `429` hasta
que el balde de tokens se vacía.
