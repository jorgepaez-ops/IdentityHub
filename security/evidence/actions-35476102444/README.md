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
  `continue-on-error` lo ocultó. (Diagnóstico inicial, corregido abajo: no era el socket.) Sin evidencia automática hoy para VULN-008, VULN-016 y VULN-019, ni
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

## Resolución de D3 y D4 y correcciones (2026-09-20)

Contrastado con los archivos de esta carpeta (`semgrep.json`, `trivy-config.json`, `gosec.json`,
`hadolint.txt`, `govulncheck.txt`, `gitleaks.redacted.json`) y con las observaciones de Claude Desktop
sobre el run en Actions.

- **D3 confirmado.** VULN-005, VULN-006 y VULN-007 no los detecta ningún gate: sin G201/G202 en gosec,
  sin regla de Semgrep y sin alerta de inyección SQL en CodeQL. Semgrep `hardcoded-jwt-key`
  (`legacy_auth.go:121`) es otro problema (VULN-001). CodeQL informa además #81 "Log entries created
  from user input" (Medium) en `legacy_auth.go:113`, sin VULN asignado (según Desktop).
- **D4 resuelto.**
  - Hadolint v2.12.0 no emite DL3002 ni DL3020. `backend/Dockerfile:44` sí contiene
    `ADD https://...`, pero Hadolint no lo marca: VULN-011 no tiene gate.
  - `USER root` (VULN-009 y VULN-018) lo detecta **Trivy config DS002** (backend y frontend).
  - Trivy config también detecta DS029 x3 (`apt-get` sin `--no-install-recommends`, relacionado con
    VULN-010) y DS031 CRITICAL (secretos en `ENV`, VULN-012).
- **VULN-024 sin gate hoy.** Los 6 hallazgos de Trivy config (DS002 x2, DS029 x3, DS031) son de los
  Dockerfiles; ninguno de `deploy/docker-compose.yml`.
- **D5 corregido.** Semgrep informa **40** `github-actions-mutable-action-tag` (no 41); el total de 46 =
  40 + 2 `request-host-used` + `run-shell-injection` + `math-random-used` + `use-of-md5` + `hardcoded-jwt-key`.
- **VULN-020.** `GO-2026-5774`, que cita la ficha, no aparece en `govulncheck.txt`; sí GO-2026-5775 y 5777.
- **VULN-001.** Los 12 hallazgos de Gitleaks son: `contrasena-en-variable-de-entorno` x5,
  `url-de-conexion-con-credenciales` x5, `aws-access-token` x1 y `slack-bot-token` x1.
- **VULN-025.** `osv-scanner` corre en `ci.yml`, no en este workflow.
- **D2 corregido.** `baseline-scan.yml` ya monta `/var/run/docker.sock` en los dos `docker run` de Trivy
  (líneas 114 y 119) y el demonio respondió `No such image: baseline/api:scan`: el socket funciona y la
  imagen no existía, es decir, el paso anterior "Construir las imágenes de la línea base" (también con
  `continue-on-error`) no la dejó disponible. La causa de ese fallo se confirma con el log de ese paso; no se
  corrige nada en Trivy hasta entonces. VULN-008, 016 y 019 siguen sin evidencia de imagen.
- **Code scanning (lectura por API, 2026-09-20).** Alertas abiertas sobre `main` (commit `acd3da8`):
  CodeQL #80 `go/weak-sensitive-data-hashing` (High, `legacy_auth.go:54`) y #81 `go/log-injection` (Medium,
  `legacy_auth.go:113`, sin VULN); Semgrep OSS con nombre de regla y línea (#42 math-random-used, #43
  use-of-md5, #44 hardcoded-jwt-key, #45 y #46 request-host-used en `default.conf`, 40
  github-actions-mutable-action-tag y 1 run-shell-injection). Además 33 alertas CodeQL
  `js/remote-property-injection` (High) en `docs/diagramas/*.html`, ajenas a los VULN de la línea base.

## Hallazgos del CI y de la reconstrucción (2026-09-20)

- **D2, causa raíz.** La imagen `api` no se construye: `apt-get install ca-certificates curl` sobre
  `debian:11-slim` devuelve 404 en `bullseye-security` (Debian 11 ya no recibe paquetes). Como los pasos
  corren con `bash -e`, ese primer fallo cortó el paso y `worker` y `web` no se llegaron a construir. Extracto en
  `docker-build-api.txt`. Es en sí evidencia de VULN-008. El `web` sí se construye (se probó en local).
- **D6 · El job `secrets` del CI no escaneó nada.** En el run 35473988275 (push inicial) falló con
  `ambiguous argument '053e15f^..9f04fec'`: `053e15f` es el primer commit y no tiene padre. Resultado:
  `scanned ~0 bytes` y sin hallazgos, con el job en rojo por otra causa. En un PR el rango sí existe.
  Las 12 huellas para T31 salen de un escaneo local del historial: `../gitleaks-huellas-historial.txt`.
- **D7 · Job "8 · Configuración de contenedores" falla en "Set up job"** (antes de ejecutar nada), así
  que Hadolint y Trivy config no corren en el CI; la evidencia de esos gates sale de este workflow.
- **D8 · El job "5 · Dependencias vulnerables" se detiene en govulncheck** (exit code 3) y `npm audit` y
  `osv-scanner` no llegan a ejecutarse.
- **Web local.** `curl -sI` y ráfaga sobre la imagen `web` del tag: `../local-web-baseline.txt` (VULN-013, 014, 015).
- **Dependabot alerts:** desactivado (captura de Claude Desktop, 2026-09-20).
