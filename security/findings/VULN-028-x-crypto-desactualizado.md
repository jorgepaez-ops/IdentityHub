# VULN-028 — `golang.org/x/crypto` desactualizado

| | |
|---|---|
| **Severidad** | no informada por los avisos recibidos; no alcanzable según análisis de llamadas |
| **Estado** | GO-2026-6354 y GO-2026-6355 remediados subiendo a `x/crypto` 0.56.0 (T34b); GO-2026-5932 sin versión corregida — riesgo aceptado y documentado (ver "Riesgo aceptado" abajo) |
| **Detectado por** | osv-scanner v2.6.0 (`ci.yml`, job `5 · Dependencias vulnerables`, paso `osv-scanner`) |
| **Componente** | `backend/go.mod`: `golang.org/x/crypto@v0.55.0` → `v0.56.0` (T34b) |
| **Avisos** | GO-2026-6354 (CVE-2026-78662, corregido en 0.56.0); GO-2026-6355 (CVE-2026-56855, corregido en 0.56.0); GO-2026-5932 (sin versión corregida, riesgo aceptado) |
| **Amenaza** | AM-009 (dependencia comprometida en la cadena de suministro) |
| **Sembrada** | no — hallazgo nuevo detectado al actualizar el escáner |
| **Evidencia antes** | `docs/evidencia/VULN-028/evidencia.json` — informe Desktop §VULN-028 antes |
| **Commit de remediación** | `98f5f98` (T34b) |
| **Evidencia después** | run 36283211113 (CI 13/13 en verde) — informe Desktop, seccion VULN-028 despues |
| **Run de Actions (antes/después)** | https://github.com/jorgepaez-ops/IdentityHub/actions/runs/36281691237 / https://github.com/jorgepaez-ops/IdentityHub/actions/runs/36283211113 |

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

T34b subió `golang.org/x/crypto` de 0.55.0 a 0.56.0 (`go get` + `go mod tidy`), lo que corrige
GO-2026-6354 y GO-2026-6355. `golang.org/x/crypto` >= 0.56.0 declara `go 1.26.0` en su propio
`go.mod`, así que la directiva `go` del módulo subió de 1.25.0 a 1.26.0 (MVS obliga a igualar o
superar el mínimo de cada dependencia); no fue una elección independiente de T34b. `backend/Dockerfile`
usa `golang:1.26-bookworm` y `GO_VERSION` en `ci.yml` usa `"1.26"`; ambos satisfacen la
directiva `go 1.26.0` del módulo tras T34b.

## Riesgo aceptado (GO-2026-5932)

- **Qué:** el aviso cubre `golang.org/x/crypto/openpgp` y sus subpaquetes (`packet`, `armor`,
  `clearsign`, `errors`, `elgamal`, `s2k`); no tiene versión corregida porque es un problema de
  diseño y falta de mantenimiento del paquete completo, según el reporte de
  `pkg.go.dev/vuln/GO-2026-5932`.
- **Por qué se acepta en vez de eliminar:** `golang.org/x/crypto` no puede eliminarse del módulo
  (lo usa `internal/auth` para Argon2id, ADR 0004), pero ningún paquete propio ni ninguna
  dependencia importa `golang.org/x/crypto/openpgp` ni sus subpaquetes. El aviso se reporta a nivel
  de módulo, no porque el código lo alcance.
- **Evidencia:** `go list -deps ./...` desde `backend/` no incluye `golang.org/x/crypto/openpgp`
  (los únicos subpaquetes de `x/crypto` en la lista de dependencias son `argon2` y `blake2b`, más
  `cryptobyte`/`chacha20` transitivos de otras dependencias). `osv-scanner` v2.6.0 y `govulncheck`
  marcan GO-2026-5932 como no llamado / no alcanzable.
- **Dónde vive la excepción:** `backend/osv-scanner.toml` (`[[IgnoredVulns]]`, solo
  `GO-2026-5932`, `ignoreUntil = 2026-12-25`).
- **Fecha de revisión:** 2026-12-25 (~90 días desde 2026-09-26). Revisar si para entonces alguna
  dependencia empezó a importar `openpgp` o si el aviso obtuvo versión corregida.
