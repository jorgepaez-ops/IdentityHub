# VULN-011 — `ADD` desde URL remota

| | |
|---|---|
| **Severidad** | no informada por ningún escáner |
| **Estado** | abierto |
| **Detectado por** | ningún gate lo detecta hoy |
| **Componente** | `backend/Dockerfile:44` |
| **Amenaza** | AM-009 (dependencia comprometida en la cadena de suministro) |
| **Sembrada** | sí |
| **Evidencia antes** | `docs/evidencia/VULN-011/evidencia.json` |
| **Commit de remediación** | |
| **Evidencia después** | |
| **Run de Actions (antes/después)** | https://github.com/jorgepaez-ops/IdentityHub/actions/runs/35476102444 / — |

## Evidencia

```
Ningún gate lo detecta: backend/Dockerfile:44 contiene ADD https://raw.githubusercontent.com/..., pero Hadolint v2.12.0 no emite DL3020 (D4) para un ADD con URL y Trivy config no lo señala.
```

## Por qué importa en esta aplicación

Descargar contenido remoto durante la construcción sin verificarlo introduce una entrada no reproducible en la cadena de suministro.

## Remediación

Eliminar la descarga remota o verificar un artefacto versionado en T27.
