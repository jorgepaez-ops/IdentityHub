# VULN-013 — Sin CSP, HSTS, X-Frame-Options y nosniff

| | |
|---|---|
| **Severidad** | no informada por el escáner |
| **Estado** | abierto |
| **Detectado por** | ningún gate lo detecta hoy |
| **Componente** | `frontend/nginx/default.conf` (sin línea reportada por el escáner) |
| **Amenaza** | AM-015 (XSS que roba el token del `localStorage`) |
| **Sembrada** | sí |
| **Evidencia antes** | `docs/evidencia/VULN-013/evidencia.json` |
| **Commit de remediación** | |
| **Evidencia después** | |
| **Run de Actions (antes/después)** | — / — |

## Evidencia

```
curl -sI http://localhost:8080/: la respuesta no trae Content-Security-Policy, Strict-Transport-Security, X-Content-Type-Options, X-Frame-Options ni Referrer-Policy
```

## Por qué importa en esta aplicación

La ausencia de cabeceras elimina defensas del navegador frente a XSS, clickjacking y contenido interpretado incorrectamente.

## Remediación

Añadir cabeceras de seguridad en Nginx durante T29.
