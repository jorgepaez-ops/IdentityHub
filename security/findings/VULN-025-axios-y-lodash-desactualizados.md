# VULN-025 — axios 0.21.1 / lodash 4.17.15

| | |
|---|---|
| **Severidad** | HIGH |
| **Estado** | abierto |
| **Detectado por** | npm audit (baseline-scan) · osv-scanner (CI) |
| **Componente** | `frontend/package.json`: axios@0.21.1, lodash@4.17.15 |
| **Amenaza** | AM-009 (dependencia comprometida en la cadena de suministro) |
| **Sembrada** | sí |
| **Evidencia antes** | `docs/evidencia/VULN-025/evidencia.json` |
| **Commit de remediación** | |
| **Evidencia después** | |
| **Run de Actions (antes/después)** | https://github.com/jorgepaez-ops/IdentityHub/actions/runs/35476102444 / — |

## Evidencia

```
npm audit: 17 vulnerabilidades (2 critical, 10 high, 5 moderate); axios <=0.32.0 (high) y lodash <=4.17.23 (high)
```

## Por qué importa en esta aplicación

Dependencias vulnerables en el cliente pueden facilitar SSRF, contaminación de prototipos o ejecución de código según su uso.

## Remediación

Actualizar axios y lodash, regenerar el lockfile y comprobar el audit en T25.
