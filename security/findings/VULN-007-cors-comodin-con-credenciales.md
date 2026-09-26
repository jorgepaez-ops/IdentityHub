# VULN-007 — CORS comodín con credenciales

| | |
|---|---|
| **Severidad** | no informada por ningún escáner |
| **Estado** | remediado |
| **Detectado por** | ningún gate lo detecta hoy |
| **Componente** | `backend/internal/api/legacy_auth.go` (sin línea reportada por el escáner) |
| **Amenaza** | sin amenaza asociada en el modelo |
| **Sembrada** | sí |
| **Evidencia antes** | `docs/evidencia/VULN-007/evidencia.json` |
| **Commit de remediación** | `51a7a4f` (T23) |
| **Evidencia después** | `docs/evidencia/VULN-007/evidencia.json` (sin gate propio, ZAP llega en semana 3; evidencia por diff del commit, `legacy_auth.go` borrado entero) |
| **Run de Actions (antes/después)** | https://github.com/jorgepaez-ops/IdentityHub/actions/runs/35476102444 / — (evidencia por commit, no por run) |

## Evidencia

```
Ningún gate lo detecta hoy (D3 confirmado); ZAP llega en la semana 3.
```

## Por qué importa en esta aplicación

Una política CORS permisiva puede exponer respuestas autenticadas a orígenes no autorizados.

## Remediación

Restringir orígenes, métodos y cabeceras al contrato de la aplicación en T23.
