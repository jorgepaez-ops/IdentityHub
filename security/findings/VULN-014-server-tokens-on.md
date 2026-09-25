# VULN-014 — `server_tokens on`

| | |
|---|---|
| **Severidad** | no informada por el escáner |
| **Estado** | abierto |
| **Detectado por** | ningún gate lo detecta hoy |
| **Componente** | `frontend/nginx/default.conf` (sin línea reportada por el escáner) |
| **Amenaza** | sin amenaza asociada en el modelo |
| **Sembrada** | sí |
| **Evidencia antes** | `docs/evidencia/VULN-014/evidencia.json` |
| **Commit de remediación** | |
| **Evidencia después** | |
| **Run de Actions (antes/después)** | — / — |

## Evidencia

```
curl -sI http://localhost:8080/: cabecera 'Server: nginx/1.31.6' (server_tokens on)
```

## Por qué importa en esta aplicación

Exponer la versión facilita que un atacante seleccione exploits y reconocimiento específicos.

## Remediación

Desactivar `server_tokens` en T29.
