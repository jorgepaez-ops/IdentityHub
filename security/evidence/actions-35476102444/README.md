# Evidencia "antes": escaneo de la línea base (run 35476102444)

- Workflow: `Escaneo de la línea base` (`baseline-scan.yml`), `workflow_dispatch`, 2026-09-19.
- Run: https://github.com/jorgepaez-ops/IdentityHub/actions/runs/35476102444 (`success`, ~5 min).
- Código escaneado: tag `v0.0.0-vuln-baseline` (`053e15f`). El workflow corrió desde `main` (`acd3da8`).
- Artefacto de origen: `evidencia-linea-base` (109 KB, retención 90 días).
- Run de `CI` sobre `main` (`9f04fec`), en rojo: https://github.com/jorgepaez-ops/IdentityHub/actions/runs/35473988275

Conservado sin cambios salvo lo indicado: `gosec.json`, `semgrep.json`, `npm-audit.json`,
`trivy-config.json`, `trivy-images.txt`, `govulncheck.txt`, `hadolint.txt` (sin códigos de color).
En `gosec.json` y `trivy-config.json` se sustituyeron por `<redactado>` los valores con forma de
secreto (token, clave, contraseñas del compose). `govulncheck.json` (807 KB) no se guarda: supera el límite de 512 KB; `govulncheck.txt` lleva lo mismo.
`gitleaks.txt` no se guarda (contiene los valores en claro); `gitleaks.redacted.json` conserva solo
regla, archivo, línea y huella, con `Secret`, `Match` y `Line` redactados.

## Resultados por herramienta

| Herramienta | Resultado |
|---|---|
| Gitleaks | 28 hallazgos brutos = **12 reales** + 16 autorreferenciales (ver D1) |
| gosec | 8: G101 x2, G401, G404, G501 (todos en `legacy_auth.go`) y G104 x3 (`broker.go`) |
| Semgrep | 46: 41 `github-actions-mutable-action-tag`, 1 `run-shell-injection`, `math-random-used`, `use-of-md5`, `hardcoded-jwt-key`, 2 `request-host-used` (nginx) |
| govulncheck | 33 vulnerabilidades: 26 de la biblioteca estándar (go1.22.12) y 7 de terceros |
| npm audit | 17: 2 críticas, 10 altas, 5 moderadas |
| Hadolint | DL3008, DL3015, DL3009 (backend); DL3007 (frontend) |
| Trivy config | 6 configuraciones erróneas |
| Trivy imágenes | **sin datos** (ver D2) |

## Discrepancias frente a lo esperado

- **D1 · Gitleaks se escanea a sí mismo.** El informe se escribe en `evidence/` dentro del árbol
  escaneado (`--no-git`), así que 16 de los 28 hallazgos son del propio `evidence/gitleaks.txt`. Los
  12 reales coinciden con lo previsto (Dockerfile 3, compose 7, `legacy_auth.go` 2). Además, las
  huellas de este modo (`/repo/<ruta>:<regla>:<línea>`) **no sirven** para `.gitleaksignore` del job
  `secrets` del CI, que escanea el historial (formato `<commit>:<ruta>:<regla>:<línea>`). T31 debe
  extraerlas del run del CI, no de este.
- **D2 · Trivy sobre imágenes falló y el paso salió "success".** `No such image: baseline/api:scan`: el
  contenedor de Trivy no ve el demonio de Docker (falta montar `/var/run/docker.sock`); el
  `continue-on-error` lo ocultó. Sin evidencia automática hoy para VULN-008, VULN-016 y VULN-019, ni
  para la parte de Trivy de VULN-009 y VULN-018.
- **D3 · SQL por concatenación (VULN-005) no aparece** en gosec (no hay G201/G202) ni en Semgrep. Tampoco
  CORS comodín (VULN-007) ni JWT sin validar algoritmo (VULN-006). Pendiente: comprobar CodeQL en la
  pestaña Security; si tampoco lo ve, el "gate que lo detecta" del registro es incorrecto.
- **D4 · Hadolint no muestra DL3002 ni DL3020**, que el registro esperaba para VULN-009, VULN-011 y
  VULN-018. Pendiente de investigar (puede que el registro esté equivocado o que el Dockerfile ya no
  coincida con la ficha).
- **D5 · Hallazgos sin ficha.** 41 acciones de GitHub fijadas por etiqueta y no por SHA, más una
  posible inyección en `baseline-scan.yml:132` (cadena de suministro del propio pipeline). Además,
  `govulncheck` lista 26 vulnerabilidades de la biblioteca estándar que solo se corrigen con Go 1.25
  (Q11), y `amqp091-go` (GO-2026-6372). Requieren fichas nuevas (T0.5).
