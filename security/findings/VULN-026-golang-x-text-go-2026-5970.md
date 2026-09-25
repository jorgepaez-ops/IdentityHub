# VULN-026 — `golang.org/x/text` (GO-2026-5970)

| | |
|---|---|
| **Severidad** | no informada por el escáner |
| **Estado** | abierto |
| **Detectado por** | govulncheck (baseline-scan) |
| **Componente** | `backend/go.mod`: golang.org/x/text@v0.14.0 |
| **Amenaza** | AM-009 (dependencia comprometida en la cadena de suministro) |
| **Sembrada** | sí |
| **Evidencia antes** | `docs/evidencia/VULN-026/evidencia.json` |
| **Commit de remediación** | |
| **Evidencia después** | |
| **Run de Actions (antes/después)** | https://github.com/jorgepaez-ops/IdentityHub/actions/runs/35476102444 / — |

## Evidencia

```
Vulnerability #5: GO-2026-5970
    Infinite loop on invalid input in golang.org/x/text
  Module: golang.org/x/text
    Found in: golang.org/x/text@v0.14.0
    Fixed in: golang.org/x/text@v0.39.0
```

## Por qué importa en esta aplicación

Una entrada inválida que activa el bucle puede degradar la disponibilidad del servicio de identidad.

## Remediación

Actualizar `golang.org/x/text` a la versión corregida en T24.
