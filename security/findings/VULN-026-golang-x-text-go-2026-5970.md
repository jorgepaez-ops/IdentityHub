# VULN-026 — `golang.org/x/text` (GO-2026-5970)

| | |
|---|---|
| **Severidad** | no informada por el escáner |
| **Estado** | remediado |
| **Detectado por** | govulncheck (baseline-scan) |
| **Componente** | `backend/go.mod`: golang.org/x/text@v0.14.0 |
| **Amenaza** | AM-009 (dependencia comprometida en la cadena de suministro) |
| **Sembrada** | sí |
| **Evidencia antes** | `docs/evidencia/VULN-026/evidencia.json` |
| **Commit de remediación** | `001a489` (T24) |
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

Actualizado a `golang.org/x/text` v0.41.0 en T24 (no v0.42.0: esa versión exige
`go 1.26.0`, un salto de directiva fuera de alcance de esta tarea).
