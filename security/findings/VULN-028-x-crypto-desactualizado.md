# VULN-028 — `golang.org/x/crypto` desactualizado

| | |
|---|---|
| **Severidad** | no informada por los avisos recibidos; no alcanzable según análisis de llamadas |
| **Estado** | abierto |
| **Detectado por** | osv-scanner v2.6.0 (`ci.yml`, job `5 · Dependencias vulnerables`, paso `osv-scanner`) |
| **Componente** | `backend/go.mod`: `golang.org/x/crypto@v0.55.0` |
| **Avisos** | GO-2026-6354 (CVE-2026-78662, corregido en 0.56.0); GO-2026-6355 (CVE-2026-56855, corregido en 0.56.0); GO-2026-5932 (sin versión corregida) |
| **Amenaza** | AM-009 (dependencia comprometida en la cadena de suministro) |
| **Sembrada** | no — hallazgo nuevo detectado al actualizar el escáner |
| **Evidencia antes** | `docs/evidencia/VULN-028/evidencia.json` |
| **Commit de remediación** | pendiente (T34b) |
| **Evidencia después** | pendiente (T34b) |
| **Run de Actions (antes/después)** | https://github.com/jorgepaez-ops/IdentityHub/actions/runs/36281691237 / — |

## Evidencia

```
golang.org/x/crypto@v0.55.0
  GO-2026-6354 (CVE-2026-78662) — fixed in 0.56.0
  GO-2026-6355 (CVE-2026-56855) — fixed in 0.56.0
  GO-2026-5932 — no fixed version; subpaquete afectado inseguro por diseño y sin mantenimiento
```

`osv-scanner` v2.6.0 ejecutó análisis de llamadas y marcó los tres avisos como no llamados.
`govulncheck` también informó que no son alcanzables. La señal de exposición actual es menor por
esa falta de alcance, pero los dos avisos con versión corregida siguen requiriendo actualización y
el tercero exige decidir explícitamente si se elimina el subpaquete afectado o se acepta el riesgo.

## Por qué importa en esta aplicación

La dependencia participa en componentes criptográficos del IdP. AM-009 exige gestionar los avisos
de la cadena de suministro incluso cuando no hay una ruta alcanzable hoy: una importación futura o
un cambio de dependencia podría volver alcanzable código vulnerable o no mantenido sin un registro
de la decisión.

## Remediación

T34b debe subir `golang.org/x/crypto` a 0.56.0 o superior y ejecutar `go mod tidy`. Para
GO-2026-5932 debe identificar el subpaquete e importaciones afectadas; si no puede eliminarse,
la aceptación del riesgo requiere decisión documentada del usuario antes de cerrar la tarea.
