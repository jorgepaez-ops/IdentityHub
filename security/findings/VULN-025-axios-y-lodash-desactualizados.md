# VULN-025 — axios 0.21.1 / lodash 4.17.15

| | |
|---|---|
| **Severidad** | HIGH |
| **Estado** | remediado |
| **Detectado por** | npm audit (baseline-scan) · osv-scanner (CI) |
| **Componente** | `frontend/package.json`: axios@0.21.1, lodash@4.17.15 |
| **Amenaza** | AM-009 (dependencia comprometida en la cadena de suministro) |
| **Sembrada** | sí |
| **Evidencia antes** | `docs/evidencia/VULN-025/evidencia.json` |
| **Commit de remediación** | `534f13e` (T25) |
| **Evidencia después** | `docs/evidencia/VULN-025/evidencia.json` (job "5", npm audit: 15 vulnerabilidades, sin axios/lodash) |
| **Run de Actions (antes/después)** | https://github.com/jorgepaez-ops/IdentityHub/actions/runs/35476102444 / https://github.com/jorgepaez-ops/IdentityHub/actions/runs/36259385478 |

## Evidencia

```
npm audit: 17 vulnerabilidades (2 critical, 10 high, 5 moderate); axios <=0.32.0 (high) y lodash <=4.17.23 (high)
```

## Por qué importa en esta aplicación

Dependencias vulnerables en el cliente pueden facilitar SSRF, contaminación de prototipos o ejecución de código según su uso.

## Remediación

Retirados por completo en T25 (no actualizados): `grep` sobre `frontend/src` confirmó que ninguno de
los dos se importa en ningún archivo (`client.ts` ya usaba `fetch`), así que quitarlos es más simple
y más seguro que fijar una versión más nueva que igual arrastraría avisos conocidos. `@types/lodash`
se retiró junto con `lodash`. Lockfile regenerado con `npm install`.
