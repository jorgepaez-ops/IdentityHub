# VULN-015 — Sin `limit_req` en `/api/v1/auth/*`

| | |
|---|---|
| **Severidad** | no informada por el escáner |
| **Estado** | abierto |
| **Detectado por** | ningún gate lo detecta hoy |
| **Componente** | `frontend/nginx/default.conf` (sin línea reportada por el escáner) |
| **Amenaza** | AM-001 (fuerza bruta sobre contraseñas) · AM-017 (Argon2id como amplificador) |
| **Sembrada** | sí |
| **Evidencia antes** | `docs/evidencia/VULN-015/evidencia.json` |
| **Commit de remediación** | |
| **Evidencia después** | |
| **Run de Actions (antes/después)** | — / — |

## Evidencia

```
30 POST seguidos a /api/v1/auth/login: 30 respuestas 502 y ninguna 429 (sin limit_req)
```

## Por qué importa en esta aplicación

Sin limitación previa al proxy, intentos masivos de login permiten fuerza bruta y agotan el coste de Argon2id.

## Remediación

Configurar `limit_req` para las rutas de autenticación en T29.
