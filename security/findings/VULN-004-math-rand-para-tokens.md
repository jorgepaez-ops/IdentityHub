# VULN-004 — `math/rand` para tokens

| | |
|---|---|
| **Severidad** | HIGH |
| **Estado** | remediado |
| **Detectado por** | gosec G404 (baseline-scan) · Semgrep math-random-used |
| **Componente** | `backend/internal/api/legacy_auth.go:30,66` |
| **Amenaza** | AM-002 (robo de refresh token y uso paralelo) |
| **Sembrada** | sí |
| **Evidencia antes** | `docs/evidencia/VULN-004/evidencia.json` |
| **Commit de remediación** | `51a7a4f` (T23) |
| **Evidencia después** | `docs/evidencia/VULN-004/evidencia.json` (golangci-lint: "0 issues."; Code scanning #42 sigue "Open" en `main`, ver Discrepancia #6 del informe) |
| **Run de Actions (antes/después)** | https://github.com/jorgepaez-ops/IdentityHub/actions/runs/35476102444 / https://github.com/jorgepaez-ops/IdentityHub/actions/runs/36208104969 |

## Evidencia

```
Use of weak random number generator (math/rand or math/rand/v2 instead of crypto/rand)

`backend/internal/api/legacy_auth.go:66`
```

## Por qué importa en esta aplicación

Un token predecible permite a un atacante anticipar sesiones o credenciales de recuperación.

## Remediación

Sustituir la generación insegura por `crypto/rand` en T23.
