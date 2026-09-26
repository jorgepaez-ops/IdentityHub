# VULN-011 — `ADD` desde URL remota

| | |
|---|---|
| **Severidad** | no informada por ningún escáner |
| **Estado** | remediado |
| **Detectado por** | ningún gate lo detecta hoy |
| **Componente** | `backend/Dockerfile:44` |
| **Amenaza** | AM-009 (dependencia comprometida en la cadena de suministro) |
| **Sembrada** | sí |
| **Evidencia antes** | `docs/evidencia/VULN-011/evidencia.json` |
| **Commit de remediación** | `a0c64d6` (T27) |
| **Evidencia después** | `docs/evidencia/VULN-011/evidencia.json` (diff del commit: `ADD` eliminado por completo) |
| **Run de Actions (antes/después)** | https://github.com/jorgepaez-ops/IdentityHub/actions/runs/35476102444 / — (sin gate ni antes ni después) |

## Evidencia

```
Ningún gate lo detecta: backend/Dockerfile:44 contiene ADD https://raw.githubusercontent.com/..., pero Hadolint v2.12.0 no emite DL3020 (D4) para un ADD con URL y Trivy config no lo señala.
```

## Por qué importa en esta aplicación

Descargar contenido remoto durante la construcción sin verificarlo introduce una entrada no reproducible en la cadena de suministro.

## Remediación

`ADD` remoto eliminado sin reemplazo: no aportaba nada al binario en tiempo de ejecución (la
licencia se puede consultar en el propio repo de Go, no hace falta copiarla a la imagen).
