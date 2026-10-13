# VULN-028 — `golang.org/x/crypto` desactualizado

| | |
|---|---|
| **Severidad** | no informada por los avisos recibidos; no alcanzable según análisis de llamadas |
| **Estado** | GO-2026-6354 y GO-2026-6355 remediados subiendo a `x/crypto` 0.56.0 (T34b); GO-2026-5932 sin versión corregida y reclasificado el 2026-10-12 como **no afectado** (código vulnerable no presente; ver "Reclasificación 2026-10-12" abajo) |
| **Detectado por** | osv-scanner v2.6.0 (`ci.yml`, job `5 · Dependencias vulnerables`, paso `osv-scanner`) |
| **Componente** | `backend/go.mod`: `golang.org/x/crypto@v0.55.0` → `v0.56.0` (T34b) |
| **Avisos** | GO-2026-6354 (CVE-2026-78662, corregido en 0.56.0); GO-2026-6355 (CVE-2026-56855, corregido en 0.56.0); GO-2026-5932 (sin versión corregida; no afectado desde 2026-10-12) |
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

## Historial: riesgo aceptado (2026-09-26 a 2026-10-12, GO-2026-5932)

Se conserva como registro de la decisión original, que T34b tomó con fecha de revisión 2026-12-25.
Quedó reemplazada por la reclasificación de abajo y ya no hay fecha de revisión pendiente.

- **Qué:** el aviso cubre `golang.org/x/crypto/openpgp` y sus subpaquetes (`packet`, `armor`,
  `clearsign`, `errors`, `elgamal`, `s2k`); no tiene versión corregida porque es un problema de
  diseño y falta de mantenimiento del paquete completo, según el reporte de
  `pkg.go.dev/vuln/GO-2026-5932`.
- **Por qué no se eliminó:** `golang.org/x/crypto` no puede eliminarse del módulo (lo usa
  `internal/auth` para Argon2id, ADR 0004).
- **Evidencia de entonces:** `go list -deps ./...` desde `backend/` no incluía
  `golang.org/x/crypto/openpgp`; `osv-scanner` v2.6.0 y `govulncheck` marcaban el aviso como no
  llamado / no alcanzable.

## Reclasificación 2026-10-12: no afectado

Decisión del usuario (2026-10-12): GO-2026-5932 deja de tratarse como riesgo aceptado con
expiración y pasa a **no afectado**, porque el código vulnerable no está presente en ningún
artefacto del proyecto. Se elimina la obligación de revisar antes del 2026-12-25.

Evidencia verificada el 2026-10-12:

- **osv.dev GO-2026-5932:** afecta a `golang.org/x/crypto` desde la versión 0, sin versión
  corregida, y solo a los paquetes `golang.org/x/crypto/openpgp`, `openpgp/packet`,
  `openpgp/armor`, `openpgp/clearsign`, `openpgp/errors`, `openpgp/elgamal` y `openpgp/s2k`. El
  paquete está obsoleto y sin mantenimiento por diseño, así que nunca habrá una versión corregida.
- **`go -C backend list -deps ./... | grep x/crypto/openpgp`:** 0 resultados. El backend solo usa
  `golang.org/x/crypto/argon2` y `golang.org/x/crypto/blake2b`.
- **`go -C tools list -deps <herramienta> | grep x/crypto/openpgp`** para oapi-codegen, gosec,
  sqlc y govulncheck: 0 resultados en las cuatro.
- **`govulncheck`** no lo reporta (no hay ruta de llamada); solo `osv-scanner` lo marca, porque
  coincide a nivel de módulo.

La dependencia no puede eliminarse (Argon2id y BLAKE2b), por lo que el aviso seguirá apareciendo en
`osv-scanner`. La exclusión de `backend/osv-scanner.toml` y `tools/osv-scanner.toml` pasa a ser
permanente (sin `ignoreUntil`) y su motivo cita esta evidencia. Si algún paquete propio o una
dependencia futura llegara a importar `x/crypto/openpgp*`, el estado debe reabrirse; `go list -deps`
es la comprobación.
