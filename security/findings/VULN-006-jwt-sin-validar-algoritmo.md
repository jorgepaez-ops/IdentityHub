# VULN-006 — JWT sin validar algoritmo

| | |
|---|---|
| **Severidad** | no informada por ningún escáner |
| **Estado** | remediado |
| **Detectado por** | ningún gate lo detecta hoy |
| **Componente** | `backend/internal/api/legacy_auth.go` (sin línea reportada por el escáner) |
| **Amenaza** | AM-003 (falsificación de JWT con `alg: none` o cambio a HS256) |
| **Sembrada** | sí |
| **Evidencia antes** | `docs/evidencia/VULN-006/evidencia.json` |
| **Commit de remediación** | `51a7a4f` (T23) |
| **Evidencia después** | |
| **Run de Actions (antes/después)** | https://github.com/jorgepaez-ops/IdentityHub/actions/runs/35476102444 / — |

## Evidencia

```
Ningún gate lo detecta (D3 confirmado). Semgrep hardcoded-jwt-key (legacy_auth.go:121) es otro problema (clave incrustada, VULN-001), no la falta de validación del algoritmo.
```

## Por qué importa en esta aplicación

Aceptar un algoritmo no permitido permitiría forjar tokens y suplantar identidades.

## Remediación

Validar explícitamente `EdDSA` antes de comprobar la firma en T23.
