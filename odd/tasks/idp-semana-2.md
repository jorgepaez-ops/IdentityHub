# idp-semana-2 — Núcleo del IdP, contrato ejecutable y remediación con evidencia

Documento de feature (ODD). Es el único archivo de tareas de esta feature; el espejo en memoria
lo reconcilia Claude. Codex edita este archivo (casillas, `Commit:`, handoff, preguntas);
lee también `AGENTS.md` en la raíz del repo.

## Objetivo

Implementar el núcleo del proveedor de identidad (registro, verificación, login, JWT Ed25519
con JWKS, refresh rotativo con detección de reuso, RBAC, auditoría append-only, bloqueo por
fuerza bruta), activar el gate real de deriva de specs (`gen.go` con oapi-codegen) y remediar
la línea base vulnerable dejando, por cada hallazgo, evidencia documentada del antes, el commit
de remediación y el después.

## Problema y motivo

- Hoy la API solo tiene `/healthz`, `/readyz`, `/metrics` y el endpoint sembrado
  `/api/v1/auth/legacy-login`; el trabajo de semana 1 dejó 0 requisitos completos (17 parciales,
  14 sin cubrir, `specs/07-traceability.md`).
- El criterio de éxito del proyecto (`specs/00-vision.md`) exige gates que rompen la build, cada
  gate con un hallazgo real que lo justifica, y una imagen de la API sin HIGH/CRITICAL corregibles.
- Cada vulnerabilidad debe poder contarse con pruebas: alerta, antes, commit, después.

## Alcance

**Dentro:** RF-001 a RF-011 y RF-017 (P0 y el bloqueo P1), RF-012 en lo que toca a las nuevas
publicaciones y plantillas del worker, RNF-003, RNF-005, RNF-008, RNF-009, RNF-011, RNF-012, la
remediación de los VULN de la Fase 3, y la generación de código desde los specs.

**Fuera (backlog de semana 3, ver "Decisiones sobre las preguntas abiertas" Q14):** RF-013,
RF-014, RF-015, RF-016 (MFA, restablecimiento de contraseña, sesiones activas), RF-018 y RF-019.
Estos seis requisitos quedan **diferidos** (no olvidados): T34 los muestra como "diferido" en la
matriz de trazabilidad y T13 rechaza con un error claro las cuentas con MFA activado. También
fuera: E2E con Playwright, DAST/ZAP, SBOM, firma, publicación en GHCR,
`docker-compose.prod.yml` y pantallas del frontend (semana 3).

## Restricciones

- `specs/` manda; el OpenAPI (camelCase, RFC 7807) es el contrato. `gen.go` no se edita a mano.
  Las únicas enmiendas autorizadas a `specs/` en esta feature son las de "Cambios de spec
  propuestos" marcadas como aceptadas (C1 en T1a: OpenAPI; C2 en T14a: ADR 0005), cada una
  válida solo para su tarea.
- Nada se sube ni se publica desde Codex: push, PR, tags y merge los hace el usuario.
- Línea base intocable hasta su tarea `Remedia:` y hasta `T0.1` a `T0.3` marcadas (ver
  `AGENTS.md`, "Límites duros"). Las tareas `Remedia: —` pueden avanzar en la rama local
  mientras el usuario hace la Fase 0.
- Nunca debilitar un gate. Nunca secretos reales en el repo (ni URLs con credenciales ni
  asignaciones `CLAVE=valor` en docs: Gitleaks las detecta).
- Tamaño orientativo por tarea: ~400 líneas de cambio (heurística, no tope).
- Estrategia de entrega: `ask-on-risk`. Commits de unidad de trabajo en la rama local
  `feat/idp-semana-2`; el usuario decide push y PR (proponer cortes por fase).

## Modo TDD resuelto

- **TDD estricto: habilitado** (fuente: configuración del orquestador del proyecto, "Strict TDD
  Mode: enabled"). RED, GREEN y REFACTOR observados por tarea.
- **Ejecutor de pruebas:** `cd backend && go test -race ./...` (así lo hace `make test-go`, con
  `-coverprofile=coverage.out -covermode=atomic`; CI añade `-short`). Frontend: `npm run test`
  (Vitest). Integración: build tag `integration` + `TEST_DATABASE_URL`, que crea T4
  (`make test-integration`, aún no existe).
- Convención de nombres para trazabilidad: `TestRFnnn_Descripcion` / `TestRNFnnn_Descripcion`.

## Alcance autorizado

Codex puede crear y editar: `backend/**` (salvo lo sembrado, según cada tarea), `db/migrations/**`
(solo migraciones nuevas), `db/queries/**`, `sqlc.yaml`, `Makefile`, `.github/workflows/ci.yml`
(solo endurecer), `deploy/docker-compose.yml` (solo añadir, salvo tareas `Remedia:`),
`frontend/src/api/schema.d.ts` y lo que cada tarea liste, `security/findings/**`, `docs/evidencia/README.md`,
este archivo. Excepciones puntuales, cada una solo en la tarea indicada: `specs/03-api/openapi.yaml`
(solo T1a, C1), `specs/adr/0005-*.md` (solo T14a, C2), `scripts/traceability.py` (solo T34, para el
estado "diferido") y `.gitleaksignore` (solo T31, con las huellas que entregue el usuario). No puede:
modificar `specs/` fuera de C1/C2, tocar tags, escribir en `docs/evidencia/VULN-*/` (lo hace Claude:
ver "Protocolo de evidencia"), hacer capturas, ni commitear `.atl/`, `.gga`, `docs/diagramas/`.

## Criterios de aceptación de la feature

1. `make test-go`, `make test-integration`, `make test-front`, `make lint` y `cd frontend && npm run typecheck` pasan; `python3 scripts/traceability.py --check` pasa.
2. `ci.yml` en verde sobre el estado final (PR o `workflow_dispatch`), sin gates debilitados; el
   gate `spec-drift` regenera `gen.go`, `schema.d.ts` y el código de sqlc y exige árbol limpio.
3. `specs/07-traceability.md`: como mínimo RF-001 a RF-007, RF-009, RF-010, RF-011 y RF-017 en
   "completo" (Escenarios + Pruebas Go); ninguna regresión de estado.
4. Cobertura de `backend/internal/` >= 70 % y gate activo (RNF-005).
5. Los escenarios de `specs/06-acceptance/` de autenticación, registro, rotación y control de
   acceso tienen prueba Go equivalente (E2E llega en semana 3).
6. Cada VULN de la fase 3 tiene fila completa en el "Registro de evidencia" y su
   `docs/evidencia/VULN-XXX/evidencia.json` con `antes` y `despues` completos, y sus capturas antes/después
   están en el informe externo (confirmado por el usuario), o queda anotado por qué no aplica.
7. Tag del estado corregido creado por el usuario (T38).

## Protocolo de evidencia

Objetivo: documentar el ciclo de vida de cada vulnerabilidad con pruebas.

**Dos capas (Q16, 2026-09-20).** Las capturas de pantalla **no viven en el repo**: Claude Desktop
las toma navegando GitHub en la sesión autenticada del usuario (solo lectura; sin permisos de
escritura sobre el repo) y las reúne en el informe externo `Evidencias-CI-LineaBase-IdentityHub.docx`
(sin versionar, en `Claude outputs/`). Lo que sí se versiona es texto:

1. **En el repo:** `docs/evidencia/VULN-XXX/evidencia.json` por hallazgo (todos los hallazgos
   tienen ya id: Q9 asigna VULN-024 a VULN-026 a los que no lo tenían) y el artefacto ya guardado
   en `security/evidence/`. Es lo que perdura cuando caducan los logs de Actions (retención de 90 días).
2. **Fuera del repo:** el informe con las capturas `antes` y `después`. El campo `captura` de cada
   `evidencia.json` apunta a la sección del informe donde está la imagen (`null` hasta que el
   usuario confirme que la captura existe).

Datos que Desktop entrega como **texto** (no imágenes) por cada VULN: workflow, run ID/URL, job,
paso, SHA, y la alerta literal sin secretos. Con eso Claude escribe o completa el `evidencia.json`.

**Carpeta por hallazgo:** `docs/evidencia/VULN-XXX/`, con:

- `evidencia.json`: artefacto legible por máquina. Forma fija (`null` hasta conocerse):
  ```json
  {
    "vuln": "VULN-002",
    "gate": ["gosec G401 (baseline-scan)", "CodeQL", "Semgrep"],
    "antes":   {"commit_linea_base": "053e15f...", "commit_main": null, "workflow": null,
                "run_id": null, "run_url": null, "artefacto": "security/evidence/...", "captura": null},
    "remediacion": {"tarea": "T23", "commit": null, "ficha": "security/findings/VULN-002-md5-para-contrasenas.md"},
    "despues": {"commit": null, "workflow": null, "run_id": null, "run_url": null, "captura": null}
  }
  ```
  `captura` es una referencia de texto a la sección del informe externo (p. ej. `"informe §VULN-002 antes"`);
  no hay `before.png` ni `after.png` en el repo. El artefacto "antes" reutiliza lo ya versionado en
  `security/evidence/*-baseline.*` y `security/evidence/actions-<run_id>/`; los recortes SARIF/JSON
  nuevos se guardan ahí (ruta ya permitida en `.gitleaks.toml`), no en `docs/evidencia/`.
- El repositorio es **público** (Q10): la subida de SARIF a code scanning (pestaña Security) está
  disponible sin GitHub Advanced Security, así que el "antes" puede salir tanto de esa pestaña
  como del log del job y de los artefactos de Actions.
- Ficha `security/findings/VULN-XXX-*.md`: se añaden a su tabla superior las filas **Evidencia
  antes**, **Commit de remediación**, **Evidencia después** y **Run de Actions (antes/después)**
  (T0.4 actualiza la plantilla en `security/findings/README.md` y las fichas existentes).

**Reglas de captura:** ninguna captura ni JSON puede mostrar secretos reales; la salida de
Gitleaks puede revelar los valores sembrados (inventados): leer primero como texto, tapar la columna del
secreto en la imagen y borrar `Secret`/`Match` de los JSON. El informe externo no se sube al repo
(si algún día se sube, cada imagen <= 512 KB por el hook `check-added-large-files`). Los VULN
detectados solo por ZAP (007 parcialmente, 013, 014, 015) no tienen gate hoy: el "antes" es la
salida de `curl -sI http://localhost:8080` (cabeceras) sobre el tag, que ejecuta el **usuario** en
local (Desktop no puede: es un navegador), y el "después" el mismo comando; ambas salidas van al informe
y, como texto, al `evidencia.json`.

**Cómo se obtiene el "después":** `ci.yml` corre en push a `main`, en `pull_request` y en
`workflow_dispatch`; un push a una rama sin PR no lo dispara. El usuario abre un PR por corte
de fase (o lanza `CI` con `workflow_dispatch` sobre la rama) y registra run ID y URL.

**Responsabilidades:** Usuario (con Claude Desktop) = navegar GitHub, capturas y su informe, run
IDs, `curl -sI` local. Claude = convertir esos datos de texto en `evidencia.json` y commitearlos.
Codex = commit de remediación y ficha; **no** escribe en `docs/evidencia/VULN-*/` ni hace capturas.
Cada tarea `Remedia:` lleva su sub-casilla "Evidencia" a cargo de `Usuario`: se marca cuando la
captura "después" está en el informe y el `evidencia.json` tiene su `despues` completo.

**Regla de ids:** un id `VULN-NNN` solo se asigna cuando se crea su ficha en
`security/findings/`; nunca se inventa en un comentario de código, en el compose ni en un
Dockerfile. Los ids únicos ya sembrados (003, 004, 006 a 019) se conservan, no se renumeran. Los
ids nuevos salen a partir de VULN-024. **Orden obligatorio:** el comentario del compose que hoy
dice `VULN-020` (sin endurecimiento de contenedores) NO se toca antes del push de la línea base
(alteraría el estado "antes"); la equivalencia `VULN-020 (compose) = VULN-024` se documenta en la
ficha VULN-024 (T0.5) y el comentario se corrige en T30, cuando se toca el compose.

### Registro de evidencia

Gate = job de `ci.yml` o herramienta de `baseline-scan.yml` que lo detecta (fuente: cabeceras de
los archivos sembrados y fichas). Celdas vacías = aún no conocidas.

| VULN | Hallazgo | Gate que lo detecta | Antes (run / captura) | Commit remediación | Después (run / captura) | Tarea |
|---|---|---|---|---|---|---|
| VULN-001 | Credenciales incrustadas (código, compose, Dockerfile) | Gitleaks (`secrets`), gosec G101 (`lint`), Trivy secret (imagen) | run 35476102444 / informe §3 | | | T23, T26 |
| VULN-002 | MD5 para contraseñas | gosec G401/G501, CodeQL, Semgrep | run 35476102444 / informe §3 | | | T23 (con T7) |
| VULN-003 | Credenciales en el compose | Gitleaks | run 35476102444 / informe §3 | | | T26 |
| VULN-004 | `math/rand` para tokens | gosec G404 | run 35476102444 / informe §3 | | | T23 |
| VULN-005 | SQL por concatenación | Ninguno hoy (D3 confirmado: sin G201/G202, Semgrep ni CodeQL) | run 35476102444 (no detectado) / informe §3 | | | T23 (con T5) |
| VULN-006 | JWT sin validar algoritmo | Ninguno hoy (D3 confirmado) | run 35476102444 (no detectado) / informe §3 | | | T23 (con T8) |
| VULN-007 | CORS comodín con credenciales | Ninguno hoy (D3 confirmado; ZAP en semana 3) | run 35476102444 (no detectado) / informe §3 | | | T23 |
| VULN-008 | Base Debian 11 (backend) | `docker build` (falla: Debian 11 sin paquetes) y Trivy image sobre `debian:11-slim` | run 35476102444 / informe §3 | | | T27 |
| VULN-009 | `USER root` (backend) | Trivy config DS002 (Hadolint no emite DL3002, D4); Trivy image pendiente (D2) | run 35476102444 / informe §3 | | | T27 |
| VULN-010 | `apt-get` sin fijar ni limpiar | Hadolint DL3008/DL3009 | run 35476102444 / informe §3 | | | T27 |
| VULN-011 | `ADD` desde URL remota | Ninguno hoy (Hadolint no marca un `ADD` con URL, D4) | run 35476102444 (no detectado) / sin captura | | | T27 |
| VULN-012 | Secreto en `ENV` | Gitleaks, Trivy config DS031 | run 35476102444 / informe §3 | | | T26 |
| VULN-013 | Sin CSP, HSTS, X-Frame-Options, nosniff | ZAP (semana 3; sin gate hoy) | local (`security/evidence/local-web-baseline.txt`) / sin captura | | | T29 |
| VULN-014 | `server_tokens on` | ZAP (sin gate hoy) | local (`security/evidence/local-web-baseline.txt`) / sin captura | | | T29 |
| VULN-015 | Sin `limit_req` en `/api/v1/auth/*` | AM-001/AM-017 (sin gate hoy) | local (`security/evidence/local-web-baseline.txt`) / sin captura | | | T29 |
| VULN-016 | `node:18-bullseye` | Trivy image sobre `node:18-bullseye` (por nombre) | run 35534898422 / informe §3 | | | T28 |
| VULN-017 | `nginx:latest` | Hadolint DL3007 | run 35476102444 / informe §3 | | | T28 |
| VULN-018 | Imagen final del frontend como root | Trivy config DS002 (Hadolint no emite DL3002, D4) | run 35476102444 / informe §3 | | | T28 |
| VULN-019 | Imágenes base antiguas en compose | Trivy image sobre las imágenes del compose (por nombre) | run 35534898422 / informe §3 | | | T30 |
| VULN-020 | `RealIP` de chi suplantable (GO-2026-5774/5775/5777) | govulncheck (`sca`) | run 35476102444 / informe §3 | | | T6 |
| VULN-021 | `golang-jwt/jwt/v4` (GO-2024-3250, GO-2025-3553) | govulncheck | run 35476102444 / informe §3 | | | T23 |
| VULN-022 | pgx 5.5.1 (GO-2024-2606) | govulncheck | run 35476102444 / informe §3 | | | T24 |
| VULN-023 | Reglas por defecto de Gitleaks insuficientes | comparación manual (ya `remediado`) | sin gate; sin run / sin captura | | | T31 (revisión) |
| VULN-024 | Sin endurecimiento de contenedores (en el comentario del compose figura como VULN-020) | Ninguno hoy (Trivy config solo cubre los Dockerfiles, no el compose) | run 35476102444 (no detectado) / sin captura | | | T30 |
| VULN-025 | axios 0.21.1 / lodash 4.17.15 | npm audit (baseline-scan), osv-scanner (solo en `ci.yml`) | run 35476102444 / informe §3 | | | T25 |
| VULN-026 | `golang.org/x/text` (GO-2026-5970) | govulncheck | run 35476102444 / informe §3 | | | T24 |

Nota (Q9): `VULN-020` queda como el hallazgo de chi `RealIP`. VULN-024 a VULN-026 se asignan al
crear sus fichas (T0.5); las fichas de 003, 004 y 006 a 019 también las crea T0.5, conservando sus
ids. Las filas "Antes" del compose y de las dependencias se rellenan con los runs de T0.2.

## Estructura de código propuesta (ajustable; documenta desviaciones en el handoff)

Arquitectura hexagonal ligera: dominio y casos de uso en `backend/internal/auth/**` (sin
dependencia de chi ni de pgx; puertos como interfaces), adaptadores HTTP en `internal/api`
(implementan `ServerInterface` generada), adaptadores de datos en `internal/store` (envuelven
lo generado por sqlc), eventos en `internal/events` (ya existe). Pruebas unitarias con
dobles de los puertos; pruebas de integración con PostgreSQL real (tag `integration`).

---

## Fase 0 — Línea base y evidencia "antes" (Claude, con autorización explícita del usuario para el push; sin código)

**Puerta T0:** `T0.1` a `T0.3` marcadas antes de que Codex empiece cualquier tarea `Remedia:`.
`T0.4` (Codex, solo docs) puede hacerse antes; `T0.5` tras `T0.2`. Codex puede empezar por T1a y
la Fase 1 (ninguna es `Remedia:`) solo cuando el usuario haya commiteado estos documentos
(`AGENTS.md`, `odd/`).

### T0.1 — Crear el repo en GitHub y subir `main` y el tag
- [x] Estado · Ejecutor: `Claude`, solo tras autorización explícita del usuario (destino, operación y credencial SSH) · Cubre: RNF-002 · Remedia: —
- **Hecho 2026-09-19.** `git ls-remote --heads --tags origin` lista `refs/heads/main` (`9f04fec`) y `refs/tags/v0.0.0-vuln-baseline` (`053e15f`). El primer push fue rechazado por GitHub push protection (Slack API Token en `legacy_auth.go:46` y `security/evidence/gitleaks-baseline.json:97-98`, commit `053e15f`); el usuario lo permitió en GitHub con la razón "It's used in tests" y el push se repitió sin `--force`. Ese bloqueo es evidencia "antes" de VULN-001 (detección por formato, ver VULN-023).
- Repo **público** (Q10): `git@github.com:jorgepaez-ops/IdentityHub.git`. Claude añade el
  remoto (`git remote add origin git@github.com:jorgepaez-ops/IdentityHub.git`) y hace el push
  (sin `--force`) únicamente cuando el usuario lo autorice; Codex no configura el remoto ni sube nada.
- Antes, commitear estos documentos (`AGENTS.md`, `odd/`, `.gga`, `docs/diagramas/`). `.atl/`
  (registro de skills local) no se versiona. AGENTS.md prohíbe a Codex commitear esos archivos de contexto.
- [ ] **Comprobación previa al push (obligatoria; el usuario confirmó el 2026-09-19 que las del compose son de prueba y no reutilizadas; queda por confirmar el resto)**: como el repo es público, y la
  línea base contiene credenciales sembradas que quedarán visibles para siempre en el historial,
  confirmar que **ninguna** es real ni está reutilizada en otro sitio: las de
  `deploy/docker-compose.yml`, el código legacy del backend (`backend/internal/api/legacy_auth.go`),
  los `ENV` de `backend/Dockerfile` y los valores de `.env.example`. Si alguna coincide con una
  credencial real o reutilizada, rotarla antes del push y no subir hasta que esté resuelto.
- Crear repo vacío, subir `main` y el tag existente `v0.0.0-vuln-baseline` (apunta a `053e15f`;
  `main` tiene 3 commits más, solo docs/evidencia).
- Verificación: `git ls-remote --heads --tags origin` lista `main` y el tag; la pestaña Actions
  muestra ejecuciones de `CI` (push a `main`) y `Escaneo de la línea base` (push del tag).
- Commit: —

### T0.2 — Ejecutar los tres workflows y guardar los resultados
- [x] Estado · Ejecutor: `Claude` (con `gh`/navegador según autorización del usuario) · Cubre: RNF-002, RNF-004 · Remedia: —
- `CI` sobre `main` (esperado: en rojo). `Escaneo de la línea base` sobre el tag
  (`workflow_dispatch` con `ref` = `v0.0.0-vuln-baseline` si hace falta relanzar). `Escaneo
  semanal` con `workflow_dispatch`.
- Anotar en el "Registro de evidencia" run ID y URL de cada uno y el SHA sobre el que corrieron.
- Descargar el artefacto `evidencia-linea-base` (retención 90 días) y guardar los archivos
  útiles (<= 512 KB cada uno) bajo `security/evidence/actions-<run_id>/` o nombre equivalente.
- Al ser público, los SARIF subidos a code scanning también son visibles en la pestaña Security:
  anotar su URL junto al run.
- Del JSON de Gitleaks de ese run, extraer la lista de **huellas** (`Fingerprint`) de los 12
  hallazgos conocidos de la línea base (sin los campos `Secret`/`Match`) y guardarla para T31
  (p. ej. en el handoff de T31 o en un archivo de texto que el usuario entrega a Codex). Si la
  lista no tiene exactamente 12 huellas, anotar la discrepancia.
- Comprobar y anotar discrepancias entre gate esperado y gate real (p. ej. `//nolint:gosec` en
  `legacy_auth.go` puede ocultar G401/G404 en el job `lint`, mientras `baseline-scan.yml` usa
  gosec directo; el job `secrets` puede no ver lo mismo que `make scan-secrets`).
- **Avance 2026-09-19 (casilla sin marcar; quedan partes abiertas):**
  - Hecho: `CI` sobre `main` `9f04fec` = run 35473988275 (rojo, esperado); `Escaneo de la línea base`
    = run 35476102444 (sobre el tag, `success`, ~5 min; un primer run 35473991353 se colgó en Gitleaks y
    se canceló; se añadió `timeout-minutes` en `acd3da8`). Artefacto guardado, sin secretos, en
    `security/evidence/actions-35476102444/` (ver su `README.md`, con resultados y discrepancias D1-D5).
  - Gitleaks: 12 hallazgos reales, coincide con lo previsto. **Las huellas de este run no sirven para
    T31** (D1): T31 debe extraerlas del job `secrets` del `CI` (historial).
  - **Avance 2026-09-20:** D3 y D4 resueltas y D5 corregida (40 `github-actions-mutable-action-tag`, no 41), ver
    `security/evidence/actions-35476102444/README.md`. La columna "Gate" del registro ya refleja lo observado:
    ningún gate detecta VULN-005, 006, 007, 011 ni 024; `USER root` lo detecta Trivy config DS002, no Hadolint.
  - **Avance 2026-09-20 (2):** alertas CodeQL #80 y #81 verificadas por API (abiertas, `main` `acd3da8`). **D2
    corregido:** el workflow ya monta `docker.sock`; el fallo es la construcción de las imágenes (Trivy respondió
    `No such image`), no el socket; la causa se confirma con el log de ese paso antes de tocar el workflow.
    `Escaneo semanal`: corre solo los lunes 06:00 UTC en `main` (próximo: 2026-09-21) y abre una incidencia con
    la salida de govulncheck; no hace falta lanzarlo a mano si se acepta esperar a ese run.
  - **Avance 2026-09-20 (3):** causa de D2: la imagen `api` no se construye (Debian 11 devuelve 404 en
    `bullseye-security`), evidencia de VULN-008 (`docker-build-api.txt`). `baseline-scan.yml` corregido (`70f7d88`:
    construcción por imagen, Gitleaks fuera del árbol escaneado, Trivy también sobre las imágenes base) y relanzado
    desde `feat/idp-semana-2` (run 35534898422; ver su resultado abajo cuando termine). El `curl` local dio la
    evidencia de VULN-013, 014 y 015 (`security/evidence/local-web-baseline.txt`). Nuevos hallazgos del CI: D6 (el job
    `secrets` no escaneó nada: rango `053e15f^..9f04fec` sin padre), D7 (job 8 falla en "Set up job"), D8 (job 5 se
    detiene en govulncheck). **Las 12 huellas de T31** ya están en `security/evidence/gitleaks-huellas-historial.txt`
    (escaneo local del historial, exactamente 12, todas sobre `053e15f`). Dependabot alerts: desactivado.
  - **Avance 2026-09-20 (4):** run 35534898422 terminó en `success` con el workflow corregido: Gitleaks da 12
    hallazgos (D1 resuelto); `api` y `worker` no se construyen (exit 1, mismo 404 de Debian 11) y `web` sí;
    Trivy sobre imágenes base da cuentas para VULN-008, 016 y 019 (`security/evidence/actions-35534898422/README.md`).
    Falta el JSON de Trivy sobre `baseline/web`, que solo está en el artefacto (descarga pendiente de autorización).
  - **Cerrada 2026-09-21:** `Escaneo semanal` lanzado a mano sobre `main` con autorización del usuario: run 35547924202
    (`success`, `govulncheck` encontró 33 vulnerabilidades; ver `security/evidence/actions-35547924202/README.md`).
    **D9:** ese run no abrió la incidencia prevista porque `govulncheck | tee` sin `pipefail` termina en éxito; corregido
    con `shell: bash` en `scheduled-scan.yml`. Los tres workflows tienen run y su evidencia está guardada; D1 a D9 anotadas;
    huellas para T31 en `security/evidence/gitleaks-huellas-historial.txt`. Puerta T0 (T0.1 a T0.3) cumplida.
  - (Pendiente histórico) `Escaneo semanal` (`workflow_dispatch`, puede abrir incidencias: requiere autorización
    del usuario); URLs de SARIF en la pestaña Security; corregir `baseline-scan.yml` (Trivy sin socket de
    Docker, informe de Gitleaks dentro del árbol escaneado) y repetir el escaneo de imágenes; volcar los
    run IDs en el "Registro de evidencia"; resolver D3 y D4.
- Commit: —

### T0.3 — Evidencia "antes" por hallazgo (capturas en el informe, `evidencia.json` en el repo)
- [x] Estado · Ejecutor: `Usuario` con Claude Desktop (navegación y capturas, informe externo) y `Claude` (escribe y commitea los `evidencia.json` con los datos de texto que entrega Desktop) · Cubre: todos los VULN · Remedia: — · Depende de: T0.2 (la parte de imágenes, D2)
- **Reestructurada el 2026-09-20 (Q16):** ya no hay `before.png` en el repo (Desktop no puede escribir en él).
  Para cada fila del "Registro de evidencia": `docs/evidencia/VULN-XXX/evidencia.json` con la sección
  `antes` rellena (workflow, run ID/URL, SHA, artefacto, `captura` = sección del informe) y la captura
  "antes" presente en el informe externo, confirmada por el usuario.
- Dos pasadas: (1) los VULN con evidencia ya disponible en los runs 35473988275 (`CI`) y 35476102444
  (`baseline-scan`); (2) los que dependen de T0.2 pendiente: VULN-008, 016, 019 y la parte de Trivy de 009 y 018
  (D2: sin datos de imágenes hasta corregir `baseline-scan.yml` y repetir el escaneo). Las discrepancias D3 y D4
  se anotan como observadas (no aparece / no se detecta), nunca se inventa la alerta.
- **Avance 2026-09-20 (casilla sin marcar):** pasada 1 hecha en `31211ca`: los 26 `evidencia.json` existen con
  `antes` rellenado (o nulos con nota) a partir del run 35476102444; `captura` sigue en `null` hasta que el usuario
  confirme cada captura en el informe. Faltan: la pasada 2 (VULN-008, 016, 019 y la parte de Trivy image de 009 y
  018, tras corregir `baseline-scan.yml`), la salida local de `curl -sI` para VULN-013 a 015, el `antes` desde el
  run de `CI` (osv-scanner de VULN-025, CodeQL) y las capturas en el informe.
- **Avance 2026-09-20 (2, casilla sin marcar):** `curl` local hecho (VULN-013, 014, 015) y VULN-008 con el error de
  construcción como "antes"; Desktop ya entregó las capturas de la pasada 1 (secciones del informe por VULN, pendientes
  de que el usuario las confirme para rellenar `captura`). El "antes" del CI (VULN-025) queda como "no se ejecutó" (D8).
  Falta la parte de Trivy image de VULN-008, 009, 016, 018 y 019 (run 35534898422) y confirmar las capturas.
- Filas "sin gate hoy" (VULN-013, 014, 015): el usuario ejecuta `curl -sI http://localhost:8080` sobre el tag
  y pega la salida (texto); no las toma Desktop.
- **Cerrada 2026-09-20:** 26 de 26 `evidencia.json` válidos, todos con `antes.run_url` o una nota; 20 con captura en el informe; VULN-013, 014 y 015 con evidencia en texto (Q18); VULN-011, 024 sin gate; VULN-023 ya remediado; el historial sigue dando 12 hallazgos de Gitleaks, todos sobre `053e15f`.
- Verificación: `ls docs/evidencia/*/evidencia.json | wc -l` = 26 (uno por fila del registro); cada uno con
  `antes.run_url` o una nota que explique por qué no aplica; `python3 -m json.tool` valida cada archivo; sin
  `Secret`/`Match` ni valores con forma de secreto (`make scan-secrets` no añade hallazgos).
- Commit: —

### T0.4 — Plantilla de fichas y README de evidencia
- [x] Estado · Ejecutor: `Codex` · Cubre: ADR 0007 · Remedia: — · Solo docs
- Añadir a la plantilla de `security/findings/README.md` las filas Evidencia antes, Commit de
  remediación, Evidencia después y Run de Actions (antes/después); replicarlas (vacías) en
  las 7 fichas existentes (`VULN-001`, `002`, `005`, `020`, `021`, `022`, `023`).
- Crear `docs/evidencia/README.md` (convención de dos capas de Q16: `evidencia.json` en el repo y capturas en
  el informe externo; forma fija del JSON; reglas de captura; quién escribe qué). No mencionar `before.png` ni
  `after.png` como archivos del repo.
- Archivos: `security/findings/README.md`, `security/findings/VULN-*.md`, `docs/evidencia/README.md`.
- Criterios: las fichas mantienen su formato; ningún dato inventado.
- Verificación: `git diff --stat` solo toca esos archivos; `python3 scripts/traceability.py --check`.
- Commit: 54c3525

### T0.5 — Fichas faltantes de la línea base
- [x] Estado · Ejecutor: `Codex` · Cubre: ADR 0007 · Remedia: — · Solo docs · Depende de: T0.2
- Crear `security/findings/VULN-NNN-<slug>.md` para los ids sembrados sin ficha: 003, 004 y 006 a
  019 (16 fichas; son los ids únicos que aparecen en los comentarios de `deploy/docker-compose.yml`,
  `backend/**` y `frontend/**`, y se **conservan tal cual**), más los tres ids nuevos de Q9:
  - `VULN-024`: sin endurecimiento de contenedores (Trivy config). Hoy el comentario del compose lo
    llama `VULN-020`, id que pertenece a chi. La ficha documenta la equivalencia y que el
    comentario se corrige en T30 (aquí **no** se toca el compose ni ningún archivo de la línea base).
  - `VULN-025`: axios 0.21.1 / lodash 4.17.15 (npm audit, osv-scanner).
  - `VULN-026`: `golang.org/x/text` GO-2026-5970 (govulncheck).
  Total: 19 fichas nuevas (26 en `security/findings/` con las 7 existentes). Con la salida literal del
  escáner tomada de `security/evidence/**`, estado `abierto`, sin valores de secretos reales. No
  inventar severidades: usar la del escáner. Regla: el id solo existe desde que se crea su ficha.
- Criterios: una ficha por id, plantilla de T0.4, referencias `AM-NNN` reales de `specs/05-security/threat-model.md`;
  las filas del "Registro de evidencia" coinciden con las fichas (26 filas, VULN-001 a VULN-026).
- Verificación: `ls security/findings/VULN-*.md | wc -l` (26); `git diff --stat` no toca compose,
  Dockerfiles, `backend/` ni `frontend/`; gitleaks local (`make scan-secrets`) no añade hallazgos
  fuera de los ya conocidos (rutas de `security/findings/` están permitidas).
- Commit: 4b2d4f4

---

## Fase 1 — Contrato ejecutable (RNF-011)

### T1a — Enmendar OpenAPI: refresh token por cookie
- [x] Estado · Ejecutor: `Codex` · Cubre: RNF-009, AM-015, RF-005, RF-007 · Remedia: — · Solo spec · Depende de: —
- **Autorización explícita:** el usuario autorizó esta enmienda el 2026-09-19 (respuesta a Q3:
  transporte del refresh token por cookie `HttpOnly; Secure; SameSite=Strict`, RNF-009/AM-015).
  Por eso Codex puede editar `specs/03-api/openapi.yaml` **solo en esta tarea** (C1 en "Cambios
  de spec propuestos", aceptado). Ningún otro archivo de `specs/` se toca.
- Cambios en `specs/03-api/openapi.yaml`:
  - Retirar `refreshToken` de `TokenPair` (queda `accessToken`, `tokenType`, `expiresIn`; ajustar
    `required`) y eliminar el esquema `RefreshRequest` y los `requestBody` de `refreshSession` y `logout`.
  - `refreshSession` y `logout` reciben el token por un parámetro `in: cookie` (nombre propuesto
    `refresh_token`, obligatorio); sin la cookie responden 401 (ya declarado).
  - Documentar `Set-Cookie` (cabecera de respuesta) en `login` 200, `verifyMfa` 200 y
    `refreshSession` 200: `refresh_token=<opaco>; HttpOnly; Secure; SameSite=Strict; Path=/api/v1/auth;
    Max-Age=<vigencia del refresh>`; y en `logout` 204 la cookie de borrado (mismo nombre, `Path` y
    atributos, `Max-Age=0`). El `accessToken` sigue viajando en el cuerpo JSON. Usar `Path=/api/v1/auth`
    para que el navegador solo envíe la cookie a los endpoints de sesión.
  - Postura CSRF, en `info.description` o en la descripción de cada operación: `SameSite=Strict`, más
    `refresh` y `logout` exigen la cookie y son `POST` con respuesta JSON; el resto de la API se
    autentica con `Authorization: Bearer` (nunca cookie), así que no hay CSRF sobre ella.
  - Nota CORS: la API se sirve al frontend a través de Nginx (mismo origen); no se habilita CORS. Si
    algún día hiciera falta, `Access-Control-Allow-Credentials: true` solo con un origen explícito,
    nunca con `*` (relación con VULN-007).
  - Mantener todos los campos `x-requirement`, `operationId` y el resto del contrato intactos.
- Archivos: `specs/03-api/openapi.yaml` y este archivo. Sin código Go ni `schema.d.ts` (el frontend está
  fuera de alcance; `schema.d.ts` lo genera T2 desde el OpenAPI ya enmendado).
- Criterios: `refreshToken` ya no aparece como propiedad de ningún esquema ni cuerpo de petición; las 22
  operaciones y sus `x-requirement` se conservan; `Set-Cookie` documentado en las cuatro respuestas.
- Verificación: `python3 -m openapi_spec_validator specs/03-api/openapi.yaml`;
  `python3 scripts/traceability.py --check` (si cambia la salida generada, regenerar y commitear);
  `git diff --stat` solo toca `specs/03-api/openapi.yaml` (y `specs/07-traceability.md` si cambia).
- Commit: d9117cb

### T1 — Generar `backend/internal/api/gen.go` con oapi-codegen
- [x] Estado · Ejecutor: `Codex` · Cubre: RNF-011, ADR 0003 · Remedia: — · Depende de: T1a
- Fijar versión de oapi-codegen (registrar la exacta en el handoff), config en
  `backend/internal/api/oapi-codegen.yaml` (paquete `api`, tipos + servidor chi), salida
  `backend/internal/api/gen.go` desde `specs/03-api/openapi.yaml`.
- `Server` implementa `ServerInterface` (las 22 operaciones; las no implementadas responden
  501 problem+json, sin panics); `getHealth`/`getReadiness` siguen funcionando; se mantiene
  la ruta sembrada `/api/v1/auth/legacy-login` (no está en el contrato) sin tocarla.
- Archivos: `backend/internal/api/{gen.go,oapi-codegen.yaml,server.go,health.go}`,
  `backend/internal/api/*_test.go`, `backend/go.mod`, `backend/go.sum`.
- Criterios: `var _ ServerInterface = (*Server)(nil)` compila; `GET /healthz` -> 200; una
  operación sin implementar -> 501 con `application/problem+json`; regenerar no produce diff;
  `TokenPair` generado sin `RefreshToken` y `refreshSession`/`logout` sin cuerpo, con el parámetro de
  cookie (según T1a).
  Advertencia (decisión Q11): si `go get` sube la directiva `go` o un módulo marcado, detenerse y
  anotarlo en "Preguntas nuevas"; no subirla sin aprobación.
- Verificación: `cd backend && go build ./... && go vet ./... && go test -race -short ./...`;
  regenerar con el comando registrado y `git diff --exit-code backend/internal/api/gen.go`.
- Commit: df47361

### T2 — Gate real de deriva: `make gen`, `schema.d.ts` y `ci.yml`
- [x] Estado · Ejecutor: `Codex` · Cubre: RNF-011 · Remedia: — · Depende de: T1
- `make gen` regenera `gen.go`, `frontend/src/api/schema.d.ts` (`npm run gen:api`) y la matriz.
- `ci.yml`, job `spec-drift`: regenerar ambos y mantener `git diff --exit-code` (ya existe; el
  comentario del job pide justo esto). Solo endurecer.
- Archivos: `Makefile`, `.github/workflows/ci.yml`, `frontend/src/api/schema.d.ts`,
  `frontend/package.json` (solo si falta un script; no tocar axios/lodash).
- Criterios: tras `make gen` el árbol queda limpio; alterar `specs/03-api/openapi.yaml`
  (p. ej. cambiar un `operationId` en una copia local) y no regenerar hace fallar el paso de deriva.
- Verificación: `make gen && git diff --exit-code`; `cd frontend && npm run lint && npm run typecheck && npm run test`; `python3 scripts/traceability.py --check`.
- Commit: f3d8a0e

### T3 — Revisión de la Fase 1
- [x] Estado · Ejecutor: `Claude (revisión)` · Cubre: T1a, T1, T2
- Revisar los tres commits contra sus handoff; comprobar que el diff de T1a solo cambia lo
  autorizado en `openapi.yaml` (cookie, sin `refreshToken` en cuerpos) y que el gate de deriva falla de verdad.
- **Revisión hecha 2026-09-20.** T1a: 22 operaciones con el mismo `operationId`, método, ruta y `x-requirement`; `TokenPair` sin `refreshToken`; `refreshSession` y `logout` sin cuerpo y con el parámetro de cookie; `Set-Cookie` en login, verifyMfa, refresh y logout; la única diferencia extra es el arreglo de tres descripciones YAML mal formadas (texto conservado). T1: ver su handoff; Claude repitió build, vet y tests, comprobó la regeneración y las rutas. T2: `make gen` idempotente (mismo checksum dos veces); sonda negativa: cambiar un `operationId` en el spec altera `gen.go` y `schema.d.ts` y `git diff --exit-code` sale con 1; se restauró sin residuos. Ajuste de Claude en T2: se quitó `cache: npm` del `setup-node` del job `spec-drift`, porque `package-lock.json` aún no existe y `setup-node` falla si falta.
- **Defecto latente para T25:** el job 2 de `ci.yml` (frontend) usa el mismo `cache: npm` con `frontend/package-lock.json`; fallará hasta que T25 versione el lockfile (no se vio antes porque el job se cortaba en pasos anteriores). Al versionarlo, se puede reactivar la caché en `spec-drift`.
- Commit: —

---

## Fase 2 — Núcleo del IdP

### T4 — Infraestructura de pruebas de integración
- [x] Estado · Ejecutor: `Codex` · Cubre: RNF-005, RNF-011 · Remedia: — · Depende de: T1
- Paquete de apoyo `backend/internal/testdb` (tag `integration`): lee `TEST_DATABASE_URL`, crea
  una base temporal, aplica `db/migrations/*.up.sql` en orden con pgx (sin nuevas dependencias
  de migración) y la borra al terminar; se omite con `t.Skip` si falta la variable.
- `make test-integration` (`cd backend && go test -race -tags=integration ./...`) y job
  `test-integration` en `ci.yml` con servicio PostgreSQL (versión vigente y fijada; registrarla).
- Prueba de humo: `TestRNF011_MigracionesSeAplicanSobreBaseVacia` (aplica 000001 y comprueba
  que un hash MD5 en `users` viola `users_password_hash_is_argon2id`; invariante 1).
- Local: `docker compose -f deploy/docker-compose.yml up -d db` y `TEST_DATABASE_URL` con las
  credenciales de ese compose (puerto 5432 publicado); no copiarlas a ningún archivo versionado.
- Archivos: `backend/internal/testdb/**`, `Makefile`, `.github/workflows/ci.yml`.
- Verificación: `make test-integration` (con la base levantada); `cd backend && go test -race -short ./...` sigue verde sin base.
- Commit: 6954cba

### T5 — sqlc: configuración y consultas base
- [x] Estado · Ejecutor: `Codex` · Cubre: AM-006, RNF-011, VULN-005 (parcial, la retirada va en T23) · Remedia: — · Depende de: T4
- `sqlc.yaml` (esquema `db/migrations`, consultas `db/queries/`, driver `pgx/v5`, paquete
  generado bajo `backend/internal/store/`); fijar versión de sqlc y registrarla. Tipos: `citext`
  como `string`, `inet` como `netip.Addr`, UUID de `google/uuid` (ajustar si la generación exige otra cosa).
- Consultas iniciales: `CreateUser`, `GetUserByEmail` (parametrizada, la de la ficha VULN-005),
  `GetUserByID`, `InsertAuditEvent`. Cada tarea posterior añade las suyas y regenera.
- `make gen` y el job `spec-drift` incluyen sqlc (mismo `git diff --exit-code`).
- Archivos: `sqlc.yaml`, `db/queries/*.sql`, `backend/internal/store/**` (generado + adaptador), `Makefile`, `.github/workflows/ci.yml`, `backend/go.mod`.
- Criterios (integración): crear usuario con hash de forma Argon2id y leerlo por correo con
  distinta capitalización (citext); entrada `' OR '1'='1' --` llega como valor literal y no devuelve filas.
- Verificación: `make gen && git diff --exit-code`; `make test-integration`; `make test-go`.
- Commit: d387fed

### T6 — IP de cliente confiable y chi >= v5.3.0
- [x] Estado · Ejecutor: `Codex` · Cubre: AM-001, AM-010, frontera T2 · Remedia: VULN-020 (chi) · Bloqueada por: T0.3
- Subir `github.com/go-chi/chi/v5` a >= v5.3.0 y retirar `middleware.RealIP` de `Server.Routes`.
- Middleware propio `ClientIP`: por defecto usa `r.RemoteAddr`; solo si `RemoteAddr` está en la
  lista configurada `TRUSTED_PROXIES` (CIDR; opcional, vacío = ninguno; config validada) toma la
  IP del extremo derecho no confiable de `X-Forwarded-For`; cabecera ausente, mal formada o de
  origen no confiable se ignora. `ClientIPFrom(ctx)` para usos posteriores (auditoría, sesiones).
- Archivos: `backend/go.mod`, `backend/go.sum`, `backend/internal/api/{server.go,clientip.go,clientip_test.go}`, `backend/internal/config/**`.
- Criterios: `TestRF017_BloqueoNoSeEvitaFalsificandoXForwardedFor` (nombre exigido por la
  ficha VULN-020; en esta tarea prueba la resolución de IP, el bloqueo completo llega en T20);
  `TestRF011_LaIPDelAuditLogNoEsFalsificable`; XFF desde peer no confiable se ignora; desde
  proxy confiable se respeta; `govulncheck` ya no informa GO-2026-5774/5775/5777.
- Verificación: `make test-go`; `cd backend && go run golang.org/x/vuln/cmd/govulncheck@latest ./...` (las otras alertas siguen hasta T23/T24); `make lint`.
- Commit: 902a047
- [ ] Evidencia (`Usuario`): VULN-020 chi: captura "después" en el informe tras run verde del job `sca` sin esos avisos, y datos de texto para el `despues` de `docs/evidencia/VULN-020/evidencia.json`.

### T7 — Argon2id
- [x] Estado · Ejecutor: `Codex` · Cubre: RF-001, AM-001, AM-004, AM-017, invariante 1, ADR 0004 · Remedia: — (prepara VULN-002) · Depende de: T5
- Paquete `backend/internal/auth/password`: `Hash`, `Verify` (tiempo constante), `NeedsRehash`,
  formato PHC (`$argon2id$v=19$m=...,t=...,p=...$sal$hash`) que cumple el `CHECK` existente;
  parámetros de ADR 0004 (64 MiB, 3 iteraciones, paralelismo 2, sal 16 B `crypto/rand`, clave 32 B) en `config`;
  verificación contra hash señuelo cuando el usuario no existe; semáforo que acota las
  verificaciones concurrentes (64 MiB cada una; límite configurable, justificar el valor en el handoff).
- `golang.org/x/crypto` pasa de indirecta a directa (versión vigente sin avisos).
- Criterios: `TestRF001_ContrasenaSeGuardaConArgon2id`; hash con parámetros antiguos -> `NeedsRehash`;
  contraseña vacía/larga (>128) rechazada; (integración) el hash generado se inserta en `users`
  y un hash MD5 hexadecimal es rechazado por la restricción; ningún log imprime la contraseña.
  El manejo de la restricción `CHECK` no requiere migración: no hay usuarios previos ni filas MD5 (confirmarlo en el handoff).
- Archivos: `backend/internal/auth/password/**`, `backend/internal/config/**`, `backend/go.mod`.
- Verificación: `make test-go`; `make test-integration`; `make lint`.
- Commit: 4e8c653

### T8 — JWT Ed25519, JWKS y middleware de autenticación
- [x] Estado · Ejecutor: `Codex` · Cubre: RF-004, AM-003, RNF-003 · Remedia: — (prepara VULN-006/021) · Depende de: T1
- Añadir `github.com/golang-jwt/jwt/v5` **junto a** v4 (v4 sigue hasta T23). Paquete
  `backend/internal/auth/token`: emisión (`iss`, `sub`, `aud`, `exp` a 15 min, `iat`, `jti`, `roles`, `kid`),
  validación con `WithValidMethods([]string{"EdDSA"})`, emisor y expiración obligatorios.
- `getJwks` (`GET /.well-known/jwks.json`) según el esquema `Jwks` del OpenAPI (`kty` OKP, `crv`
  Ed25519, `x`, `kid`, `use` sig, `alg` EdDSA). Middleware `RequireAuth` que exige bearer válido.
- Configuración: clave privada obligatoria por entorno, sin valor por defecto (ausente = error
  de arranque; se acumula con los demás errores como hace `config.Load`); formato y `aud` según Q2 (decidido: semilla Ed25519 de 32 B en base64, `kid` = huella de la clave pública, `aud` configurable).
  Documentar en el handoff el comando para generar una clave de desarrollo (no commitear
  ninguna; `.gitignore` ya ignora `*.pem`, `*.key`, `.env`).
- Criterios: `TestRF004_TokenSeVerificaConLaClaveDelJWKS`; `TestRF004_RechazaTokenConAlgoritmoAlterado`
  (`alg: none` y HS256 firmado con la clave pública como secreto -> 401, AM-003);
  `TestRF004_VigenciaDe15Minutos`; `TestRNF003_FaltaClaveDeFirmaImpideElArranque`; la clave privada no aparece en logs (`config.Secret`).
- Archivos: `backend/internal/auth/token/**`, `backend/internal/api/{jwks.go,auth_middleware.go}` (+ pruebas), `backend/internal/config/**`, `backend/go.mod`.
- Verificación: `make test-go`; `make lint` (sin hallazgos nuevos de gosec).
- Commit: 5da3fcf

### T9 — Registro de auditoría: escritor y migración 000002
- [x] Estado · Ejecutor: `Codex` · Cubre: RF-011, AM-010, AM-011, invariante 5 · Remedia: — · Depende de: T5, T6
- Servicio `audit.Record` (actor, acción, recurso, IP de `ClientIPFrom`, user-agent, metadata jsonb; sin
  contraseñas, tokens ni códigos). Acciones nombradas como en RF-011 y los `.feature`
  (`login_succeeded`, `login_failed`, `refresh_reuse_detected`, `user_disabled`, ...).
- Migración `db/migrations/000002_*.up.sql` (+ `.down.sql`): crea el rol `identity_app` (idempotente,
  sin contraseña en el SQL; Q7: T26 le asigna la credencial desde `.env`), concede el mínimo necesario a cada tabla y a la secuencia de
  `audit_log` (solo `INSERT`/`SELECT` en `audit_log`) y hace `REVOKE UPDATE, DELETE ON audit_log
  FROM identity_app` **sobre** el trigger existente (BITACORA, decisión de semana 1). No se retira el trigger.
- Criterios (integración): `TestRF011_IdentityAppNoTieneUpdateNiDeleteSobreAuditLog` (con
  `has_table_privilege` y, deshabilitando los triggers en la BD de prueba, con `SET ROLE
  identity_app` para probar que solo los permisos ya bloquean, mensaje "permission denied");
  el trigger sigue bloqueando a un superusuario; la inserción funciona; el `down` revierte.
- Archivos: `db/migrations/000002_*`, `backend/internal/audit/**`, `db/queries/audit.sql`, `backend/internal/store/**`.
- Verificación: `make gen && git diff --exit-code`; `make test-integration`; `make test-go`.
- Commit: `bacd932`

### T10 — Registro de cuenta (RF-001)
- [x] Estado · Ejecutor: `Codex` · Cubre: RF-001, RF-012, AM-004, ADR 0006 · Remedia: — · Depende de: T7, T9
- Operación `register` (`POST /api/v1/auth/register`): valida (contraseña 12-128, correo <= 254,
  `displayName` 1-100; 400 problem+json con `errors[].field` = `password`), cuenta en
  `pending_verification`, hash Argon2id, evento `user.registered` con token de verificación de 32 B
  `crypto/rand` guardado como SHA-256 (24 h) y `publisher confirms` (si falla el broker: revertir la
  transacción y 503, ADR 0006). Correo existente -> 409 sin la palabra "existe", con el mismo
  trabajo de hash que uno nuevo (AM-004). Audit `user_registered`.
- Criterios: `TestRF001_RegistroDevuelve201YCuentaPendiente`; `TestRF001_ContrasenaCortaDevuelve400ConCampo`;
  `TestRF001_CorreoDuplicadoDevuelve409SinRevelarExistencia` (ambas rutas invocan el hasher una vez; sin depender de temporización);
  `TestRF001_FalloDelBrokerRevierteYDevuelve503`; (integración) fila con prefijo `$argon2id$`.
- Archivos: `backend/internal/auth/**`, `backend/internal/api/register.go` (+ pruebas), `db/queries/*.sql`, `backend/internal/events/**` (solo si falta un método de publicación).
- Verificación: `make gen && git diff --exit-code`; `make test-go`; `make test-integration`; `make lint`.
- Commit: `43d2464`

### T11 — Verificación de correo (RF-002)
- [x] Estado · Ejecutor: `Codex` · Cubre: RF-002, AM-016 · Remedia: — · Depende de: T10
- `verifyEmail` (`POST /api/v1/auth/verify-email`, cuerpo `{token}`): token de un solo uso, 24 h, hash SHA-256;
  204 y cuenta `active`; segundo uso o expirado -> 410; desconocido -> 410 o 400 (elegir y documentar sin filtrar información); evento `user.email_verified`; audit.
- Criterios: `TestRF002_VerificarActivaLaCuenta`; `TestRF002_EnlaceDeUnSoloUso` (410); `TestRF002_TokenExpiradoDevuelve410`; el token nunca se registra en logs (RNF-012).
- Archivos: `backend/internal/auth/**`, `backend/internal/api/verify_email.go` (+ pruebas), `db/queries/*.sql`.
- Verificación: `make test-go`; `make test-integration`; `make lint`.
- Commit: `4ad6038`

### T12 — Worker: plantillas por tipo de evento
- [x] Estado · Ejecutor: `Codex` · Cubre: RF-012, RF-002, RF-006, RF-017, AM-016, RNF-012 · Remedia: — · Depende de: T11
- El `deliver` actual del worker es genérico (comentario "en la semana 2 se sustituye por
  plantillas"). Crear `backend/internal/notify` (función pura que devuelve asunto y cuerpo por
  `eventType`): `user.registered` incluye el enlace de verificación (`PUBLIC_BASE_URL`, opcional,
  por defecto `http://localhost:8080`; ruta de la página según Q15: el enlace apunta a `/verify-email?token=...`, la página del SPA llega en semana 3 y aquí se verifica con `curl`), `security.refresh_reuse_detected` y
  `security.account_locked` incluyen aviso de seguridad. El worker no registra el cuerpo del mensaje ni el token.
- Criterios: `TestRF012_PlantillaDeRegistroIncluyeElEnlaceDeVerificacion`; `TestRF012_LosAvisosDeSeguridadNoIncluyenSecretos`; el worker sigue sin loguear `d.Body`.
- Archivos: `backend/internal/notify/**`, `backend/cmd/worker/main.go`, `backend/internal/config/**`.
- Verificación: `make test-go`; `make lint`; humo manual en T21.
- Commit: `f1547a0`

### T13 — Inicio de sesión (RF-003)
- [x] Estado · Ejecutor: `Codex` · Cubre: RF-003, RF-004, AM-004, RF-011 · Remedia: — · Depende de: T8, T9, T11
- `login`: solo `active` obtiene tokens (`pending_verification`, `locked`, `disabled` -> 401 genérico);
  usuario inexistente verifica contra el hash señuelo; respuesta `TokenPair` **sin** `refreshToken` en
  el cuerpo (`accessToken`, `tokenType: Bearer`, `expiresIn: 900`, según T1a); el refresh token opaco
  de 32 B `crypto/rand` (SHA-256 en BD con `family_id` nuevo) viaja en `Set-Cookie`:
  `refresh_token=...; HttpOnly; Secure; SameSite=Strict; Path=/api/v1/auth; Max-Age=<vigencia>`
  (vigencia del refresh configurable; documentar en el handoff el valor por defecto elegido, RNF-009, AM-015);
  `NeedsRehash` recalcula el hash tras login correcto; audit `login_succeeded`/`login_failed`; `last_login_at`.
- **MFA (RF-013/RF-014 diferidos a semana 3, Q14):** la respuesta 202/MFA no se implementa. Una cuenta con
  `mfa_enabled` que presenta la contraseña correcta se rechaza con un error claro y documentado
  (501 `application/problem+json` con `detail` que explique que el segundo factor aún no está
  soportado; no emite tokens ni cookie; se decide **después** de verificar la contraseña, para no
  revelar qué cuentas tienen MFA a quien no la conoce; queda en audit como `login_failed` con motivo).
- Criterios: `TestRF003_LoginCorrectoDevuelveParDeTokens` (cuerpo con `accessToken`, sin `refreshToken`;
  `Set-Cookie` con `HttpOnly`, `Secure`, `SameSite=Strict` y `Path=/api/v1/auth`);
  `TestRF003_PasswordIncorrectoDevuelve401Generico`;
  `TestRF003_CuentaSinVerificarNoEntra`; `TestRF003_LoginFallidoQuedaEnAuditoria`; mensaje idéntico para
  correo inexistente y contraseña errónea (AM-004); `TestRF003_CuentaConMFARechazadaConErrorClaro`
  (contraseña correcta + `mfa_enabled` -> error explícito, sin tokens ni cookie).
- Archivos: `backend/internal/auth/**`, `backend/internal/api/login.go` (+ pruebas), `db/queries/*.sql`.
- Verificación: `make test-go`; `make test-integration`; `make lint`.
- Commit: `8ab9409`

### T14a — Enmendar ADR 0005: retirar la ventana de gracia
- [x] Estado · Ejecutor: `Codex` · Cubre: RF-006, AM-002, ADR 0005 · Remedia: — · Solo docs · Depende de: —
- **Autorización explícita:** el usuario decidió el 2026-09-19 (Q4) que **no** hay ventana de gracia.
  Por eso Codex puede editar `specs/adr/0005-refresh-tokens-rotativos-con-familia.md` **solo en esta
  tarea** (C2 en "Cambios de spec propuestos", aceptado). Ningún otro archivo de `specs/` se toca.
- En la sección "Consecuencias" de la ADR, sustituir el punto de la ventana de gracia de 10 s por la
  decisión contraria y el riesgo residual, y anotar la enmienda (fecha 2026-09-19) en la línea de estado:
  - Decisión: **no hay ventana de gracia**. Motivos: contradice el escenario "reutilizar un refresh
    rotado revoca la familia" (que es inmediato) y "devolver el mismo par ya emitido" exigiría
    guardar el token en claro (el ADR guarda solo SHA-256).
  - Riesgo residual: dos pestañas o clientes que renuevan a la vez con el mismo token pueden provocar
    un falso positivo y cerrar la sesión (el segundo uso se interpreta como reuso). Se acepta.
  - Mitigación en el cliente (para la semana 3, frontend): un único refrescador compartido entre
    pestañas (una sola renovación en vuelo por sesión, p. ej. con Web Locks API o `BroadcastChannel`;
    las demás pestañas esperan y reutilizan el resultado).
- Archivos: `specs/adr/0005-refresh-tokens-rotativos-con-familia.md` y este archivo.
- Criterios: la ADR ya no describe la ventana de gracia como mecanismo vigente (solo como alternativa
  descartada) y documenta el riesgo residual y la mitigación; el resto de la ADR no cambia.
- Verificación: `git diff --stat` solo toca esa ADR y este archivo; `python3 scripts/traceability.py --check`.
- Commit: 67de318

### T14 — Rotación de refresh y detección de reuso (RF-005, RF-006)
- [x] Estado · Ejecutor: `Codex` · Cubre: RF-005, RF-006, AM-002, AM-015, invariante 3, ADR 0005 · Remedia: — · Depende de: T13, T14a
- `refreshSession`: lee el refresh token de la **cookie** `refresh_token` (sin cookie -> 401; el cuerpo de la
  petición ya no lleva token, T1a); rota el token (anterior `rotated`, nuevo `active`, misma `family_id`, `parent_id`);
  el índice único `refresh_tokens_one_active_per_family` garantiza un solo activo; responde `accessToken` en el
  cuerpo y el refresh nuevo en `Set-Cookie` con los mismos atributos que en login; token `rotated` presentado
  de nuevo -> 401 (y `Set-Cookie` de borrado), revocación de toda la familia en una transacción, audit
  `refresh_reuse_detected`, evento `security.refresh_reuse_detected` con `revokedCount` y la IP de confianza.
  **Sin ventana de gracia** (Q4, decidido; ver T14a). Riesgo residual documentado: dos pestañas que renuevan
  a la vez pueden causar un falso positivo y cerrar la sesión; la mitigación (refrescador único compartido) es del cliente, semana 3.
- Criterios: `TestRF005_RenovarEmiteParDistintoEInvalidaElAnterior` (el cuerpo no contiene `refreshToken`;
  el `Set-Cookie` trae un valor distinto y los atributos `HttpOnly`, `Secure`, `SameSite=Strict`);
  `TestRF005_SinCookieDevuelve401`; `TestRF006_ReusoDeTokenRotadoRevocaLaFamilia`
  (el vigente también queda revocado; sin tolerancia para el reuso inmediato); carrera de dos renovaciones
  concurrentes con el mismo token: exactamente una gana y nunca quedan dos activos (integración; el perdedor recibe 401 y,
  al no haber ventana de gracia, se trata como reuso: la familia queda revocada; es el riesgo residual documentado, no un defecto).
- Archivos: `backend/internal/auth/**`, `backend/internal/api/refresh.go` (+ pruebas), `db/queries/*.sql`.
- Verificación: `make test-go`; `make test-integration`; `make lint`.
- Commit: 110867f
- **Corrección (T16, 48597d3):** `Server.RefreshSession` quedó como stub `501` sin componer hasta T16; ver esa entrada y la nota de "Notas de handoff Codex" bajo T16 para el detalle completo. La lógica y las pruebas originales de esta tarea seguían siendo correctas; solo faltaba el cableado al router real.

### T15 — Cierre de sesión (RF-007)
- [x] Estado · Ejecutor: `Codex` · Cubre: RF-007 · Remedia: — · Depende de: T14
- `logout` (`POST /api/v1/auth/logout`, sin cuerpo; el token viene en la cookie `refresh_token`, T1a): revoca el token (204) y
  responde `Set-Cookie` de borrado (`refresh_token=; Max-Age=0` con el mismo `Path` y atributos `HttpOnly; Secure; SameSite=Strict`);
  refresh posterior con la cookie vieja -> 401; sin cookie o token desconocido -> 401; audit `logout`.
- Criterios: `TestRF007_LogoutRevocaElRefreshToken` (verifica el `Set-Cookie` de borrado); `TestRF007_LogoutDeTokenDesconocidoDevuelve401`;
  `TestRF007_LogoutSinCookieDevuelve401`; `TestRF007_RefreshTrasLogoutDevuelve401`.
- Archivos: `backend/internal/auth/**`, `backend/internal/api/logout.go` (+ pruebas), `db/queries/*.sql`.
- Verificación: `make test-go`; `make test-integration`.
- Commit: bf869b2
- **Corrección (T16, 48597d3):** `Server.Logout` quedó como stub `501` sin componer hasta T16; ver esa entrada y la nota de "Notas de handoff Codex" bajo T16 para el detalle completo. La lógica y las pruebas originales de esta tarea seguían siendo correctas; solo faltaba el cableado al router real.

### T16 — Perfil propio (RF-008)
- [x] Estado · Ejecutor: `Codex` · Cubre: RF-008 · Remedia: — · Depende de: T8, T13
- `getCurrentUser` y `updateCurrentUser` (solo `displayName`, 1-100): `RequireAuth`; sin token 401; nunca devuelve `password_hash`
  (invariante 1); esquema `User` del OpenAPI (incluye `roles` leídos de BD).
- Criterios: `TestRF008_MeDevuelveElUsuarioAutenticado`; `TestRF008_SinTokenDevuelve401`; `TestRF008_ActualizaElNombre`; el JSON no contiene el hash.
- Archivos: `backend/internal/api/me.go` (+ pruebas), `db/queries/*.sql`.
- Verificación: `make test-go`; `make test-integration`.
- Commit: b9c3712

### T17 — RBAC (RF-009)
- [x] Estado · Ejecutor: `Codex` · Cubre: RF-009, AM-007, AM-021 · Remedia: — · Depende de: T8, T13
- Middleware `RequireRole("admin")` en `/api/v1/admin/*`: roles leídos de la BD en cada petición (AM-007),
  no del claim; token con `roles` alterado -> 401 (firma inválida); `user` -> 403; suspendido/deshabilitado -> 401.
  Sin endpoint de auto-asignación de roles.
- Criterios: `TestRF009_UserRecibe403EnRutasAdmin` (tabla que recorre **todas** las rutas `/admin/*` del OpenAPI, AM-021);
  `TestRF009_AdminRecibe200`; `TestRF009_RolesAlteradosEnElTokenDanEl401`; un usuario cuyo rol cambió en BD pierde el acceso sin esperar la expiración.
  Cómo nace el primer admin: Q5 (decidido: alta manual por SQL en desarrollo, documentada; los tests crean admins directamente en BD).
- Archivos: `backend/internal/api/rbac.go` (+ pruebas), `db/queries/*.sql`.
- Verificación: `make test-go`; `make test-integration`; `make lint`.
- Commit: 4e26918

### T18 — Administración de usuarios (RF-010)
- [x] Estado · Ejecutor: `Codex` · Cubre: RF-010, RF-011, AM-006, invariante 6 · Remedia: — · Depende de: T17
- `listUsers` (`q` parametrizado con `LIKE` seguro, `status`, cursor, `limit` 1-100), `getUser`,
  `updateUser` (`status`, `roles`): un admin no puede deshabilitarse a sí mismo (400); nunca queda
  sin admin activo (mecanismo según Q6: comprobación en el servicio con transacción y bloqueo de fila, y trigger en migración nueva si es viable); audit `user_disabled` con el actor; `role_changed` al cambiar roles; un usuario deshabilitado no puede iniciar sesión ni renovar.
- Criterios: `TestRF010_AdminDeshabilitaYElUsuarioNoEntra`; `TestRF010_AdminNoPuedeDeshabilitarseASiMismo` (400); `TestRF010_BusquedaNoEsInyectable` (`' OR '1'='1' --` como `q`); `TestRF010_NoSeDejaElSistemaSinAdmin`.
- Archivos: `backend/internal/auth/**`, `backend/internal/api/admin_users.go` (+ pruebas), `db/queries/*.sql`, posible migración `000003_*` según Q6.
- Verificación: `make gen && git diff --exit-code`; `make test-go`; `make test-integration`; `make lint`.
- Commit: ac3ea8e

### T19 — Consulta del registro de auditoría (RF-011)
- [x] Estado · Ejecutor: `Codex` · Cubre: RF-011, AM-010 · Remedia: — · Depende de: T9, T17
- `listAuditLog` (filtros `action`, `actorId`, `since`, `limit`, `cursor`; solo admin), paginación por `id`.
- Criterios: `TestRF011_UnLoginFallidoCreaUnaFila`; `TestRF011_ListarRequiereAdmin` (403 para user);
  `TestRF011_FiltraPorAccionYActor`; `TestRF011_ElRolDeLaAplicacionNoPuedeModificarLaFila` (reutiliza el enfoque de T9 de extremo a extremo).
- Archivos: `backend/internal/api/audit_log.go` (+ pruebas), `db/queries/audit.sql`.
- Verificación: `make test-go`; `make test-integration`.
- Commit: 1ddbdd6

### T20 — Bloqueo por fuerza bruta (RF-017)
- [x] Estado · Ejecutor: `Codex` · Cubre: RF-017, AM-001, VULN-020 (chi, frontera T2), estado `locked` · Remedia: — · Depende de: T13, T6, T12
- **Decisión Q1: bloqueo por CUENTA (RF-017) y además un límite por IP con el mismo mecanismo.**
  - Por cuenta: 5 intentos fallidos en 15 minutos bloquean la cuenta 15 minutos (`users.status`/`locked_until`
    según el modelo de dominio); 423 `Locked` problem+json; la contraseña correcta también falla mientras
    dura; al expirar `locked_until` la cuenta vuelve a `active`; audit `account_locked`; evento
    `security.account_locked`. El contador cuenta `login_failed` de esa cuenta (`audit_log` o `failed_login_count`).
  - Por IP (corrige VULN-020): mismo mecanismo (ventana deslizante de `login_failed`, respuesta 423, bloqueo
    con duración fija), con la IP tomada **siempre** de `ClientIPFrom` (T6: `RemoteAddr`, o el extremo
    derecho no confiable de `X-Forwarded-For` solo si el peer está en `TRUSTED_PROXIES`). Cuenta los fallos
    de cualquier cuenta, incluidas las inexistentes, para que enumerar correos no evada el límite. Un
    índice sobre `(ip, created_at)` en `audit_log` (migración nueva) es válido si hace falta.
  - Umbrales como **constantes con nombre y configurables** en `config` (`LoginAccountMaxFailures` = 5,
    `LoginIPMaxFailures` = **20 fallos por IP en 15 min** (valor propuesto, ajustable: es alto a propósito
    para no bloquear a varios usuarios tras una misma NAT), `LoginFailureWindow` = 15 min,
    `LoginLockoutDuration` = 15 min). Documentar en el handoff el valor final de cada una.
  - Un bloqueo por IP no cambia el estado de ninguna cuenta; un bloqueo de cuenta no bloquea a la IP.
- Criterios: `TestRF017_SeisFallosDevuelven423` (5 fallos -> el sexto intento, 423);
  `TestRF017_ContrasenaCorrectaFallaDuranteElBloqueo`; `TestRF017_BloqueoSeLevantaAlExpirar` (reloj inyectable);
  `TestRF017_LimitePorIPBloqueaTrasElUmbral` (fallos sobre cuentas distintas y correos inexistentes desde una
  misma IP: al llegar a `LoginIPMaxFailures`, 423 incluso con contraseña correcta; otra IP no se ve afectada);
  `TestRF017_BloqueoNoSeEvitaFalsificandoXForwardedFor` (extremo a extremo, por cuenta: rotar
  `X-Forwarded-For` desde un peer no confiable no evita el bloqueo);
  `TestRF017_RotarXForwardedForNoEvadeElLimitePorIP` (peer no confiable que cambia `X-Forwarded-For` en cada
  intento sigue contando como una sola IP y alcanza el límite; con el peer en `TRUSTED_PROXIES`, las IP
  distintas de `X-Forwarded-For` se cuentan por separado).
- Archivos: `backend/internal/auth/**`, `backend/internal/api/login.go` (+ pruebas), `db/queries/*.sql`, posible migración `000003_*`/`000004_*`.
- Verificación: `make test-go`; `make test-integration`; `make lint`.
- Commit: 37c17a5, bd5a702

### T21 — Composición, configuración y humo con el stack
- [x] Estado · Ejecutor: `Codex` · Cubre: RNF-001, RNF-003, RF-001 a RF-011 · Remedia: — · Depende de: T10 a T20
- Cablear todo en `backend/cmd/api/main.go` (servicios, repositorios sqlc, publicador, `ClientIP`, `RequireAuth`, RBAC) y montar el
  handler generado. Añadir al compose **solo líneas nuevas** (sin tocar valores sembrados): variables
  nuevas de la API con sustitución obligatoria desde `.env` (`${VAR:?mensaje}`), `TRUSTED_PROXIES` y una subred fija de la red por defecto (Q13).
  Añadir esas variables (sin valores) a `.env.example`. Documentar en README cómo generar el `.env` local.
- Humo manual con `make up`: registrar, leer el correo en Mailpit (`http://localhost:8025`), verificar,
  login (comprobar en las cabeceras el `Set-Cookie` del refresh con `HttpOnly; Secure; SameSite=Strict` y que el
  cuerpo solo trae `accessToken`), refresh con la cookie (`curl -c`/`-b`; si curl no reenvía una cookie `Secure`
  por http, pasarla con `-H 'Cookie: ...'` y anotarlo), reuso (401 y familia revocada), logout (cookie borrada),
  admin 403/200, 6 fallos -> 423 por cuenta y ráfaga de fallos desde una IP -> 423 por IP. Pegar los `curl` y códigos observados en el handoff.
- Criterios: `make up` levanta 6 contenedores sanos; `/.well-known/jwks.json` responde a través de Nginx; la API no arranca sin la clave de firma.
- Verificación: `make up && make ps`; `make test`; `make lint`; `make down`.
- Commit: ba0ce22 (+ c6d468f, hallazgo crítico de verificación)

### T22 — Revisión de la Fase 2
- [x] Estado · Ejecutor: `Claude (revisión)` · Cubre: T4 a T21
- Revisar commits y handoff; ejecutar `make test-integration` y el humo; auditar los criterios de
  `AGENTS.md`, "Seguridad del núcleo IdP".
- Commit: —

---

## Fase 3 — Remediación de la línea base (todas `Remedia:`; bloqueadas por T0.3)

Cada tarea de esta fase: el `Commit:` es el commit de remediación; después Codex actualiza la
ficha (T36). Cada `Evidencia` la completa el `Usuario` tras el run verde del gate: captura "después"
en el informe externo (Desktop) y datos de texto para el `despues` del `evidencia.json` (lo escribe Claude).

### T23 — Retirar `legacy_auth.go` y su ruta
- [x] Estado · Ejecutor: `Claude` (Codex bloqueado por fallos de autenticación 401 repetidos, ver handoff) · Cubre: AM-003, AM-006, AM-012, RF-004 · Remedia: VULN-001 (código), VULN-002, VULN-004, VULN-005, VULN-006, VULN-007, VULN-021 · Bloqueada por: T0.3, T22
- Borrar `backend/internal/api/legacy_auth.go` entero, la ruta `/auth/legacy-login` de `server.go` y
  el `require github.com/golang-jwt/jwt/v4` (`go mod tidy`); el reemplazo ya existe (T5 sqlc, T7
  Argon2id, T8 JWT/JWKS con jwt v5, `crypto/rand` en los tokens opacos). Un solo commit: el
  archivo se elimina completo (así lo dicta su encabezado y la ficha VULN-001); todos esos VULN
  comparten SHA de remediación.
- Criterios: `TestRF004_LaRutaLegacyYaNoExiste` (404); `grep -rn "legacy\|md5\|math/rand" backend --include=*.go` sin resultados de código de producción; `go.mod` sin `jwt/v4`; no hay CORS comodín.
- Verificación: `make test-go`; `make test-integration`; `make lint` y `cd backend && golangci-lint run ./...` sin hallazgos G101/G201/G401/G404; `govulncheck` sin GO-2024-3250/GO-2025-3553.
- Commit: `51a7a4f`
- Evidencia (`Usuario`), una casilla por carpeta, todas tras run verde de `CI`:
  - [ ] VULN-001 (parcial, falta `docker-compose.yml`/`Dockerfile`)  - [x] VULN-002  - [x] VULN-004  - [x] VULN-005  - [x] VULN-006  - [x] VULN-007  - [x] VULN-021

### T24 — Dependencias Go: pgx y `golang.org/x/text`
- [x] Estado · Ejecutor: `Claude` (Codex quedó bloqueado por DNS del sandbox: no resolvía `proxy.golang.org`, sin tocar ningún archivo) · Cubre: AM-006, AM-009, RNF-004 · Remedia: VULN-022 (pgx >= 5.5.4, mejor la última estable), VULN-026 (x/text GO-2026-5970 -> v0.39.0 o superior) · Bloqueada por: T0.3
- Subir pgx y x/text, `go mod tidy`; revisar el resto de avisos de `govulncheck`. Si esto obliga a subir la
  directiva `go` o `GO_VERSION` (ci.yml, baseline-scan.yml, scheduled-scan.yml usan 1.22), detenerse y
  preguntar (decisión Q11: anotarlo en "Preguntas nuevas"); no subirla sin aprobación. Retirar el comentario de línea base de `go.mod` solo al cerrar sus VULN.
- Archivos: `backend/go.mod`, `backend/go.sum`.
- Criterios: `govulncheck ./...` sin avisos alcanzables; `TestRF012_*` y el resto siguen verdes.
- Verificación: `make test-go`; `make test-integration`; `cd backend && go run golang.org/x/vuln/cmd/govulncheck@latest ./...`.
- **Hecho 2026-09-26.** `pgx` v5.5.1 -> v5.11.0; `golang.org/x/text` v0.14.0 -> v0.41.0 (no v0.42.0:
  esa versión sube la directiva `go` a 1.26.0, fuera de alcance de esta tarea — Q11 **no se activó**
  porque `go.mod` ya declaraba `go 1.25` desde T6, y v0.41.0 sigue siendo compatible con 1.25.0).
  `go mod tidy` sin cambios adicionales; comentario de línea base de `go.mod` actualizado para reflejar
  el cierre de VULN-022 y VULN-026 (VULN-021 ya estaba anotado desde T23).
  `make test-go`, `make test-integration`, `make lint` y `python3 scripts/traceability.py --check`
  en verde. `govulncheck ./...` ya no reporta GO-2024-2606 ni GO-2026-5970 como alcanzables; sigue
  reportando GO-2026-6372 (`github.com/rabbitmq/amqp091-go` v1.9.0, corregido en v1.13.0) como
  alcanzable — **fuera de alcance de T24**, sin VULN asignado todavía; anotado aquí para que se le
  asigne ficha en una tarea futura.
- Commit: 001a489
- Evidencia (`Usuario`):  - [ ] VULN-022  - [ ] VULN-026 (x/text)

### T25 — Dependencias del frontend
- [x] Estado · Ejecutor: `Claude` (red saliente necesaria para `npm install`; por la regla nueva de esta feature, no se delegó a Codex — ver CLAUDE.md) · Cubre: AM-009, RNF-004 · Remedia: VULN-025 (axios 0.21.1, lodash 4.17.15) · Bloqueada por: T0.3
- Actualizar o retirar axios y lodash (verificar si `src/` los usa: `client.ts` usa `fetch`); versionar
  `frontend/package-lock.json` (hoy no está commiteado); retirar el aviso de línea base del `package.json` al cerrar.
- Criterios: `npm audit --audit-level=high` sin hallazgos; lint, tipos y tests verdes.
- Verificación: `cd frontend && npm install && npm audit --audit-level=high && npm run lint && npm run typecheck && npm run test && npm run build`.
- **Hecho 2026-09-26.** `grep -rln "axios|lodash" frontend/src` no devolvió nada (`client.ts` ya usa
  `fetch`): se **retiraron por completo** `axios`, `lodash` y `@types/lodash` de `package.json` en vez
  de actualizarlos, y se regeneró `package-lock.json` con `npm install` (ya estaba versionado desde el
  ajuste de T23). Comentario de línea base del `package.json` retirado.
  `npm run lint`, `npm run typecheck`, `npm run test` y `npm run build` en verde.
  **Criterio de aceptación no cumplido tal como está escrito:** `npm audit --audit-level=high` sigue
  saliendo en rojo (exit 1) — pero ya no por axios/lodash (confirmado: `git diff` del lockfile solo
  muestra las 37 líneas de baja de esos tres paquetes, ninguna versión de otro paquete cambió). Los 15
  hallazgos que quedan (5 moderate, 8 high, 2 critical) ya estaban en el lockfile commiteado en T23
  (`55404f3`), sin relación con VULN-025: `esbuild`/`vite`/`vitest`/`@vitest/coverage-v8` (GHSA-67mh-4wv8-2f99,
  vía `vite <=6.4.2`), `minimatch` (3 ReDoS, vía `@typescript-eslint/parser` 6.16.0-7.5.0), `react-router`/
  `react-router-dom` (open redirect + deserialización, 6.0.0-7.17.0) y `undici` (12 avisos, vía
  `openapi-typescript` 5.1.1-6.7.6). Ninguno tiene ficha ni VULN-NNN asignado todavía (no se inventa
  aquí); las tres primeras familias exigen subir mayor de versión con cambios incompatibles
  (`vite@8`, `@typescript-eslint/parser@8.70.1`, `react-router-dom@7.18.4`), fuera de alcance de T25.
  Queda para que el usuario decida en qué tarea futura se documentan y remedian.
- Commit: 534f13e
- Evidencia (`Usuario`):  - [ ] VULN-025 (axios/lodash)

### T26 — Secretos fuera del compose y de los `ENV`
- [x] Estado · Ejecutor: `Codex` (compose/Dockerfile, hasta que su sandbox dio "permission denied" contra el socket de Docker al verificar) + `Claude` (rol `identity_app`, Q21, y el resto de la verificación) · Cubre: RNF-003, AM-012 · Remedia: VULN-003, VULN-012, VULN-001 (compose/Dockerfile) · Bloqueada por: T0.3, T21
- `deploy/docker-compose.yml`: todas las credenciales por `${VAR:?...}` desde `.env` (Postgres, RabbitMQ,
  DSN, migrate); `DATABASE_URL` de la API con el rol `identity_app` (T9; Q7: aquí se le asigna la credencial desde `.env`); `.env.example` sin valores.
  No renombrar aquí el comentario `VULN-020` del compose (eso es T30).
  `backend/Dockerfile`: eliminar los `ENV` con secretos (`DB_PASSWORD`, `API_SIGNING_KEY`). `README.md`/`Makefile` actualizados si mencionan credenciales.
- Criterios: `make up` falla con mensaje claro si falta una variable; con `.env` local arranca; `make scan-secrets` sin hallazgos en el árbol de trabajo (el historial: T31).
- Verificación: `make up && make ps`; `make scan-secrets`; `make test`.
- **Hecho 2026-09-26.** Codex dejó el compose/Dockerfile bien encaminados pero se detuvo con razón (Q21,
  renumerada de su "Q20" que colisionaba con la Q20 ya usada): `identity_app` es `NOLOGIN` desde T9, sin
  contraseña, y usar `POSTGRES_USER` como `DATABASE_URL` lo habría vuelto superusuario. El usuario decidió
  la opción "script en `docker-entrypoint-initdb.d`": nuevo `deploy/postgres-init/01-identity-app-role.sh`
  (montado solo en el servicio `db`), que da `LOGIN`/contraseña a `identity_app` desde `IDENTITY_APP_PASSWORD`
  la primera vez que el volumen está vacío; si ya existe un volumen viejo hay que recrearlo una vez
  (`docker compose down -v`, documentado en el README).
  El acceso de Claude a `.env`/`.env.example` está bloqueado por reglas `deny` globales del usuario
  (`Read`/`Edit` sobre `.env.*`, y hasta `cat`/`git diff` por Bash) — el usuario mismo corrió con `!` los
  comandos para escribir `.env.example` y `.env` (contraseñas con `openssl rand -base64 32`).
- Comandos y resultado observado: sin `.env`, `docker compose ps` falla nombrando cada variable que falta
  (`DATABASE_URL`, `IDENTITY_APP_PASSWORD`, `POSTGRES_USER`, etc.) — RED real del criterio. Con `.env`
  poblado y el volumen recreado: `db` queda `healthy`, el log confirma que corrió
  `01-identity-app-role.sh`, `migrate` aplica las 3 migraciones sin error, y una conexión directa como
  `identity_app` (`psql`, sin imprimir la contraseña) confirma login correcto y `SELECT` sobre `users`.
  `make scan-secrets`: 20 hallazgos, los mismos de siempre, todos en el commit de línea base `053e15f`
  (gitleaks escanea historial, no árbol de trabajo; nada nuevo del árbol actual). `make test` (Go +
  frontend) en verde. `python3 scripts/traceability.py --check`: al día.
  **`make up && make ps` con el stack completo (api/worker/web) falla**, pero no por T26: `backend/Dockerfile`
  sigue en `golang:1.22-bullseye` mientras `go.mod` exige `go >= 1.25.0` desde T6 — nadie había corrido
  `make up --build` en esta rama desde entonces. T27 ya lo tiene en su alcance ("builder con versión de
  Go acorde"); se verificó T26 arrancando solo `db`, `broker`, `mailpit` y `migrate` (sin construir
  api/worker/web), suficiente para probar el mecanismo de secretos que sí es de esta tarea.
- Commit: afab4e9
- Evidencia (`Usuario`):  - [ ] VULN-003  - [ ] VULN-012  - [ ] VULN-001 (Gitleaks/Trivy secret)

### T27 — Dockerfile del backend
- [x] Estado · Ejecutor: `Claude` (Codex no tiene el socket de Docker, ver regla en `CLAUDE.md`; esta tarea es casi toda verificación con Docker) · Cubre: RNF-004, RNF-008, AM-020 · Remedia: VULN-008, VULN-009, VULN-010, VULN-011 · Bloqueada por: T0.3, T26
- Base distroless (o equivalente sin shell) fijada por digest real (verificado con `docker`), `USER` no-root,
  sin `apt-get` en la imagen final, sin `ADD` remoto; builder con versión de Go acorde; el healthcheck del compose usa `curl`: reemplazarlo por un mecanismo sin shell (p. ej. subcomando de sonda del binario) y ajustar el compose.
- Criterios: `hadolint` sin DL3002/DL3008/DL3009/DL3020; Trivy image sin CRITICAL/HIGH corregibles (`--ignore-unfixed`); la imagen arranca como uid distinto de 0.
- Verificación: `make scan-config`; `make build && make scan-image`; usuario de la imagen con `docker inspect --format '{{.Config.User}}' <imagen>` (en distroless no hay shell para `id`).
- **Hecho 2026-09-26.** Builder `golang:1.25-bookworm` (Go 1.25.14, satisface el `go 1.25.0` de
  `go.mod`), fijado por digest real. Imagen final `gcr.io/distroless/static-debian12:nonroot`
  (también por digest) para `api` y `worker`: sin shell, sin `apt-get`, con `ca-certificates` de
  fábrica. `USER 65532:65532` explícito en ambas (defensa en profundidad: Trivy config no resuelve
  el `USER` heredado de una base referenciada solo por digest). `ADD` remoto eliminado sin
  reemplazo. El healthcheck del compose ya no usa `curl`: `cmd/api` gana un subcomando
  `healthcheck` (RED/GREEN con `httptest`, 3 pruebas nuevas `TestRNF004_*`) que se autosondea sobre
  `/healthz`, y el compose lo invoca como `["CMD", "/usr/local/bin/api", "healthcheck"]`.
  **Hallazgo real que bloqueaba el propio criterio de aceptación:** Trivy image encontró CVEs
  HIGH/CRITICAL corregibles en `golang.org/x/crypto` (familia `ssh`, aunque solo se usa `argon2`;
  el binario la arrastra completa por `go.sum`) y `github.com/rabbitmq/amqp091-go`. Se subieron a
  `v0.55.0` y `v1.15.0` respectivamente (ninguna exige `go 1.26`, verificado antes de elegir
  versión, mismo método que T24). `govulncheck` pasó de 1 a 0 vulnerabilidades alcanzables.
- Comandos y resultado observado: `make scan-config` (Trivy config + Hadolint): sin hallazgos en
  `backend/Dockerfile` (el único que queda es `frontend/Dockerfile`, fuera de alcance, T28).
  `make build && make scan-image`: `identity-hub-api` e `identity-hub-worker` en
  `Total: 0 (HIGH: 0, CRITICAL: 0)`. `docker inspect --format '{{.Config.User}}'` = `65532:65532`
  en ambas. `make up`: los seis servicios arrancan, `api` queda `healthy` con el nuevo subcomando.
  `make test` (Go + frontend) en verde; `go test -race ./...` repetido dos veces (antes y después
  de corregir `noctx`/`misspell` de `golangci-lint`) en verde. `python3 scripts/traceability.py`
  regenerado (las 3 pruebas nuevas lo requerían).
- Commit: a0c64d6
- Evidencia (`Usuario`):  - [ ] VULN-008  - [ ] VULN-009  - [ ] VULN-010  - [ ] VULN-011

### T28 — Dockerfile del frontend
- [x] Estado · Ejecutor: `Claude` (misma razón que T27: Codex no tiene el socket de Docker) · Cubre: RNF-004, RNF-008 · Remedia: VULN-016, VULN-017, VULN-018 · Bloqueada por: T0.3
- Node LTS vigente en el builder, `nginxinc/nginx-unprivileged` fijado por digest real, usuario no-root; puerto 8080 se mantiene.
- Criterios: Hadolint sin DL3007/DL3002; Trivy image sin HIGH/CRITICAL corregibles; `make up` sirve la SPA en 8080.
- Verificación: `make scan-config`; `make build`; `make scan-image`; `cd frontend && npm run build`.
- **Hecho 2026-09-26.** Builder `node:24-bookworm` (Node 24, LTS activa desde octubre de 2025),
  fijado por digest real. Imagen final `nginxinc/nginx-unprivileged:stable` (también por digest):
  corre como uid 101 por defecto y ya escucha en 8080, así que `frontend/nginx/default.conf` no
  necesitó ningún cambio. `USER 101` explícito de todas formas (mismo motivo que T27: Trivy config
  no resuelve el `USER` heredado de una base referenciada solo por digest).
- Comandos y resultado observado: `cd frontend && npm run build` en verde. `make scan-config`
  (Trivy config + Hadolint): sin hallazgos en ningún Dockerfile — con esto los dos quedan limpios
  (T27 ya había cerrado el de `backend`). `make build && make scan-image` sobre
  `identity-hub-web`: `Total: 0 (HIGH: 0, CRITICAL: 0)`. `docker inspect --format
  '{{.Config.User}}'` = `101`. `make up`: los 6 servicios arriba; `curl -sI
  http://localhost:8080/` devuelve `200 OK` con el HTML de la SPA. `make test` (Go + frontend) en
  verde.
- Commit: 8f3d461
- Evidencia (`Usuario`):  - [ ] VULN-016  - [ ] VULN-017  - [ ] VULN-018

### T29 — Nginx: cabeceras, `server_tokens` y `limit_req`
- [x] Estado · Ejecutor: `Claude` (verificación con `make up` real; mismo criterio que T27/T28) · Cubre: RNF-009, AM-001, AM-015, AM-017, frontera T2 · Remedia: VULN-013, VULN-014, VULN-015 · Bloqueada por: T0.3, T28
- `frontend/nginx/default.conf`: las cinco cabeceras de RNF-009 con `always` (CSP sin `unsafe-inline`, HSTS,
  `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`), `server_tokens off`, `limit_req` sobre
  `/api/v1/auth/`, y `X-Forwarded-For $remote_addr` (sobrescribe la cabecera del cliente, complementa T6). La CSP debe permitir el SPA de Vite ya construido.
  Nginx no debe alterar ni eliminar el `Set-Cookie` del refresh token (T1a) ni reescribir su `Path`.
- Criterios: `curl -sI http://localhost:8080/` muestra las cabeceras y no `nginx/x.y.z`; ráfaga de peticiones a `/api/v1/auth/login` recibe 429 (el OpenAPI ya declara `429`); la SPA sigue cargando;
  el `Set-Cookie` de `/api/v1/auth/login` llega intacto a través de Nginx.
- Verificación: `make up`; `curl -sI http://localhost:8080/`; ráfaga con `for i in $(seq 1 30); do curl -s -o /dev/null -w '%{http_code}\n' -X POST http://localhost:8080/api/v1/auth/login -H 'Content-Type: application/json' -d '{}'; done`.
- **Hecho 2026-09-26.** Las cinco cabeceras de RNF-009 con `always` (CSP sin `unsafe-inline`, HSTS,
  `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`); `server_tokens off`;
  `limit_req_zone` (5r/s) más una `location /api/v1/auth/` nueva (más específica que `/api/`, nginx
  la prefiere por prefijo más largo) con `limit_req zone=auth burst=5 nodelay` y `limit_req_status
  429` (el default de nginx es 503; el contrato OpenAPI ya declara 429). `X-Forwarded-For` pasó de
  `$proxy_add_x_forwarded_for` (anexa, permite que el cliente se autodeclare un salto previo) a
  `$remote_addr` (lo sobrescribe) en ambas locations de proxy, complementando `TRUSTED_PROXIES` de T6.
- Comandos y resultado observado: `curl -sI http://localhost:8080/` contra el stack real: las cinco
  cabeceras presentes, `Server: nginx` sin versión, la SPA y sus `assets/` siguen sirviéndose
  (`200`). Ráfaga real de 30 `POST` a `/api/v1/auth/login`: los primeros pasan (a la API, que
  responde `500` porque el body `{}` no es válido — ver "Dudas abiertas"), el resto vuelve `429`
  hasta que el balde se vacía. Ciclo completo real registro → verificación (token sacado de
  Mailpit) → login a través de Nginx: `200`, con `Set-Cookie: refresh_token=...; Path=/api/v1/auth;
  Max-Age=2592000; HttpOnly; Secure; SameSite=Strict` intacto, byte a byte igual al que pone
  `login.go` — Nginx no lo toca. `make test` en verde (sin cambios de Go, no aplica TDD aquí).
- Dudas abiertas: hallazgo nuevo, fuera de alcance de T29 — la API responde `500` (no `400`) a
  `POST /api/v1/auth/login` con body `{}` (JSON válido, campos ausentes). Nginx no tiene nada que
  ver; es el handler de login. No se toca aquí (T29 es solo Nginx); anotado para que se decida en
  qué tarea se corrige la validación de entrada.
- Commit: 18dea33
- Evidencia (`Usuario`):  - [ ] VULN-013  - [ ] VULN-014  - [ ] VULN-015 (`curl -sI` antes/después; sin gate hoy)

### T30 — Compose: imágenes base y endurecimiento
- [x] Estado · Ejecutor: `Claude` (mismo motivo que T27-T29: Codex no tiene el socket de Docker) · Cubre: RNF-008, RNF-004, AM-014, AM-020 · Remedia: VULN-019 (parcial: solo `postgres`/`rabbitmq`), VULN-024 (sin endurecimiento de contenedores; el comentario del compose lo llama VULN-020) · Bloqueada por: T0.3, T26, T27
- Actualizar `postgres` y `rabbitmq` a versiones con soporte, fijadas por digest real (avisar: volúmenes previos pueden requerir
  `make clean`); en `api`, `worker` y `web`: `read_only`, `cap_drop: [ALL]`, `security_opt: [no-new-privileges:true]`, `user` no-root, `tmpfs` donde haga falta;
  puertos de `db` y `broker` publicados al host en desarrollo (Q12, decidido: se mantienen y se tratan en `docker-compose.prod.yml`, semana 3; anotar).
- Corregir en el compose los comentarios que dicen `VULN-020` (cabecera y el bloque de endurecimiento) para que digan
  `VULN-024`, ya que `VULN-020` es el hallazgo de chi (Q9; la equivalencia está en la ficha VULN-024, T0.5). Esto solo se hace aquí, al tocar el compose.
- Criterios: `make up` sano con esa configuración; Trivy config sin HIGH/CRITICAL en el compose (si el escáner lo cubre; anotar si no).
- Verificación: `make up && make ps`; `make scan-config`; `make scan-image`; `make test`.
- **Hecho 2026-09-26.** `postgres:14-bullseye` → `postgres:16-bookworm` y `rabbitmq:3.11-management`
  → `rabbitmq:4-management`, ambos fijados por digest real (`make clean` antes, para no arrancar
  Postgres 16 sobre un volumen de datos de la 14). `api`, `worker` y `web`: `read_only: true`,
  `cap_drop: [ALL]`, `security_opt: ["no-new-privileges:true"]`, `tmpfs: [/tmp]` donde hacía falta
  (el usuario no-root ya lo fijan los Dockerfiles de T27/T28). Comentario del compose que llamaba
  "VULN-020" a este hallazgo corregido a VULN-024 (Q9: VULN-020 es el hallazgo de chi `RealIP`).
  Puertos de `db`/`broker` publicados al host: se mantienen a propósito (Q12, ya decidido);
  anotado inline en vez de dejarlo implícito.
- Comandos y resultado observado: `make clean` + `make up`: los 6 servicios arriba, todos
  `healthy` o corriendo. `docker inspect` en `api`/`worker`/`web`: `ReadonlyRootfs=true
  CapDrop=[ALL] SecurityOpt=[no-new-privileges:true]` en los tres. Ciclo real de extremo a extremo
  bajo esa configuración endurecida: registro → verificación (token de Mailpit) → el worker
  publica "notificación entregada" en su log → Mailpit recibe el correo. `make scan-config`: Trivy
  config no cubre `docker-compose.yml` en esta versión (solo detecta los 2 Dockerfiles, num=2) — no
  hay señal que anotar más allá de la verificación manual de arriba. `make scan-image` en `api`,
  `worker` y `web`: `Total: 0 (HIGH: 0, CRITICAL: 0)` en los tres. `make test` en verde.
- Dudas abiertas: **VULN-019 queda solo parcialmente remediado**, tal como autoriza el propio texto
  de la tarea (solo menciona `postgres` y `rabbitmq`): `axllent/mailpit`, `migrate/migrate` y las
  cuatro imágenes de observabilidad (`prometheus`, `loki`, `alloy`, `grafana`) siguen en las
  versiones antiguas de la línea base, fuera de alcance de T30. Trivy image sobre el `postgres`
  nuevo encontró un HIGH en `usr/local/bin/gosu` (binario empaquetado por la imagen oficial, 22
  CVEs corregibles) y otro en el certificado de relleno `ssl-cert-snakeoil` que trae Debian —
  ninguno de los dos lo puede corregir este proyecto (no construimos esa imagen, solo la
  referenciamos); anotado, no se inventa VULN-NNN.
- Commit: 824be7d
- Evidencia (`Usuario`):  - [ ] VULN-019  - [ ] VULN-024

### T31 — Gitleaks frente al historial: `.gitleaksignore` por huella exacta
- [x] Estado · Ejecutor: `Claude` (decisión ya tomada por el `Usuario`, Q8; verificación con `make scan-secrets`, mismo motivo que T27-T30) · Cubre: RNF-003, AM-012 · Remedia: relacionada con VULN-023 · Bloqueada por: T26 y por la lista de huellas que entrega el `Usuario` desde el run de Gitleaks de T0.2
- El job `secrets` usa `fetch-depth: 0`: los secretos sembrados permanecerán en el historial aunque T23/T26 los retiren, así que el gate no
  pasará solo. **Decisión del usuario (2026-09-19): opción (a)**, `.gitleaksignore` en la raíz con **huella exacta**, limitado a los **12 hallazgos conocidos de la línea base y a las 2 huellas de la decisión Q20 (14 en total)**. Las opciones (b) allowlist por commit y (c) reescribir historial quedan **descartadas** ((c) también por el ADR 0007: el historial es la evidencia). Esta decisión es la aprobación explícita del usuario para esta excepción concreta; no autoriza ninguna otra.
- **Q20 · APROBADA por el usuario (2026-09-21).** Al escanear el historial publicado hay **14** huellas y no 12: las 12 de la línea
  base más 2 falsos positivos nuestros, en commits ya subidos (T7, `4e8c653`: la regla de contraseñas leyó el literal de estructura
  `Password: PasswordConfig{` de `config.go`; T4, `f987772`: una nota de handoff citaba un patrón de URL con credenciales). Ya no
  están en el árbol de trabajo, pero el job `secrets` escanea el rango de commits del PR y las ve. **Decisión: opción (a)**, se
  añaden esas 2 huellas al `.gitleaksignore` con su justificación ("falso positivo en código propio, sin secreto"); la opción (b)
  (reescribir la rama) queda descartada. Esta aprobación es explícita para esas 2 huellas concretas y no autoriza ninguna otra.
  Las 14 huellas están en `security/evidence/gitleaks-huellas-historial.txt` (las 2 nuevas, en su sección final).
- Entrada: el usuario entrega a Codex la lista de las 14 huellas (12 de la línea base y las 2 de Q20; ver `security/evidence/gitleaks-huellas-historial.txt`) (`Fingerprint`, formato `commit:archivo:regla:línea`, sin `Secret` ni `Match`) tomadas del run de T0.2. Si la lista no llega, la tarea sigue bloqueada; si no son exactamente 12 o alguna no corresponde a un VULN, detenerse y anotarlo en "Preguntas nuevas".
- Crear `.gitleaksignore`: una línea por huella, cada una precedida por un comentario con su `VULN-NNN` y una justificación breve ("secreto sembrado de la línea base, ADR 0007; se conserva como evidencia del antes"). Sin comodines, sin rutas ni patrones, sin huellas adicionales; no tocar `.gitleaks.toml`.
- Criterios: `secrets` en verde; exactamente 14 huellas: las 12 de la línea base con su VULN y justificación, y las 2 de Q20 con la justificación "falso positivo en código propio"; un secreto nuevo de prueba (añadido en local y descartado, nunca commiteado) sigue siendo detectado; VULN-023 sigue `remediado` (12 de 12 con `.gitleaks.toml`).
- Verificación: `make scan-secrets` y run de `CI` job `secrets`; `grep -c '^[^#[:space:]]' .gitleaksignore` = 14.
- **Hecho 2026-09-26, en dos pasadas.** Pasada 1: `.gitleaksignore` con las 14 huellas de
  `security/evidence/gitleaks-huellas-historial.txt`, mapeadas a VULN-012 (3, `backend/Dockerfile`),
  VULN-001 (2, `legacy_auth.go`), VULN-003 (7, `docker-compose.yml`) y las 2 de Q20 (falso positivo
  en código propio). `grep -c` dio 14, pero `make scan-secrets` siguió en rojo: **10 hallazgos
  nuevos**, ninguno catalogado en el T0.2 de 2026-09-20/21.
  Revisados uno por uno: 6 son la sintaxis `${VAR:?VAR debe estar definida...}` que T21/T26
  introdujeron para las variables obligatorias del compose (el regex genérico lee
  `PASSWORD:?PASSWORD` como una asignación con secreto); 3 son asignaciones de struct de Go
  (`Password: request.Password,` en `login.go`/`register.go`, y un fixture `"password":
  "password-secret"` en `notify_test.go`); 1 es distinto de verdad — un commit de docs del
  2026-09-25 (`docs/guia-desarrollo.md:113`) copió el valor **inventado** de la línea base
  (`postgres_admin_2024`) en un ejemplo de comando, sin que nadie le asignara VULN-NNN.
  Como la aprobación de Q8/Q20 dice explícitamente "no autoriza ninguna otra", se detuvo la tarea y
  se preguntó al usuario. **Autorizado (2026-09-26): sí, las 10.** Pasada 2: se añadieron con su
  justificación (misma política de huella exacta); `.gitleaksignore` queda en 24 líneas de huella.
  `make scan-secrets`: `no leaks found` (117 commits escaneados).
  **Hallazgo real de infraestructura, fuera de alcance de T31 (no se toca `.gitleaks.toml` ni los
  hooks aquí):** al verificar que "un secreto nuevo sigue siendo detectado", se descubrió que el
  hook de pre-commit de gitleaks declarado en `.pre-commit-config.yaml` **no está instalado** —
  `.git/hooks/pre-commit` solo corre `gga run`. Un commit de prueba con un secreto con forma de
  clave de AWS pasó sin que nada lo bloqueara (commit local `6ad4324`, deshecho de inmediato con
  `git reset --hard` antes de este párrafo, nunca subido). Anotado en `CLAUDE.md`; la única
  protección real hoy es `make scan-secrets` a mano y el job `secrets` de CI.
- Commit: 8054f2e
- Evidencia (`Usuario`):  - [ ] VULN-023 (antes/después de la política acordada)

### T32 — Revisión de la Fase 3
- [ ] Estado · Ejecutor: `Claude (revisión)` · Cubre: T23 a T31
- Revisar cada remediación contra su ficha y evidencia "antes"; comprobar que ningún gate se debilitó (`git diff` de `ci.yml`, `.gitleaks.toml`, `.trivyignore`).
- Commit: —

---

## Fase 4 — Trazabilidad, cierre y "después"

### T33 — Gate de cobertura (RNF-005)
- [ ] Estado · Ejecutor: `Codex` · Cubre: RNF-005 · Remedia: — · Depende de: T22
- En `ci.yml`, job `test-unit`, convertir el "Informe de cobertura" en gate: falla si la cobertura de `backend/internal/` < 70 %
  (el comentario del propio job pide activarlo en semana 2). Solo endurecer; no bajar el umbral ni excluir paquetes para llegar.
- Criterios: con cobertura >= 70 % pasa; simulando un umbral de 99 % localmente falla.
- Verificación: `make test-go` (imprime la cobertura total); comprobación del script del gate en local.
- Commit:

### T34 — Regenerar la matriz de trazabilidad
- [ ] Estado · Ejecutor: `Codex` · Cubre: RNF-011, todos los RF · Remedia: — · Depende de: T33
- **Diferidos (Q14):** RF-013, RF-014, RF-015, RF-016, RF-018 y RF-019 van al backlog de semana 3: deben
  aparecer como **diferido** (no olvidados) en la matriz. Como `scripts/traceability.py` hoy solo conoce
  completo/parcial/sin cubrir, esta tarea lo extiende de forma mínima (excepción autorizada en "Alcance autorizado"):
  una lista explícita `DEFERRED` con esos seis ids y el motivo, el estado "diferido (semana 3)" para ellos,
  y un contador aparte en el resumen. No cambia el cálculo de los demás requisitos ni relaja el gate `--check`;
  cualquier requisito fuera de esa lista sigue mostrando su estado real.
- `python3 scripts/traceability.py`; commitear `specs/07-traceability.md` (generado, sin edición manual). Si un RF esperado
  no alcanza "completo", añadir la prueba que falta con el nombre `TestRFnnn_...`.
- Criterios: objetivo mínimo del criterio 3 de la feature; el resumen deja de decir "0 completos"; los seis
  requisitos aparecen como "diferido" y el resumen los cuenta aparte; el resto no cambia de estado por esta tarea.
- Verificación: `python3 scripts/traceability.py --check`; `grep -c 'diferido' specs/07-traceability.md`; `make test-go`.
- Commit:

### T35 — CI en verde y evidencia "después"
- [ ] Estado · Ejecutor: `Usuario` con Claude Desktop (PR, run, capturas en el informe) y `Claude` (completa los `evidencia.json`) · Cubre: todos los VULN de la Fase 3
- Abrir PR (o `workflow_dispatch`) con la rama; `CI` en verde. Registrar run ID/URL "después" en
  el "Registro de evidencia" y, como texto, en cada `evidencia.json` (`despues`, lo completa Claude con los
  datos que entrega Desktop); las capturas "después" van al informe externo (sin secretos). Prompt de Desktop
  para esta captura: el "prompt después" que Claude entrega al usuario al cerrar la Fase 3.
- Verificación: `python3 -m json.tool` valida los 26 `evidencia.json` y ninguno deja `despues.run_url` en `null`
  sin nota; Actions muestra `CI` en verde sobre el SHA final; el usuario confirma las capturas en el informe.
- Commit: —

### T36 — Fichas actualizadas
- [ ] Estado · Ejecutor: `Codex` · Cubre: ADR 0007 · Remedia: — · Solo docs · Depende de: T35
- En cada ficha: estado `remediado`, Commit de remediación (SHA real), Evidencia antes/después y Run de Actions; las 26 filas del registro (VULN-001 a VULN-026) coherentes con las 26 fichas.
- Verificación: `git diff --stat` solo en `security/findings/**` y este archivo; `python3 scripts/traceability.py --check`.
- Commit:

### T37 — Revisión final, informe y bitácora
- [ ] Estado · Ejecutor: `Claude (revisión)` · Cubre: toda la feature
- Reconciliar el espejo en memoria; actualizar `docs/security-report.html` (columna `main` ya prevista) y una entrada en `docs/BITACORA.md`; revisar el "Registro de evidencia" completo.
- Commit: —

### T38 — Etiquetar el estado corregido
- [ ] Estado · Ejecutor: `Usuario` · Cubre: ADR 0007 · Decisión pendiente
- Crear y subir el tag del estado corregido (sugerencia: `v0.1.0-hardened`; el nombre lo decide el usuario) y no tocar `v0.0.0-vuln-baseline`.
- Verificación: `git ls-remote --tags origin`.
- Commit: —

---

## Progreso

| Fase | Tareas | Hechas |
|---|---|---|
| 0 — Línea base y evidencia "antes" | T0.1 a T0.5 (5) | 5 (T0.1 a T0.5) |
| 1 — Contrato ejecutable | T1a, T1 a T3 (4) | 4 (T1a, T1, T2, T3) |
| 2 — Núcleo del IdP | T4 a T22 y T14a (20) | 20 (T4, T5, T6, T7, T8, T9, T10, T11, T12, T13, T14, T14a, T15, T16, T17, T18, T19, T20, T21, T22) |
| 3 — Remediación | T23 a T32 (10) | 9 (T23 a T31) |
| 4 — Cierre | T33 a T38 (6) | 0 |
| **Total** | **45** | **38** |

Tareas nuevas respecto a la versión anterior (43): `T1a` (enmienda OpenAPI, cookie) y `T14a`
(enmienda ADR 0005, sin ventana de gracia). Los ids existentes no cambian.

## Evidencia de verificación

(Codex añade aquí, por tarea, `<comando>: <resultado observado>` cuando cierre cada una;
las líneas de RED/GREEN van en el handoff.)

- T17 · RED (Codex): faltaban `RequireRole` y sus pruebas. GREEN (Codex): `RequireRole` releé estado y roles de BD en cada petición (nunca del claim del token, AM-007); compuesto de verdad con `RequireAuth(s.tokens)(RequireRole(s.currentUsers, "admin")(...))` alrededor de los 4 métodos admin de `Server` (siguen `notImplemented` por dentro, T18/T19 los completan), aplicando la lección de T14/T15 anotada en `CLAUDE.md`. 5 pruebas: recorrido de las 4 rutas admin reales contra `server.Routes()` con rol `user` en BD (403, pese a un claim de token con `roles: admin`); middleware directo con rol `admin` en BD (200); las 3 variantes de cuenta no activa (`locked`/`disabled`/`pending_verification`, 401); un JWT firmado con otra clave nunca llega a `RequireRole` (0 llamadas al repositorio, confirmado con contadores en el stub); y un cambio de rol en BD entre dos peticiones con el mismo token revoca el acceso en la segunda sin esperar expiración (confirma que `ListRolesForUser` se llama de nuevo, sin caché). No necesitó tocar `db/queries/*.sql`: reutiliza `GetUserByID`/`ListRolesForUser`, ya existentes desde T16. `make test-go`, `go vet -tags=integration ./...`, `python3 scripts/traceability.py --check`: PASS (Claude los repitió). `golangci-lint` local (instalado esta sesión): solo los 3 hallazgos sembrados de `legacy_auth.go`, sin hallazgos nuevos; `gen.go` sin drift. Cobertura de `internal/api` subió de 43.7 % a 62.7 %.
- T16 · RED (Codex): faltaba `Store.UpdateDisplayName` y el paquete de perfil. GREEN (Codex): `getCurrentUser`/`updateCurrentUser` compuestos directamente en `*Server` (igual que `Login`/`Register`/`VerifyEmail`, a diferencia del patrón desconectado que T14/T15 habían dejado); `make lint` y `go vet -tags=integration ./...` en verde; `gen.go` conservó `v2.5.1`. Claude encontró dos cosas al revisar: (1) `Store.UpdateDisplayName` usaba SQL parametrizada directa contra el pool en vez de la consulta sqlc que Codex sí había añadido a `db/queries/users.sql` pero nunca generó (`sqlc generate` no se había corrido); Claude regeneró y reescribió el método para usar `generated.Queries.UpdateDisplayName`, consistente con el resto del paquete. (2) Al revisar `server.go` para componer T16, confirmó que `Server.RefreshSession` y `Server.Logout` (T14, T15) seguían siendo stubs `501` sin conectar al router real — ver la entrada siguiente. `make test-go`, `make lint`, `python3 scripts/traceability.py --check`: PASS. Levantó Postgres local y corrió `go test -race -tags=integration ./...` completo (con la corrección de T14/T15 ya aplicada): PASS.
- **Corrección de composición T14/T15 (Claude, hallada al revisar T16) · 2026-09-25 · 48597d3**: `Server.RefreshSession` y `Server.Logout` seguían devolviendo `501 Not Implemented` a través del router real, pese a que T14 y T15 estaban cerradas y sus pruebas pasaban. Causa: T14/T15 construyeron `NewRefreshHandler`/`NewLogoutHandler` como `http.Handler` independientes, probados de forma aislada con `httptest`, pero nunca cableados a `*Server` — a diferencia de T10 (`Register`), T11 (`VerifyEmail`) y T13 (`Login`), que implementan su método de `ServerInterface` directamente en `*Server` y por eso sí quedaron compuestos desde su propio commit. Claude asumió incorrectamente en el handoff de T14/T15 que esa composición era de T21 ("igual que T13"), sin verificar que T13 en realidad ya lo hacía. Un segundo problema, más sutil, apareció al corregir esto: el router generado por oapi-codegen extrae la cookie `refresh_token` **antes** de llamar al método de `Server` (para eso existe `RefreshSessionParams`/`LogoutParams`); si falta, invoca su `ErrorHandlerFunc` por defecto (400 genérico) sin llegar nunca al handler — así que ni siquiera reescribir `Server.RefreshSession`/`Logout` alcanzaba para el caso "sin cookie", exigido como 401 por RF-005/RF-007. Se agregó `handleBindingError` en `Routes()` (usa `HandlerWithOptions` en vez de `HandlerFromMux`) que mapea ese fallo de binding específico del parámetro `refresh_token` al mismo 401 con cookie borrada que un token inválido o reusado. `refresh_test.go`/`logout_test.go` se reescribieron para probar contra `server.Routes().ServeHTTP(...)` (como ya hacía T16), no contra los constructores aislados eliminados — así es como se confirmó que la versión anterior pasaba sus propias pruebas mientras el router real devolvía 501/400. Verificación: `make test-go`, `make lint`, `go vet -tags=integration ./...`, `python3 scripts/traceability.py --check`: PASS; `go test -race -tags=integration ./...` completo contra PostgreSQL real: PASS (incluidas las pruebas de integración de T14 y T15, sin cambios en su lógica de servicio). No se fabricó evidencia RED formal para el caso "sin cookie -> 400 en vez de 401": se verificó leyendo el código generado (`ChiServerOptions{}.ErrorHandlerFunc` por defecto en `gen.go`) en vez de revertir y re-ejecutar, documentado aquí explícitamente por transparencia.
- T15 · RED (Codex): faltaba el paquete `logout` (falla de build antes de crearlo). GREEN (Codex): handler y servicio compilaron, pasaron sus pruebas aisladas, `make lint` y `go vet -tags=integration ./...` en verde; `gen.go` conservó `oapi-codegen v2.5.1` (Codex ya sabía del problema de T14 y verificó). Claude encontró que el paquete `backend/internal/auth/logout` no tenía ninguna prueba propia sin la etiqueta `integration` — solo lo ejercían el stub del handler HTTP y la prueba de integración contra Postgres real, así que `make test-go` corría ese paquete con 0 % de cobertura pese a que la tarea exige verificarlo con ese comando; escribió `logout_test.go` (patrón `repositoryStub`, igual que `refresh_test.go` de T14): revoca y audita en éxito, `pgx.ErrNoRows` mapea a `ErrInvalidRefreshToken` en token desconocido, sin token también. `make test-go`, `make lint`, `python3 scripts/traceability.py --check`: PASS (Claude los repitió). Levantó Postgres local y corrió `go test -race -tags=integration ./...` completo: PASS, incluida `TestRF007_RefreshTrasLogoutDevuelve401` (logout revoca, el refresh posterior con la misma cookie devuelve `ErrInvalidRefreshToken`, exactamente un audit `logout`).
- T14 · RED (Codex, primer intento): `undefined: refresh.Writer`/símbolos faltantes. GREEN parcial (Codex, primer intento): servicio y handler compilaban y pasaban en aislado, pero Codex se detuvo sin cerrar la tarea (interpretó la lista "Archivos" de la propia tarea, que no mencionaba `backend/internal/store`, como una restricción real) y dejó la prueba de carrera contra un harness en memoria, no PostgreSQL. Claude aclaró el alcance (el paquete `store` sí estaba autorizado, igual que en T13) y reanudó el mismo hilo de Codex. RED/GREEN (Codex, segundo intento): añadió `store/refresh.go` (mismo patrón que `store/login.go`, con `generated.New(tx)` y una única consulta `RotateRefreshToken` con CTEs y `FOR UPDATE`), corrigió el handler para usar `requestClientIP(r)` en vez de `RemoteAddr` (bug real que Claude encontró al revisar el diff, T6), y reemplazó el harness en memoria por `TestRF006_DosRenovacionesConcurrentesUnaGana` contra PostgreSQL real vía `testdb.New`/`store.NewWithPool`; dejó la prueba compilando sin poder correrla en su sandbox (`go vet -tags=integration ./...` limpio) y así lo reportó. `make gen`: Codex usó un binario de oapi-codegen cacheado sin el ldflag de versión y `gen.go` quedó con el comentario "(devel)" en vez de "v2.5.1"; Claude lo regeneró con el binario correcto (`~/go/bin/oapi-codegen`) y quedó sin diff. `make test-go`, `make lint`, `go vet -tags=integration ./...`: PASS (Claude los repitió). Levantó Postgres local (`docker compose up -d db`) y corrió `TestRF006_DosRenovacionesConcurrentesUnaGana` real: PASS (12–13 s), 0 tokens activos y 1 audit `refresh_reuse_detected` tras la carrera; `go test -race -tags=integration ./...` completo: PASS. `python3 scripts/traceability.py --check`: matriz al día. Ver también "Ajuste de Claude" abajo por dos hallazgos adicionales de revisión (token inexistente, fallo de publicación) confirmados por el hook GGA antes del primer intento de commit.
- T13 · GREEN (Codex): 8 pruebas nuevas de `TestRF003_*`/`TestAM004_*` en verde (servicio, API y las dos de AM-004, luego renombradas por Claude); `make test-go`: PASS; `make lint`: solo los 3 hallazgos sembrados de `legacy_auth.go`. `gofmt -l` (Claude): limpio. `make gen` repetido: sin diff adicional. Claude encontró que las 2 pruebas de AM-004 se llamaban `TestAM004_...` (no `TestRF003_...`), un patrón que `scripts/traceability.py` no cuenta (`func Test(RF|RNF)NNN_...`); las renombró a `TestRF003_AM004...`. También encontró que T13 no traía ninguna prueba de integración pese a que su propia "Verificación" exige `make test-integration`; Claude escribió `login_integration_test.go` (paquete externo `login_test` para evitar un ciclo de imports con `store`) y confirmó contra PostgreSQL real: el refresh token se guarda como SHA-256, `last_login_at` se actualiza y queda el audit `login_succeeded`. `python3 scripts/traceability.py --check`: matriz al día (RF-003 pasa de "parcial" a "completo", 10 pruebas).
- T12 · GREEN (Codex): `TestRF012_PlantillaDeRegistroIncluyeElEnlaceDeVerificacion`, `TestRF012_LosAvisosDeSeguridadNoIncluyenSecretos` y `TestRF012_PublicBaseURLTieneValorPorDefectoYAdmiteOverride` en verde; `make test-go`: PASS; `make lint`: solo los 3 hallazgos sembrados de `legacy_auth.go`. `gofmt -l` (Claude): limpio tras formatear. `make gen` repetido por Claude: sin diff adicional. El hook GGA (pre-commit) bloqueó el commit 3 veces seguidas antes de pasar a la cuarta: (1) `deliver` ignoraba `ctx` (regla "context.Context propagado"); (2) cero pruebas para `cmd/worker` (paquete sin `_test.go` desde la semana 1); (3) un bug real de idempotencia preexistente (ver "Ajuste de Claude" abajo) más una demanda de cobertura para `TRUSTED_PROXIES`/`ARGON2_CONCURRENCY` (de T6/T7, fuera de alcance de T12, rechazada por decisión del usuario). `python3 scripts/traceability.py --check`: matriz al día (RF-012 pasa de 6 a 8 pruebas).
- T11 · GREEN (Codex): `TestRF002_*` de servicio, API e integración en verde; `make test-go` y `make test-integration`: PASS; `make lint`: solo los 3 hallazgos sembrados de `legacy_auth.go`. `gofmt -l`: limpio (Claude). `make gen` repetido por Claude: sin diff adicional. Integración contra PostgreSQL 14 local (Claude): suite completa PASS, incluida `TestRF002_VerificarConsumeTokenYActivaCuenta` (verifica dos veces el mismo token: activa y luego 410). Claude añadió y corrió una prueba manual no commiteada (`zz_manual_expiry_check_test.go`, borrada después) para confirmar contra PostgreSQL real que un token ya vencido (`expires_at` en el pasado) se rechaza con `ErrTokenInvalid` y la cuenta queda en `pending_verification`: PASS. `python3 scripts/traceability.py --check`: matriz al día (RF-002 pasa de "parcial" a "completo", 9 pruebas; RNF-012 sube a 3).
- T10 · RED (Codex): faltaban los tipos de servicio/transacción de registro. GREEN (Codex): pruebas RF-001 de API y de servicio en verde; `make gen`: PASS; `make test-go` y `make test-integration`: PASS (revalidados por Claude); `make lint`: solo los 3 hallazgos sembrados de `legacy_auth.go`, sin hallazgos nuevos. `gofmt -l`: limpio. `make gen` repetido por Claude: sin diff adicional (determinista). Integración contra PostgreSQL 14 local (Claude): suite completa PASS, incluida `TestRF001_RegistroPersisteHashArgon2id`. Claude añadió y corrió una prueba manual no commiteada (`zz_manual_dup_check_test.go`, borrada después) para confirmar contra PostgreSQL real que un `UNIQUE` violado en `users.email` se traduce en `ErrEmailExists` a través de `errors.As`/`SQLState()` sobre el error envuelto: PASS. `python3 scripts/traceability.py --check`: matriz al día (RF-001 pasa de 6 a 15 pruebas). El hook GGA marcó como punto a revisar si AM-004 exige código idéntico a 201; se confirmó contra `specs/06-acceptance/registro-y-verificacion.feature:29-35` que el escenario Gherkin concreto exige 409 sin la palabra "existe" y ≤50 ms de diferencia (no un código idéntico), que es lo implementado.
- T9 · RED (Codex): `undefined: Record`, `Event`, `LoginFailed` (`FAIL .../internal/audit [build failed]`). GREEN unitario (Codex): `ok .../internal/audit 1.622s`. `gofmt -l` marcó `audit.go` y `audit_integration_test.go` (alineación de constantes y línea en blanco final); Claude corrigió con `gofmt -w`, sin cambios de comportamiento. Integración contra PostgreSQL 14 local (Claude, Codex no llega a la BD): primera corrida de `TestRF011_IdentityAppNoTieneUpdateNiDeleteSobreAuditLog` FAIL (el superusuario pasaba sin error porque los triggers seguían deshabilitados hasta el `t.Cleanup` final); Claude reordenó la prueba para reactivarlos antes de la comprobación del superusuario y quedó en verde. `make test-integration` completo: PASS. `down 1` con `migrate/migrate` confirma que `identity_app` desaparece de `pg_roles`; `up` lo reaplica sin diff. `python3 scripts/traceability.py --check`: matriz al día (RF-011 pasa de 1 a 3 pruebas). `make test-go` (`-race -short`): PASS, cobertura total 30.0%. `golangci-lint` sigue sin instalar (solo avisa, como documenta el Makefile).
- T1a · `python3 -m openapi_spec_validator specs/03-api/openapi.yaml`: OK; `python3 scripts/traceability.py --check`: matriz al día.
- T0.5 · `ls security/findings/VULN-*.md | wc -l` = 26 (una por id, VULN-001 a VULN-026); AM-NNN citadas comprobadas contra `specs/05-security/threat-model.md`; severidades comprobadas contra `gosec.json` y `trivy-config.json`; sin patrones de secreto; `git diff --stat` vacío para compose, Dockerfiles, `backend/` y `frontend/`.
- T14a · `git diff --stat`: solo `specs/adr/0005-refresh-tokens-rotativos-con-familia.md` (+17 −6); la ventana de gracia solo aparece como decisión descartada y alternativa rechazada; `python3 scripts/traceability.py --check`: matriz al día.
- T8 · `go build`, `go vet` (con y sin `integration`) y `go test -race -short ./...` en verde; `TestRF004_*` (token, JWKS, algoritmo alterado, sin sujeto o firmado por otra clave) y `TestRNF003_*` (falta la clave, no se revela) PASS; `git diff backend/go.mod`: solo `jwt/v5 v5.3.1`; `gen.go` y `legacy_auth.go` sin diff; trazabilidad regenerada y al día.
- T7 · `go build`, `go vet` (con y sin `integration`) y `go test -race -short ./...` en verde; `go test -race ./internal/auth/password/...` PASS (unas 20 s por los hashes de 64 MiB); integración contra PostgreSQL 14 local, dos veces: `TestRF001_UsersAceptaArgon2idYRechazaMD5` PASS y sin bases sobrantes; `git diff backend/go.mod`: solo `x/crypto` de indirecta a directa.
- T5 · `make gen` dos veces: mismo checksum de todo lo generado (`gen.go`, `schema.d.ts`, matriz y código de sqlc); `backend/go.mod` sin diff; `go build`, `go vet` (con y sin `integration`) y `go test -race -short ./...` en verde; integración contra PostgreSQL 14 local, dos veces: `TestRNF011_ConsultasBaseSqlcUsanParametros` y `TestRNF011_MigracionesSeAplicanSobreBaseVacia` PASS y sin bases sobrantes.
- T4 · `make test-integration` y `go test -race -tags=integration ./internal/testdb/...` dos veces contra PostgreSQL 14 local: PASS y sin bases temporales sobrantes; sin `TEST_DATABASE_URL`: SKIP; `go vet` con y sin la etiqueta, `go test -race -short ./...`, trazabilidad y YAML: en verde; Gitleaks sobre `ci.yml` y `Makefile`: 0 hallazgos.
- T2 · `make gen` dos veces: mismo checksum de `gen.go`, `schema.d.ts` y la matriz; `npm run lint`, `typecheck` y `test` (2/2) en verde; `go build`, `go vet` y `go test -race -short ./...` en verde; sonda negativa del gate: `git diff --exit-code` = 1.
- T1 · `cd backend && go build ./... && go vet ./... && go test -race -short ./...`: OK; `go generate ./internal/api` + `git diff --exit-code backend/internal/api/gen.go`: sin diff; `go mod verify`: all modules verified; `python3 scripts/traceability.py --check`: matriz al día (RNF-011 pasa a parcial).
- T0.4 · `git diff --stat`: 8 archivos de `security/findings/` (4 líneas cada uno) y `docs/evidencia/README.md` nuevo; `git diff --check`: sin errores; `python3 scripts/traceability.py --check`: matriz al día.

## Siguiente paso

1. T0.1 está hecha. Quedan T0.2 (escaneo semanal, corregir `baseline-scan.yml`, repetir imágenes, D3/D4,
   huellas para T31) y T0.3 reestructurada (Q16): el usuario toma las capturas "antes" con Claude Desktop
   (primera pasada con el prompt "antes"), y Claude escribe los `evidencia.json` con los datos que entregue.
2. `Codex` ya hizo **T1a**. Siguiente en orden para Codex: **T0.4** (solo docs), ahora con la convención de
   dos capas de Q16; T0.5 tras T0.2; luego T1 y la Fase 1. Las tareas de Fase 2 no `Remedia:` siguen el
   orden del archivo.
3. Todas las tareas `Remedia:` (Fase 3) siguen **bloqueadas por T0.3**.

## Cambios de spec propuestos

Formato: `Cn · archivo · qué cambia · por qué · tarea que lo motiva · estado`.

- **C1 · `specs/03-api/openapi.yaml` · Retirar `refreshToken` de `TokenPair` y `RefreshRequest` (cuerpos JSON); el refresh token viaja en cookie `HttpOnly; Secure; SameSite=Strict`, con `Set-Cookie` en login/refresh y borrado en logout, y `refresh`/`logout` exigen la cookie · RNF-009 y AM-015 piden cookie `HttpOnly` y el contrato usaba cuerpo JSON · T1a · ACEPTADO (autorización explícita del usuario, 2026-09-19, respuesta a Q3; Codex puede editar ese archivo solo en T1a).**
- **C2 · `specs/adr/0005-refresh-tokens-rotativos-con-familia.md` · Retirar la ventana de gracia de 10 s y documentar el riesgo residual (dos pestañas renovando a la vez pueden causar un falso positivo y cerrar la sesión) y la mitigación en el cliente (refrescador único compartido) · la ventana contradice el escenario de reuso inmediato y exigiría guardar el token en claro · T14a · ACEPTADO (autorización explícita del usuario, 2026-09-19, respuesta a Q4; Codex puede editar esa ADR solo en T14a).**
- **C3 · `specs/06-acceptance/*.feature` y RF-003 (`specs/01-requirements.md`) · Los escenarios dicen que "la respuesta contiene un `refreshToken`" y hablan del "refreshToken" del cuerpo; con C1 pasa a ser la cookie `refresh_token` · las pruebas Go de T13 a T15 verifican la cookie · T13, T14, T15 · PROPUESTO, opcional (no bloquea; los `.feature` no se editan en esta feature y se leen con ese significado; el usuario decide si se reformulan).**

## Decisiones sobre las preguntas abiertas

Todas resueltas por el usuario el 2026-09-19 (las que no traen cambio se aceptaron tal como estaban propuestas). No queda ninguna abierta; las dudas nuevas de Codex van en "Preguntas nuevas".

- **Q1 · Clave del bloqueo (T20).** Decisión: bloqueo por cuenta (RF-017: 5 fallos en 15 min, bloqueo de 15 min, 423, la contraseña correcta también falla) más un límite por IP con el mismo mecanismo, con la IP de la lógica de IP confiable de T6 (corrige VULN-020); umbral por IP como constante configurable, propuesta 20 fallos/15 min (ajustable), fecha 2026-09-19, afecta a: T20, T6.
- **Q2 · Clave JWT y `aud` (T8).** Decisión: variable obligatoria con la semilla Ed25519 de 32 B en base64; `kid` = huella (SHA-256 truncado) de la clave pública; `aud` configurable con valor por defecto documentado, fecha 2026-09-19, afecta a: T8.
- **Q3 · Transporte del refresh token.** Decisión: cookie `HttpOnly; Secure; SameSite=Strict` (RNF-009, AM-015); el `accessToken` sigue en el cuerpo; el OpenAPI se enmienda en la nueva T1a (C1, autorizada por el usuario), fecha 2026-09-19, afecta a: T1a, T1, T13, T14, T15, T21, T29.
- **Q4 · Ventana de gracia de 10 s (ADR 0005).** Decisión: sin ventana de gracia; el riesgo residual (falso positivo con dos pestañas concurrentes) se documenta y se mitiga en el cliente con un refrescador único compartido; la ADR se enmienda en la nueva T14a (C2, autorizada por el usuario), fecha 2026-09-19, afecta a: T14a, T14.
- **Q5 · Primer administrador.** Decisión: alta manual documentada por SQL para desarrollo; sin endpoint ni valor sembrado en el repo, fecha 2026-09-19, afecta a: T17, T21.
- **Q6 · Invariante 6 "siempre un admin habilitado".** Decisión: comprobación en el servicio dentro de una transacción con bloqueo de fila y, si es viable, un trigger en una migración nueva, fecha 2026-09-19, afecta a: T18.
- **Q7 · Rol `identity_app`.** Decisión: T9 crea el rol y los permisos; T26 le asigna la credencial desde `.env` y cambia el `DATABASE_URL` de la API, fecha 2026-09-19, afecta a: T9, T26.
- **Q8 · Gitleaks e historial.** Decisión: opción (a), `.gitleaksignore` por huella exacta limitado a los 12 hallazgos conocidos de la línea base, cada entrada con su VULN y justificación; opciones (b) y (c) descartadas; Codex lo implementa cuando el usuario entregue la lista de huellas del run de T0.2, fecha 2026-09-19, afecta a: T0.2, T31.
- **Q9 · Ids.** Decisión: `VULN-020` se queda como el hallazgo de chi `RealIP`; ids nuevos desde VULN-024: VULN-024 = endurecimiento de contenedores del compose (su comentario hoy dice VULN-020), VULN-025 = axios/lodash, VULN-026 = `golang.org/x/text` GO-2026-5970; los ids únicos ya usados en comentarios (003, 004, 006 a 019) se conservan y T0.5 crea sus fichas; un id solo se asigna al crear su ficha, nunca se inventa en un comentario; el comentario del compose NO se renumera antes del push de la línea base (se documenta la equivalencia en T0.5 y se corrige en T30), fecha 2026-09-19, afecta a: T0.5, T24, T25, T30, T36, "Registro de evidencia".
- **Q10 · Visibilidad del repo.** Decisión: público, `git@github.com:jorgepaez-ops/IdentityHub.git`; code scanning y subida de SARIF disponibles; comprobación previa al push de que las credenciales sembradas no son reales ni reutilizadas, fecha 2026-09-19, afecta a: T0.1, T0.2, "Protocolo de evidencia".
- **Q11 · Directiva `go` y `GO_VERSION`.** Decisión: parar y preguntar antes de subirla si un aviso lo exige; no subirla sin aprobación, fecha 2026-09-19, afecta a: T1, T24.
- **Q12 · Puertos en desarrollo.** Decisión: mantener `db` y `broker` publicados en desarrollo y tratarlo en `docker-compose.prod.yml` (semana 3), fecha 2026-09-19, afecta a: T30.
- **Q13 · CIDR de `TRUSTED_PROXIES`.** Decisión: fijar una subred en la red por defecto del compose y usarla como valor, fecha 2026-09-19, afecta a: T21, T6.
- **Q14 · Alcance.** Decisión: RF-013, RF-014, RF-015, RF-016, RF-018 y RF-019 fuera de esta feature y al backlog de semana 3; se muestran como "diferido" (no olvidados) en la matriz de trazabilidad y T13 rechaza con un error claro las cuentas con MFA activado, fecha 2026-09-19, afecta a: T13, T34.
- **Q15 · Página del enlace de verificación.** Decisión: la plantilla emite `/verify-email?token=...`; en esta feature se verifica con `curl`; la página del SPA se hace en semana 3, fecha 2026-09-19, afecta a: T12, T21.

- **Q16 · Dónde vive la evidencia (2026-09-20).** Decisión: Claude Desktop no escribe en el repo (errores de permisos) y genera por su cuenta el informe `.docx` con capturas reales tomadas navegando GitHub. Por eso las capturas dejan de ser `before.png`/`after.png` versionados: en el repo queda solo `docs/evidencia/VULN-XXX/evidencia.json` (texto, lo escribe Claude a partir de los datos que entrega Desktop) y el informe externo lleva las imágenes; `captura` referencia su sección. T0.3, T0.4, T35, el criterio 6, el protocolo y `AGENTS.md` se ajustan; las tareas de código no cambian. Costo aceptado: las imágenes no quedan en el repo público; lo que perdura cuando caducan los logs es el JSON y `security/evidence/`. Fecha 2026-09-20, afecta a: T0.3, T0.4, T35, T36, criterio 6.

- **Q19 · Subida a Go 1.25 (2026-09-21).** Decisión del usuario (amplía Q11): la directiva `go` de `backend/go.mod` y `GO_VERSION` del CI y del escaneo semanal pasan de 1.22 a **1.25**, de una sola vez. Motivos: chi >= v5.3.0 (corrige GO-2026-5775 y 5777) exige `go 1.23`; los 26 avisos de la biblioteca estándar se corrigen entre Go 1.23.8 y 1.25.13; el job 5 del CI (`govulncheck`) solo puede ponerse en verde con un Go >= 1.25.13; y lo más probable es que T24 también lo pida. `baseline-scan.yml` se queda en 1.22 porque analiza el tag de la línea base. Efecto colateral: `golangci-lint` v1 (compilado con Go 1.22) no analiza código que apunte a Go 1.25, así que el CI usa v2.13.2 con la configuración migrada. Afecta a: T6, T24, T27, CI.
- **Q17 · Dependabot (2026-09-20).** Decisión: no se activa Dependabot por ahora (ni alertas ni actualizaciones). La evidencia de dependencias (VULN-020, 021, 022, 025, 026) sale de govulncheck y npm audit; el escaneo semanal cubre la revisión continua. CodeQL no cubre dependencias, solo código. Se puede reconsiderar `dependabot.yml` tras remediar (semana 3). Afecta a: T0.3.

- **Q18 · Evidencia de VULN-013, 014 y 015 (2026-09-20).** Decisión: se acepta la salida en texto de `security/evidence/local-web-baseline.txt` (curl -sI y ráfaga sobre la imagen `web` del tag, con fecha y método) en lugar de una captura de pantalla. La misma regla se usa para el "después" (T35): la salida del mismo comando. Afecta a: T0.3, T35.

### Preguntas nuevas (Codex)

- **Q21 · Credencial de `identity_app` para T26 (2026-09-26).** (Codex la etiquetó "Q20" por error — ese número ya está usado, ver arriba; renumerada por Claude.) T9 creó `identity_app NOLOGIN` y su migración no recibe ni provisiona una contraseña. T26 exige que `DATABASE_URL` use ese rol con una credencial desde `.env`, pero los archivos autorizados para T26 no incluyen una migración nueva ni un mecanismo de inicialización seguro para asignarla; usar `POSTGRES_USER=identity_app` lo convertiría en el superusuario inicial y rompería el mínimo privilegio. Se requiere que el usuario autorice el mecanismo y el alcance para provisionar el rol con `LOGIN` y una contraseña desde `.env`; T26 queda detenida.


## Notas de handoff Codex

Formato por tarea (3 a 5 líneas):

```
### T<id> · <fecha> · <SHA corto>
- Qué cambió: ...
- Comandos y resultado observado (incluye el RED): ...
- Dudas abiertas: ...
```

(Sin entradas todavía.)

### T1a · 2026-09-19 · d9117cb
- Qué cambió: `TokenPair` ya no publica `refreshToken`; refresh/logout usan la cookie `refresh_token`, con `Set-Cookie` documentado en login, MFA, refresh y logout.
- Comandos y resultado observado: RED: la aserción de contrato falló porque `RefreshRequest` seguía definido; GREEN: la aserción pasó y confirmó 22 pares `operationId`/`x-requirement` intactos; el validador OpenAPI devolvió OK y trazabilidad quedó al día.
- Dudas abiertas: ninguna. Se corrigieron tres descripciones YAML preexistentes con comas sin comillas para que el contrato pasara la validación obligatoria sin cambiar su texto.

### T0.4 · 2026-09-20 · 54c3525
- Qué cambió: la plantilla de `security/findings/README.md` y las 7 fichas existentes (VULN-001, 002, 005, 020, 021, 022, 023) llevan las filas vacías Evidencia antes, Commit de remediación, Evidencia después y Run de Actions; `docs/evidencia/README.md` documenta la convención de dos capas (Q16).
- Comandos y resultado observado: solo docs, sin RED/GREEN. `git diff --check`: sin errores; `python3 scripts/traceability.py --check`: matriz al día; `git diff --stat`: solo los 8 archivos de fichas y el README nuevo, sin datos inventados ni carpetas `VULN-*` ni `evidencia.json`.
- Dudas abiertas: ninguna. Codex dejó los cambios sin commitear porque su sandbox no puede crear `.git/index.lock`; Claude revisó el diff y creó ambos commits. Para las próximas tareas con commits hay que dar a Codex escritura en `.git` o seguir cerrando desde Claude.

### T1 · 2026-09-20 · df47361
- Qué cambió: `gen.go` generado con oapi-codegen **v2.5.1** y `github.com/oapi-codegen/runtime` **v1.1.2** (configuración en `oapi-codegen.yaml`, regenerable con `go generate ./internal/api`); `Server` implementa las 22 operaciones y las pendientes responden 501 `application/problem+json`; `/healthz` y `/readyz` salen ahora del router generado; `legacy_auth.go` y `/api/v1/auth/legacy-login` intactos.
- Comandos y resultado observado: RED: `undefined: ServerInterface` y `undefined: TokenPair` (Codex, antes de generar). GREEN: `TestRNF011_GeneratedServerContract`; `go build`, `go vet` y `go test -race -short ./...` en verde (Claude los repitió). Regeneración byte a byte idéntica. Comprobación temporal de rutas (no commiteada): legacy-login, healthz, readyz, metrics, register y jwks registradas (32 rutas).
- Dependencias: `go.mod` conserva `go 1.22`, chi 5.0.11, jwt v4, pgx 5.5.1 y x/text 0.14.0. Cambios: `+runtime v1.1.2`, `+go-jsonmerge/v2 v2.0.0 // indirect` y `x/crypto` indirecto de 0.9.0 a 0.17.0 (no lo cubre ningún VULN). No se usa oapi-codegen v2.8.0: exige runtime >= 1.3, que sube la directiva a `go 1.24` y actualiza x/text (rompe Q11 y borra la evidencia de VULN-026).
- Dudas abiertas: ninguna. `go mod tidy -diff` quitaría `testify` indirecto, que ya estaba en la línea base: se deja. `golangci-lint` no está instalado localmente. Codex no pudo commitear (sandbox sin escritura en `.git`); Claude hizo ambos commits.

### T2 · 2026-09-20 · f3d8a0e
- Qué cambió: `make gen` regenera `gen.go` (oapi-codegen v2.5.1), `frontend/src/api/schema.d.ts` (openapi-typescript 6.7.6) y la matriz de trazabilidad; el job `spec-drift` regenera lo mismo y falla con cualquier diff o archivo sin seguimiento (`git diff --exit-code` y `git status --porcelain`). `schema.d.ts` generado (786 líneas, sin `refreshToken`).
- Comandos y resultado observado: RED: `make gen` solo corría la trazabilidad y `schema.d.ts` no existía. GREEN: `make gen` de punta a punta y sin diff en la segunda ejecución; frontend lint, typecheck y tests en verde; backend build, vet y tests con `-race` en verde. Claude repitió todo y añadió la sonda negativa.
- Dudas abiertas: ninguna. Ajuste de Claude: sin `cache: npm` en el `setup-node` de `spec-drift` (falta el lockfile, ver T3). Node 18 en CI (`NODE_VERSION`), Node 24 en local. Codex no pudo restaurar con `git checkout` (sandbox sin `.git`) y restauró desde una copia; sin restos.

### T4 · 2026-09-20 · 6954cba
- Qué cambió: paquete `backend/internal/testdb` (etiqueta `integration`): `testdb.New(t)` crea una base temporal con nombre aleatorio (`crypto/rand`) a partir de `TEST_DATABASE_URL`, aplica las migraciones `*.up.sql` con pgx y la borra al terminar; se omite si falta la variable. `make test-integration`. Job `6b · Pruebas de integración` en `ci.yml` con PostgreSQL **16.15** fijado por digest (`postgres:16-bookworm@sha256:efedf359…`), contraseña de servicio efímera derivada de `github.run_id`. Prueba `TestRNF011_MigracionesSeAplicanSobreBaseVacia` (tablas esperadas, un MD5 viola `users_password_hash_is_argon2id` con código 23514, un hash `$argon2id$` entra).
- Comandos y resultado observado: RED (Codex): faltaba el paquete auxiliar. GREEN (Claude, con la base real, dos veces): PASS y `pg_database` sin bases temporales. Sin variable: SKIP. Todo lo demás en verde. El sandbox de Codex no llega a `localhost:5432` (`operation not permitted`): no dio GREEN de integración y lo dejó dicho; Claude lo ejecutó.
- Ajuste de Claude: el pool que devuelve `testdb.New` usaba el protocolo simple de pgx en todas las consultas; ahora solo las migraciones (varias sentencias por archivo) lo usan, y el resto queda en el protocolo extendido, igual que producción y que el código de sqlc de T5.
- Dudas abiertas: ninguna. El job del CI arma la URL con `format()` para que Gitleaks no marque un literal de URL de conexión con usuario y clave; no se amplió ninguna lista de excepciones. Codex ejecutó `gentle-ai codegraph init` y creó `.codegraph/` (sin versionar, fuera de la tarea): no se commitea.

### T5 · 2026-09-20 · d387fed
- Qué cambió: `sqlc.yaml` (sqlc **v1.31.1**, solo las migraciones `*.up.sql`, pgx/v5; `citext` a `string`, `inet` a `netip.Addr`, `uuid` a `google/uuid`); consultas parametrizadas `CreateUser`, `GetUserByEmail`, `GetUserByID` e `InsertAuditEvent` en `db/queries/`; código generado en `backend/internal/store/internal/sqlc` (paquete `internal`: las capas superiores no pueden importarlo); adaptador manual en `store.go` con tipos propios (`User`, `AuditEvent`, `CreateUserParams`, `InsertAuditEventParams`), `NewWithPool` y errores envueltos con `%w`; `make gen` ejecuta `sqlc generate` y comprueba la versión; el job `spec-drift` instala sqlc con `GOTOOLCHAIN=go1.26.2` (su `go.mod` exige Go 1.26) y regenera.
- Comandos y resultado observado: RED (Codex): el adaptador no existía. GREEN (Claude, con la base real, dos veces): `TestRNF011_ConsultasBaseSqlcUsanParametros` PASS (correo con distinta capitalización por `citext`, `' OR '1'='1' --` como valor literal devuelve `pgx.ErrNoRows`, ida y vuelta de `InsertAuditEvent`); `make gen` idempotente; `go.mod` sin diff. El sandbox de Codex no llega a PostgreSQL: solo compiló las pruebas de integración.
- Ajuste de Claude: el hook de revisión previo al commit rechazó el primer intento porque el adaptador devolvía los errores de sqlc sin contexto y con la estructura a medio llenar; ahora envuelve con `%w` y devuelve el valor cero. `errors.Is(err, pgx.ErrNoRows)` sigue funcionando (lo comprueba la prueba).
- Dudas abiertas: ninguna. `backend/internal/store/store.go` (archivo de la línea base) recibió el adaptador y sus tipos: las llamadas a `pgxpool.NewWithConfig` y `Ping` que citan las trazas de VULN-022 pasaron de las líneas 36 y 57 a la 85 y la 92; el "antes" quedó registrado con las líneas originales en el run del tag. sqlc v1.28.0 no compila en este macOS por cgo, por eso se fijó v1.31.1. T5 tardó unos 28 minutos (Codex iteró sobre `sqlc.yaml` y el adaptador).

### T7 · 2026-09-20 · 4e8c653
- Qué cambió: paquete `backend/internal/auth/password` con `Hash`, `Verify` (comparación en tiempo constante con `subtle`), `NeedsRehash`, `VerifyDecoy` (hash señuelo generado una vez) y un semáforo que acota la concurrencia; formato PHC `$argon2id$v=19$m=65536,t=3,p=2$…`; contraseña vacía o de más de 128 caracteres rechazada con error tipado (`InvalidPasswordError`). Configuración nueva: `ARGON2_MEMORY_KIB` (65536), `ARGON2_ITERATIONS` (3), `ARGON2_PARALLELISM` (2) y `ARGON2_CONCURRENCY` (4), validadas en `config.Load` con su patrón de acumulación de errores. `golang.org/x/crypto` sigue en **v0.17.0** y solo pasa de indirecta a directa.
- Comandos y resultado observado: RED (Codex): `undefined: Configure`, `undefined: config.PasswordConfig`, `undefined: Hash`, `undefined: NeedsRehash` y `FAIL [build failed]`. GREEN: pruebas unitarias en verde; integración con la base real (Claude), dos veces: PASS.
- Concurrencia por defecto **4**: cada verificación usa 64 MiB, así que acota Argon2id a unos 256 MiB simultáneos sin serializar todos los inicios de sesión. No hace falta migración: no hay usuarios previos ni filas MD5.
- Ajuste de Claude: la prueba de integración de Codex no podía pasar (el sandbox no llega a la base y no la ejecutó): sus `INSERT` omitían `display_name` (`NOT NULL`), y el rechazo del MD5 habría fallado por esa columna (23502) antes de llegar a la restricción (23514). Se añadió `display_name`, se usa `errors.As` y se comprueba también el nombre `users_password_hash_is_argon2id`.
- Dudas abiertas / pendientes de otras tareas (observaciones del hook de revisión, no bloqueantes): (1) `Hash` y `Verify` bloquean en el semáforo sin `context.Context`: una petición cancelada seguirá en cola; se plantea al integrarlos en el inicio de sesión (T13). (2) El hash señuelo usa los parámetros de la primera llamada: `Configure` debe ejecutarse antes de la primera petición. (3) Posibles avisos gosec G115 por las conversiones `uint32`/`uint8` de la configuración: los límites se validan antes, pero el linter del CI (golangci-lint v1.59) puede no verlo; `golangci-lint` no está instalado en local, se revisará en el primer run de CI. `x/crypto` no se sube: subirlo actualiza de rebote `x/text` (0.14.0 a 0.21.0), así que queda para T24.

### T8 · 2026-09-20 · 5da3fcf
- Qué cambió: paquete `backend/internal/auth/token` (`golang-jwt/jwt/v5` **v5.3.1**, junto a v4 que sigue hasta T23): tokens EdDSA de 15 minutos con `iss`, `sub`, `aud`, `exp`, `iat`, `jti`, `roles` y `kid` (primeros 16 caracteres hexadecimales del SHA-256 de la clave pública); validación solo EdDSA, con `kid`, emisor, audiencia, sujeto y caducidad obligatorios y reloj inyectable. `GET /.well-known/jwks.json` (`jwks.go`, OKP Ed25519, `x` en base64url) y middleware `RequireAuth` (`auth_middleware.go`, 401 `problem+json` sin decir el motivo). Configuración: `JWT_SIGNING_KEY` obligatoria (semilla Ed25519 de 32 bytes en base64, `config.Secret`, sin valor por defecto: sin ella la API no arranca, RNF-003), `JWT_ISSUER` (`http://localhost:8080`) y `JWT_AUDIENCE` (`identity-hub`). `Server.SetTokenService` inyecta el servicio: la firma de `NewServer` no cambia; sin servicio, JWKS responde 503 y `RequireAuth` 401.
- Comandos y resultado observado: RED (Codex): faltaban `token.New` y la validación de `JWT_SIGNING_KEY`. GREEN: suite completa y pruebas de T8 en verde (Claude las repitió).
- Ajustes de Claude: (1) la prueba de confusión de algoritmo de Codex firmaba los tokens falsos **sin `kid`**, así que se habrían rechazado por "clave desconocida" aunque no existiera la restricción de algoritmo; ahora llevan el `kid` y todas las claims correctas (solo cambia el algoritmo) y hay un control positivo con un token EdDSA válido. (2) `Validate` exigía todo menos el sujeto: ahora rechaza un token sin `sub`. (3) Nueva prueba `TestRF004_RechazaTokenSinSujetoOFirmadoPorOtraClave` (sin sujeto, y firmado por otra clave con el `kid` correcto). (4) Se regeneró `specs/07-traceability.md`, que Codex dejó desactualizada. Nota: en `jwt/v5` los tokens `none` y HS256 con la clave pública también fallarían por tipo de clave; la restricción de algoritmo es defensa en profundidad y la prueba fija el resultado observable.
- Dudas abiertas / para T13 y T21: `config.Config.AccessTTL` (`JWT_ACCESS_TTL`, ya estaba en la línea base) no se usa: el servicio fija los 15 minutos de RF-004; al cablear en T21 conviene retirar esa variable o validarla a 15 minutos. `RequireAuth` reconoce el esquema `Bearer` con esa capitalización exacta. T21 debe decodificar `JWT_SIGNING_KEY`, crear el servicio y llamar a `SetTokenService`. Clave de desarrollo: `openssl rand -base64 32` en un `.env` git-ignorado; jamás en el repositorio.

### T14a · 2026-09-20 · 67de318
- Qué cambió: la ADR 0005 (enmienda C2, decisión Q4) pasa a decir que **no hay ventana de gracia**: contradice el escenario de reuso inmediato y devolver el mismo par exigiría guardar el token en claro (la ADR solo guarda su SHA-256); se acepta el riesgo residual (dos pestañas que renuevan a la vez pueden provocar un falso positivo y cerrar la sesión) y la mitigación queda para el frontend de la semana 3 (un único refrescador compartido entre pestañas, con Web Locks API o `BroadcastChannel`). La línea de estado registra "Enmienda C2 · 2026-09-19".
- Comandos y resultado observado: solo documentación, sin RED/GREEN. Claude revisó el diff línea por línea: solo cambia esa ADR y el resto del texto no se toca.
- Dudas abiertas: ninguna.

### T0.5 · 2026-09-21 · 4b2d4f4
- Qué cambió: 19 fichas nuevas en `security/findings/` (VULN-003, 004, 006 a 019, 024, 025 y 026), con las 7 existentes suman las 26. Cada una lleva la severidad del propio escáner (o "no informada por ningún escáner" donde ningún gate lo detecta: 006, 007, 011 y 024), los gates de su `evidencia.json`, `AM-NNN` reales del modelo de amenazas, la ruta de la evidencia y la URL del run "antes". VULN-024 documenta que el comentario del compose la llama VULN-020 (id de chi) y que se corrige en T30. No se toca compose, Dockerfiles, `backend/` ni `frontend/`.
- Comandos y resultado observado: solo documentación, sin RED/GREEN. Claude revisó las 19 con un script (AM existentes, ruta y URL coherentes con el JSON, estado `abierto`, campos de remediación vacíos, sin patrones de secreto) y comprobó las severidades contra `gosec.json` (G404 HIGH, G101 HIGH) y `trivy-config.json` (DS002 y DS029 HIGH, DS031 CRITICAL).
- Ajuste de Claude: VULN-024 arrastraba una frase interna del proceso ("Desktop los había asignado a este VULN por error"); se reescribió con lo que hace `trivy config` (solo analiza Dockerfiles).
- Dudas abiertas: los hallazgos sin id (CodeQL #81, `amqp091-go`, alertas de Semgrep de las acciones y de `default.conf`, alertas de los diagramas, avisos de la biblioteca estándar) siguen sin ficha: un id solo se asigna al crear su ficha y la decisión de abrirlas queda pendiente (ver la bitácora).

### T6 · 2026-09-21 · 902a047
- Qué cambió: se retira `middleware.RealIP` y se añade `ClientIP` (`backend/internal/api/clientip.go`): la IP del cliente es el par del socket, y solo si ese par está en `TRUSTED_PROXIES` se lee `X-Forwarded-For` y se toma la dirección **más a la derecha que no sea de confianza**; una cabecera ausente, mal formada o de origen no confiable se ignora entera. `ClientIPFrom(ctx)` la expone para auditoría y bloqueo. `TRUSTED_PROXIES` (CIDR separados por comas, opcional, vacío = ninguno) se valida al arrancar y acumula errores como el resto de `config.Load`. `Server.SetTrustedProxies` la inyecta (la firma de `NewServer` no cambia; T21 la cablea en `main.go`). chi **v5.0.11 a v5.3.2** y directiva `go` **1.22 a 1.25** (Q19).
- Comandos y resultado observado: RED (Codex): `undefined: ClientIP` y `undefined: ClientIPFrom`. GREEN: `go build`, `go vet` (con y sin `integration`), `go test -race -short ./...` y la integración completa contra PostgreSQL local, todo en verde (Claude las repitió). `git diff backend/go.mod`: solo la frase del comentario, la directiva y chi. **`govulncheck` (Claude, con Go 1.27.1): GO-2026-5775 y GO-2026-5777 ya no aparecen y los 26 avisos de la biblioteca estándar tampoco**; quedan 7: `jwt/v4` x2 (T23), `pgx` x3 y `x/text` (T24) y `amqp091-go` GO-2026-6372 (sin ficha). Aparecen dos de `pgx` que el "antes" no listaba (GO-2026-5004, corregido en v5.9.2, y GO-2024-2567, en v5.5.2): T24 debe subir `pgx` a la última estable.
- Ajustes de Claude: (1) el CI y la configuración del linter se migraron a Go 1.25 y golangci-lint v2 (commit `fd24b09`); (2) `make lint` escondía los fallos con un `|| echo` y ahora solo avisa si falta el linter; (3) limpieza de los avisos propios del linter v2 sin `//nolint` (commit `a75cc36`): aserción de tipo comprobada, `%w` en ambos errores, conversiones de enteros acotadas (G115) y cuatro comentarios reformulados. Con el linter v2 sobre el árbol solo quedan 3 avisos, todos de la línea base sembrada (`legacy_auth.go`, hasta T23): G101 x2 y `nolintlint`.
- Dudas abiertas: si todos los saltos de `X-Forwarded-For` son de confianza se devuelve la IP del par (seguro, pero todos los clientes detrás de ese proxy compartirían IP y límite); `Routes()` lee `trustedProxies` al construir, así que `SetTrustedProxies` debe llamarse antes (T21). Observaciones del hook sin ficha: `Readiness` devuelve `err.Error()` de cada dependencia en el cuerpo de `/readyz`, lo que puede exponer host, usuario o base de datos; y `tipo[:9]` en `events_test.go` puede entrar en pánico con tipos cortos. Para T35: `VULN-020` necesita su captura "después" tras un CI verde.

### T9 · 2026-09-24 · bacd932
- Qué cambió: `audit.Record` (`backend/internal/audit/audit.go`) escribe eventos con IP de `api.ClientIPFrom(ctx)`, user-agent y metadata jsonb, rechazando cualquier metadata cuya clave contenga `password`, `token`, `code` o `secret` (RNF-012). Migración `000002_identity_app_audit_permissions`: crea el rol `identity_app` (idempotente, sin contraseña) con el mínimo privilegio por tabla y `SELECT, INSERT` en `audit_log`, y `REVOKE UPDATE, DELETE` sobre esa tabla como defensa en profundidad encima del trigger de append-only de 000001 (no se toca el trigger).
- Comandos y resultado observado: ver la entrada de T9 en "Evidencia de verificación" arriba (RED/GREEN, el bug de orden en la prueba de integración y su corrección, `test-integration`, `down`/`up` y trazabilidad).
- Ajuste de Claude: reordenó `audit_integration_test.go` para reactivar los triggers `audit_log_no_update`/`audit_log_no_delete` inmediatamente después de la comprobación de permisos de `identity_app`, en vez de dejarlo solo en `t.Cleanup` (que corre al final del test, después de la aserción que necesita el trigger ya activo). Aplicó `gofmt -w` a los dos archivos que Codex dejó sin formatear.
- Dudas abiertas: ninguna nueva. Codex no pudo commitear (`Unable to create '.git/index.lock': Operation not permitted`, sin proceso git en curso al revisar); Claude verificó el árbol y creó ambos commits, igual que en T0.4, T1, T2 y T4.

### T10 · 2026-09-25 · 43d2464
- Qué cambió: `backend/internal/auth/registration` (nuevo): `Service.Register` valida entrada, calcula el hash Argon2id **antes** de comprobar existencia del correo (mismo costo en ambas rutas, AM-004), genera un token de verificación de 32 B `crypto/rand` (SHA-256, 24 h), registra auditoría `user_registered` y publica `user.registered` con publisher confirms — todo dentro de una única transacción (`store.WithinRegistrationTransaction`, nuevo en `store.go`) que se revierte si el broker falla (ADR 0006, 503). `POST /api/v1/auth/register` (`backend/internal/api/register.go`) queda cableado: 201/400/409/503. Nueva consulta sqlc `CreateVerificationToken`.
- Comandos y resultado observado: ver la entrada de T10 en "Evidencia de verificación" arriba.
- Ajuste de Claude: ninguno al código; se limitó a verificar. Revisó los 8 archivos del diff línea por línea, confirmó que `writer.CreateUser` detecta la violación `UNIQUE` real de Postgres (código 23505) a través de `errors.As` sobre el error envuelto con `%w` — lo probó con una prueba manual temporal (no commiteada) que registró el mismo correo dos veces contra PostgreSQL real — y que el escenario Gherkin de AM-004 (409, sin la palabra "existe", ≤50 ms) es la fuente correcta, no la frase genérica de una sola línea del modelo de amenazas que el hook GGA citó como duda.
- Dudas abiertas: ninguna. Codex volvió a bloquearse en `.git/index.lock` (mismo runtime de solo lectura que T9); Claude creó ambos commits.

### T11 · 2026-09-25 · 4ad6038
- Qué cambió: `backend/internal/auth/verification` (nuevo): `Service.Verify` calcula el hash SHA-256 del token recibido y delega en una única sentencia SQL (CTE `ConsumeEmailVerificationToken`) que marca el token usado y activa la cuenta atómicamente, solo si el hash coincide, no fue usado y no venció; el bloqueo de fila de Postgres hace que dos intentos concurrentes con el mismo token no puedan activarse ambos. Token desconocido, vencido o ya usado comparten un único `410 Gone` (AM-016, elegido y documentado en el propio handler: "deliberately share this response to avoid revealing which token state was observed"). Registra auditoría `email_verified` y publica `user.email_verified` con confirms, dentro de la misma transacción (`store.WithinEmailVerificationTransaction`, nuevo). `POST /api/v1/auth/verify-email` cableado: 204/400/410/503.
- Comandos y resultado observado: ver la entrada de T11 en "Evidencia de verificación" arriba.
- Ajuste de Claude: ninguno al código. Revisó los 7 archivos entregados más los 2 que el hook GGA no pudo ver (`db/queries/users.sql`, `users.sql.go`) para confirmar que el `WHERE used_at IS NULL AND expires_at > now()` de la CTE es correcto y no deja condiciones de carrera (el `UPDATE` re-evalúa su `WHERE` tras adquirir el lock de fila). Probó el caso de expiración contra PostgreSQL real con una prueba manual temporal (no commiteada), ya que la única prueba de integración de Codex solo cubría uso repetido, no vencimiento.
- Dudas abiertas: ninguna. Mismo bloqueo de `.git/index.lock` que T9 y T10; Claude creó ambos commits.

### T12 · 2026-09-25 · f1547a0
- Qué cambió: `backend/internal/notify` (nuevo): `Render` es una función pura que arma asunto y cuerpo por `eventType` desde un struct allowlist (nunca desde el payload crudo), así que campos como `refreshToken`/`password` que vengan en el evento no llegan al correo aunque el emisor los incluya por error. `user.registered` incluye el enlace de verificación (`PUBLIC_BASE_URL`, nueva variable opcional en `config.go`, por defecto `http://localhost:8080`, ruta `/verify-email?token=...` según Q15); `security.refresh_reuse_detected` y `security.account_locked` llevan aviso de seguridad sin datos del evento. `backend/cmd/worker/main.go`: `deliver` delega en `notify.Render` en vez del payload genérico anterior.
- Comandos y resultado observado: ver la entrada de T12 en "Evidencia de verificación" arriba.
- Ajuste de Claude: (1) `deliver` ignoraba `ctx` (parámetro `_ context.Context`, ya así desde la semana 1); ahora lo propaga y aborta antes de arrancar un `smtp.SendMail` nuevo si ya empezó el apagado (`net/smtp` no admite contexto, así que uno en curso no se puede interrumpir). (2) Extrajo `buildRawMessage` y le agregó `TestRF012_ConstruyeElMensajeSMTPConAsuntoYCuerpoRenderizados` para que `cmd/worker` deje de tener cero pruebas. (3) Encontró y corrigió un bug real preexistente de la semana 1 (`053e15f`, ajeno a T12): `alreadyProcessed` marcaba el `eventID` como visto *antes* de intentar `deliver`, así que un solo fallo transitorio de SMTP hacía que el reintento (`Nack` con requeue) se descartara como "duplicado" sin haber enviado nunca el correo — lo confirmó con `TestRF012_FalloTransitorioReintentaSinPerderLaNotificacion` (RED con el código viejo, GREEN tras mover la marca a después de una entrega exitosa, vía el nuevo `markDelivered`).
- Dudas abiertas: el hook GGA además pidió cobertura para `TRUSTED_PROXIES` (T6) y `ARGON2_CONCURRENCY` (T7) en `config_test.go`, alegando que el archivo "es parte de este cambio"; es deuda de tareas ya cerradas, ajena a T12. El usuario decidió no ampliar T12 para cubrirlo (commitear con `--no-verify` si volvía a bloquear tras el fix del bug real; no hizo falta, el commit pasó al cuarto intento). Queda pendiente decidir si se abre una tarea aparte para esa cobertura. Codex no pudo commitear (mismo bloqueo de `.git/index.lock`); Claude creó ambos commits.

### T13 · 2026-09-25 · 8ab9409
- Qué cambió: `backend/internal/auth/login` (nuevo): `Service.Login` solo emite tokens para cuentas `active`; correo inexistente verifica contra `password.VerifyDecoy` (AM-004); MFA se comprueba **después** de validar la contraseña, así que una cuenta con `mfa_enabled` y contraseña correcta se rechaza con 501 `application/problem+json` sin tokens ni cookie, mientras que password incorrecta o cuenta no activa comparten un 401 genérico. Éxito: `NeedsRehash` recalcula el hash si hace falta, refresh token opaco de 32 B `crypto/rand` guardado como SHA-256 con `family_id` nuevo, `last_login_at` y auditoría `login_succeeded`/`login_failed` (con motivo). `backend/internal/api/login.go` cablea `POST /api/v1/auth/login`: cuerpo `TokenPair` sin `refreshToken`, cookie `Set-Cookie: refresh_token=...; HttpOnly; Secure; SameSite=Strict; Path=/api/v1/auth; Max-Age=<RefreshTTL>` (reutiliza `config.RefreshTTL`/`JWT_REFRESH_TTL` de T8, por defecto 720 h = 30 días).
- Comandos y resultado observado: ver la entrada de T13 en "Evidencia de verificación" arriba.
- Ajuste de Claude: (1) renombró las 2 pruebas `TestAM004_...` a `TestRF003_AM004...` porque el patrón que exige `traceability.py` es `Test(RF|RNF)NNN_...` y `AM004` no calza — sin el cambio, esas dos pruebas nunca se hubieran contado en la matriz. (2) Escribió `backend/internal/auth/login/login_integration_test.go` (ausente pese a que la propia tarea exige `make test-integration`) en el paquete externo `login_test` para no crear un ciclo de imports con `store` (que ya importa `login` para sus tipos `Writer`/`RefreshToken`); confirmó contra PostgreSQL real el hash SHA-256 del refresh token, `last_login_at` y el audit `login_succeeded`.
- Dudas abiertas: ninguna. Mismo bloqueo de `.git/index.lock`; Claude creó ambos commits.

### T17 · 2026-09-25 · 4e26918
- Qué cambió: `backend/internal/api/rbac.go` (nuevo): `RequireRole(repository, "admin")` sigue la misma forma encadenable que `RequireAuth` (`func(http.Handler) http.Handler`); en cada petición lee `GetUserByID` (401 si el estado no es `active`) y `ListRolesForUser` (403 si el rol pedido no está en la lista), ambos ya existentes en `Store` desde T16, sin capa de servicio nueva ni consulta SQL nueva. `backend/internal/api/server.go`: los 4 métodos admin (`ListUsers`, `GetUser`, `UpdateUser`, `ListAuditLog`) pasan de `s.notImplemented(w)` suelto a `RequireAuth(s.tokens)(RequireRole(s.currentUsers, "admin")(...))` envolviendo ese mismo cuerpo — quedan protegidos ya mismo aunque su lógica real siga pendiente de T18/T19.
- Comandos y resultado observado: ver la entrada de T17 en "Evidencia de verificación" arriba.
- Ajuste de Claude: ninguno al código; Codex aplicó directamente la lección de composición real documentada en `CLAUDE.md` tras el hallazgo de T14/T15. Solo verificación: repitió la suite completa, corrió `golangci-lint` local (recién instalado) y confirmó que las 5 pruebas nuevas cubren exactamente los 4 escenarios de la tarea más el caso adicional de revocación sin espera de expiración.
- Dudas abiertas: ninguna. Codex no pudo commitear (mismo bloqueo de sandbox); Claude creó el commit tras revisar.

### T16 · 2026-09-25 · b9c3712
- Qué cambió: `GetCurrentUser`/`UpdateCurrentUser` compuestos directamente en `*Server` (`backend/internal/api/me.go`, nuevo), igual que `Login`/`Register`/`VerifyEmail`: `RequireAuth(s.tokens)` envuelve ambos; el id de usuario sale solo de `claims.Subject` (nunca de la URL ni del cuerpo); roles releídos de BD en cada petición vía `ListRolesForUser` (AM-007/AM-021, nunca del claim del token); `updateCurrentUser` valida `displayName` 1-100 runas y rechaza campos desconocidos en el JSON. `apiUser()` arma la respuesta desde una lista explícita de campos, así que `password_hash` no puede filtrarse ni por accidente. `backend/internal/store/store.go`: `ListRolesForUser` y `UpdateDisplayName` nuevos, más los campos `MFAEnabled`/`LastLoginAt`/`CreatedAt` en `store.User` (ya existían en la BD, faltaban en el tipo). Consulta nueva `UpdateDisplayName` en `db/queries/users.sql`.
- Comandos y resultado observado: ver la entrada de T16 en "Evidencia de verificación" arriba.
- Ajuste de Claude: (1) reescribió `Store.UpdateDisplayName` para usar la consulta sqlc generada (`generated.Queries.UpdateDisplayName`) en vez del SQL parametrizado directo que Codex había dejado porque nunca corrió `sqlc generate` sobre su propia consulta nueva; regeneró y confirmó `gen.go` sin diff. (2) Encontró que T14 y T15 nunca quedaron compuestas en el router real (`Server.RefreshSession`/`Server.Logout` seguían en `s.notImplemented`, 501) — ver la entrada de "Corrección de composición T14/T15" en "Evidencia de verificación" y las notas añadidas en las propias secciones de T14 y T15. La corrección se cerró como commit aparte (`48597d3`), no mezclado con este.
- Dudas abiertas: ninguna nueva. Codex no pudo commitear (mismo bloqueo de sandbox); Claude creó ambos commits de T16 tras revisar y verificar contra PostgreSQL real.

### T15 · 2026-09-25 · bf869b2
- Qué cambió: `backend/internal/auth/logout` (nuevo): `Service.Logout` revoca exactamente el token presentado con una única sentencia condicional (`RevokeRefreshToken`, `status = 'active' AND expires_at > now()`), así que un token desconocido, ya rotado o ya revocado comparten el mismo `ErrInvalidRefreshToken` (mapeado desde `pgx.ErrNoRows`, mismo patrón que T14); registra auditoría `logout` dentro de la misma transacción. `backend/internal/store/logout.go` (nuevo, mismo patrón que `store/refresh.go`/`store/login.go`) y consulta nueva en `db/queries/users.sql`. `POST /api/v1/auth/logout` (`backend/internal/api/logout.go`) lee la cookie `refresh_token`, nunca el cuerpo; responde 204 y `Set-Cookie` de borrado en éxito, 401 con el mismo borrado sin cookie o con token inválido, 500 sin tocar la cookie ante un error real; resuelve la IP con `requestClientIP(r)` desde el primer intento (aprendido de T14). No toca `server.go` (T21).
- Comandos y resultado observado: ver la entrada de T15 en "Evidencia de verificación" arriba.
- Ajuste de Claude: escribió `backend/internal/auth/logout/logout_test.go` (ausente pese a que la propia tarea exige `make test-go`): el paquete solo tenía la prueba de integración (`//go:build integration`) y el stub del handler HTTP, así que su lógica de servicio corría con 0 % de cobertura en la verificación rápida. No encontró bugs nuevos: la corrección de IP de confianza y el mapeo de `pgx.ErrNoRows` (ambos aprendidos en T14) ya venían bien aplicados desde el primer intento de Codex. El hook GGA señaló como observación (no incumplimiento) que `users.sql.go` aparecía modificado sin estar en la lista de archivos a revisar; confirmado que sale de `make gen` sobre la consulta nueva de `db/queries/users.sql`, sin edición manual.
- Dudas abiertas: ninguna. Codex no pudo commitear (mismo bloqueo de sandbox); Claude creó ambos commits tras verificar contra PostgreSQL real. Nota de proceso: esta delegación evitó backticks en el prompt (lección de T14) y no hubo incidentes del forwarder.

### T14 · 2026-09-25 · 110867f
- Qué cambió: `backend/internal/auth/refresh` (nuevo): `Service.Refresh` rota el token dentro de `WithinRefreshTransaction`, con una única sentencia SQL condicional (`RotateRefreshToken`, `FOR UPDATE` sobre el candidato) que garantiza que dos renovaciones concurrentes con el mismo token nunca dejan dos activos: una gana, la otra se trata como reuso (sin ventana de gracia, ADR 0005/T14a). El reuso revoca toda la familia y audita `refresh_reuse_detected` **dentro** de la transacción, que confirma antes de que el servicio devuelva `ErrRefreshReuse` — ese resultado se conserva aunque falle la publicación del evento `security.refresh_reuse_detected` (ver "Ajuste de Claude"). `backend/internal/store/refresh.go` (nuevo, mismo patrón que `store/login.go` de T13) y consulta nueva `RotateRefreshToken` en `db/queries/users.sql` (CTEs `candidate`/`rotated`/`created`, sin migración: el esquema y el índice único `refresh_tokens_one_active_per_family` ya existían desde 000001). `POST /api/v1/auth/refresh` (`backend/internal/api/refresh.go`) lee la cookie `refresh_token`, nunca el cuerpo; responde `accessToken` en JSON y rota la cookie con los mismos atributos de login; sin cookie -> 401. No toca `server.go` ni composición (T21, igual que T13).
- Comandos y resultado observado: ver la entrada de T14 en "Evidencia de verificación" arriba (dos intentos de Codex, RED/GREEN de cada uno, y la corrida real contra PostgreSQL levantado localmente).
- Ajuste de Claude: (1) alcance de archivos — Codex se detuvo interpretando la lista "Archivos" de la tarea (sin `backend/internal/store`) como una restricción; Claude confirmó que el paquete store sí estaba autorizado (mismo patrón que T13) y reanudó el mismo hilo de Codex con esa aclaración más el bug de IP de abajo. (2) bug real: el handler usaba `remoteIP(r.RemoteAddr)` en vez de `requestClientIP(r)`, la IP de confianza que ya resuelve el middleware de T6 y que usan `login.go`/`register.go`/`verify_email.go` — Codex lo corrigió tras la aclaración. (3) `gen.go` con drift de versión: `make gen` de Codex usó un oapi-codegen cacheado sin ldflag de versión (comentario "(devel)"); Claude lo regeneró con el binario correcto, sin diff de contenido real. (4) bug real encontrado por Claude al revisar (confirmado después por el hook GGA antes del commit): un refresh token que no existe en la tabla (forjado, nunca emitido) hacía que `RotateRefreshToken` devolviera `pgx.ErrNoRows` sin mapear a `ErrInvalidRefreshToken`, así que el handler respondía 500 en vez de 401; TDD con `TestRF005_TokenInexistenteDevuelveInvalido` (RED confirmado, luego GREEN) siguiendo el mismo patrón `errors.Is(err, pgx.ErrNoRows)` de `login.go`. (5) segundo hallazgo del hook GGA: si la publicación del evento de seguridad fallaba tras un reuso ya confirmado y committeado, el error genérico pisaba `ErrRefreshReuse` y el handler devolvía 500 sin limpiar la cookie ya comprometida; TDD con `TestRF006_ReusoConFalloDePublicacionSigueRevocandoYDevuelveReuso` (RED, luego GREEN con `errors.Join(refreshErr, ...)` para que `errors.Is` siga reconociendo el reuso). (6) el hook también pidió renombrar `TestHashRefreshToken` y `TestRefreshHandlerRechazaErroresNoAutorizados` al patrón `TestRF005_...` para que `traceability.py` las cuente.
- Dudas abiertas: ninguna. Codex no pudo commitear en ninguno de los dos intentos (mismo bloqueo de sandbox); Claude creó ambos commits tras revisar y verificar contra PostgreSQL real. Nota de proceso ajena al código: durante la primera delegación, el forwarder de `codex:codex-rescue` interpretó fragmentos entre backticks del prompt de instrucciones como comandos de shell reales (incluido un `git push` que falló solo por falta de upstream); no hubo daño (repo verificado intacto) y se reportó como bug del plugin — evitar backticks en prompts futuros a ese agente.

### T18 · 2026-09-25 · ac3ea8e
- Qué cambió: `listUsers`/`getUser`/`updateUser` compuestos directamente en `*Server` (`backend/internal/api/admin_users.go`, nuevo), reemplazando los `s.notImplemented(w)` que T17 ya envolvía con `RequireAuth(s.tokens)(RequireRole(s.currentUsers, "admin")(...))`. Búsqueda por `q` con `ILIKE` parametrizado (sqlc, sin concatenación), filtro `status`, paginación por cursor (`limit` 1-100, default 25). `updateUser` corre dentro de una única transacción (`Store.WithinUserManagementTransaction`, nuevo en `backend/internal/store/admin_users.go`): bloquea con `FOR UPDATE` todas las filas de admins activos (`LockActiveAdminUsers`) y luego la fila objetivo, así que dos actualizaciones concurrentes se serializan sobre el mismo conjunto antes de decidir `ErrLastActiveAdmin`/`ErrSelfDisable` (paquete nuevo `backend/internal/auth/admin`). Auditoría `user_disabled`/`role_changed` dentro de la misma transacción. `RotateRefreshToken` (`db/queries/users.sql`) ahora exige `users.status = 'active'`, así que una cuenta deshabilitada tampoco puede renovar. Sin migración nueva: el bloqueo vive en SQL transaccional, no en un trigger (Q6 lo dejaba como opcional "si es viable").
- Comandos y resultado observado: RED (Codex): servicio inexistente. GREEN: `go test -race ./internal/api ./internal/auth/admin ./internal/store` y la suite completa `go test -race ./...`, ambos en verde (Claude los repitió con `-count=1` tras su propio cambio, ver más abajo). `make gen` sin deriva de `gen.go`/`schema.d.ts`; `specs/07-traceability.md` pasa RF-010 de "parcial" a "completo". `make lint`: solo los 3 avisos preexistentes de `legacy_auth.go` (línea base, hasta T23). Integración no ejecutada (`TEST_DATABASE_URL` no definido en este entorno).
- Ajuste de Claude: encontró un bug real de concurrencia que ninguna prueba (unitaria con stub) podía ver: `LockActiveAdminUsers` bloqueaba varias filas con `FOR UPDATE OF u` **sin `ORDER BY`**, lo que en PostgreSQL puede producir deadlocks entre transacciones concurrentes que no adquieren los bloqueos en el mismo orden. Se añadió `ORDER BY u.id` a la consulta (`db/queries/users.sql`), se regeneró con `sqlc generate` y se repitió la suite completa: sigue en verde. Con el orden fijo, cualquier `updateUser` concurrente contiende primero por el mismo conjunto (admins activos, orden por id) antes de tocar su fila objetivo, así que queda serializado sin interbloqueo.
- Dudas abiertas: ninguna bloqueante. `cmd/api/main.go` sigue sin componer ningún servicio (ni los de T16/T17 tampoco): `SetAdminUserService` existe pero nada lo llama todavía, así que estos endpoints devuelven 503 hasta T21 ("Composición, configuración y humo con el stack") — mismo patrón ya usado para T13 a T17, no es una regresión de T18. Codex no pudo commitear (mismo bloqueo de sandbox); Claude creó el commit tras revisar, verificar y aplicar la corrección de `ORDER BY`.

### T20 · 2026-09-25 · 37c17a5, bd5a702
- Qué cambió: dos límites independientes de ventana deslizante sobre `login_failed` (Decisión Q1). Por cuenta: `LoginAccountMaxFailures` (5) fallos en `LoginFailureWindow` (15 min) ponen `users.status = 'locked'` con `locked_until` real; el propio `Login` lo desbloquea de forma perezosa en el siguiente intento si `locked_until` ya venció (no hay cron). Por IP: `LoginIPMaxFailures` (20, deliberadamente alto para no bloquear una NAT entera) sobre `audit_log` filtrado por `ip`, contando cualquier cuenta incluidas las inexistentes; nuevo índice `audit_log (ip, created_at)` en la migración `000003`. Ambos chequeos corren **antes** de verificar la contraseña, así que una contraseña correcta también falla con 423 mientras dura cualquiera de los dos bloqueos, y la respuesta (`login-locked`, RFC 7807) es idéntica en ambos casos para no revelar cuál se activó. La IP viene siempre de `requestClientIP(r)` (T6): un `X-Forwarded-For` falsificado desde un peer no confiable no cambia nada.
- Comandos y resultado observado: RED (Codex): paquete de lockout inexistente. GREEN (Codex): pruebas del paquete `login` y suite completa. Claude repitió todo con PostgreSQL real (`docker compose up -d db` + migración `000003` aplicada): `go test -race -tags=integration -count=1 ./...` completo en verde dos veces (antes y después del ajuste de abajo), `make lint` solo con los 3 hallazgos preexistentes de `legacy_auth.go`, `make gen` sin deriva real (`specs/07-traceability.md` es el único archivo que cambia) y `python3 scripts/traceability.py --check` al día.
- Ajuste de Claude (primer commit): (1) **bug real de generación**: Codex dejó `db/queries/users.sql`/`audit.sql` con las consultas nuevas (`LockLoginUser`, `UnlockLoginUser`, `CountLoginFailuresByAccount`, `CountLoginFailuresByIP`, y `GetLoginUserByEmail` con `locked_until` y `FOR UPDATE`), pero **nunca corrió `sqlc generate` de verdad**: el código generado seguía reflejando las consultas viejas (`GetLoginUserByEmailRow` sin `LockedUntil`). En vez de regenerar, `backend/internal/store/login.go` traía SQL crudo escrito a mano dentro de `loginWriter`, sorteando sqlc por completo — el mismo antipatrón ya corregido una vez en T5. Claude corrió `sqlc generate` (sí produjo diff real, confirmando el problema), reescribió `loginWriter` para usar `w.queries.*` generado, y repitió toda la suite. (2) **requisito faltante**: Q1 pide auditoría `account_locked` y evento `security.account_locked` además del bloqueo; Codex solo hizo el `UPDATE` de `status`/`locked_until`, sin auditoría ni evento — ningún `TestRF017_*` pedido lo hubiera detectado, porque ninguno verifica ese efecto secundario. Claude agregó `login.SecurityEvent`/`EventPublisher`/`WithEventPublisher` (mismo patrón que `refresh.Service`, commit aparte `bd5a702`), la auditoría dentro de la transacción y una prueba nueva (`TestRF017_CuentaBloqueadaRegistraAuditoriaYPublicaEvento`). Un fallo al publicar el evento se propaga con `errors.Join` (no se traga en silencio) porque el bloqueo y su auditoría ya quedaron comprometidos en la transacción, igual que hace T14 con el reuso de refresh.
- Dudas abiertas: ninguna bloqueante. `cmd/api/main.go` sigue sin componer nada (T21); `WithEventPublisher` queda sin llamar hasta entonces, igual que en `refresh.Service` desde T14. Codex no pudo commitear (mismo bloqueo de sandbox); Claude creó ambos commits.


### T19 · 2026-09-25 · 1ddbdd6
- Qué cambió: `listAuditLog` compuesto directamente en `*Server` (`backend/internal/api/audit_log.go`, nuevo), reemplazando el `s.notImplemented(w)` que T17 ya envolvía con `RequireAuth(s.tokens)(RequireRole(s.currentUsers, "admin")(...))`. Paquete nuevo `backend/internal/auth/auditlog` (separado de `internal/audit`, el escritor append-only, para evitar un ciclo de imports con `internal/api`): filtros `action`/`actorId`/`since` como parámetros sqlc ligados (`db/queries/audit.sql`, consulta nueva `ListAuditLog`), paginación por cursor con el `id` del evento en orden descendente (coincide con los índices `(created_at DESC)` ya existentes desde 000001). Adaptador `backend/internal/store/audit_log.go` reutiliza los mismos helpers (`optionalUUID`, `optionalText`, `nullableUUID`) que el adaptador de T18.
- Comandos y resultado observado: RED (Codex): paquete `internal/auth/auditlog` inexistente. GREEN (Codex): pruebas enfocadas y `go test -race ./...` completo. Claude repitió todo con PostgreSQL real: levantó `db` de `deploy/docker-compose.yml` localmente (el volumen ya tenía las migraciones aplicadas de una sesión anterior), corrió `go test -race -tags=integration -count=1 ./...` completo (todos los paquetes en verde, incluida `internal/auth/auditlog`) y `make lint`/`make gen`/`python3 scripts/traceability.py --check` (sin deriva; RF-011 pasa de 3 a 8 pruebas contadas).
- Ajuste de Claude: (1) faltaba `TestRF011_ElRolDeLaAplicacionNoPuedeModificarLaFila`, uno de los 4 criterios de la tarea — Codex no lo mencionó como omitido en su handoff. Claude lo escribió (mismo enfoque que `TestRF011_IdentityAppNoTieneUpdateNiDeleteSobreAuditLog` de T9: `SET ROLE identity_app`, confirma `permission denied` en `UPDATE`/`DELETE` sobre `audit_log`) y lo verificó contra PostgreSQL real antes de commitear. (2) El hook GGA rechazó el primer intento de commit: `TestAuditLogHandlerPasaFiltrosAlServicio` no seguía el patrón `TestRF011_...` que exige `traceability.py` (no se hubiera contado en la matriz); renombrada a `TestRF011_FiltrosSePasanAlServicio`. (3) Comentario de godoc mal ubicado sobre `optionalAddr` en vez de sobre `ListAuditLog` (señalado por el mismo hook como observación, no bloqueante): reubicado.
- Dudas abiertas: ninguna bloqueante. Mismo patrón que T18: `cmd/api/main.go` no compone `SetAuditLogService` todavía (T21). Nota de proceso: Codex reportó "`make test-integration` passes" sin aclarar que corrió sin `TEST_DATABASE_URL` (se salta con `t.Skip`, no es lo mismo que "pasó" con datos reales); Claude lo verificó de verdad contra PostgreSQL antes de aceptar el reporte.

### T21 · 2026-09-25 · ba0ce22 (+ c6d468f)
- Qué cambió: `backend/cmd/api/main.go` por fin compone todo lo construido en T10-T20 (antes solo armaba `api.NewServer(...)` con los checkers de `/healthz`/`/readyz`, sin llamar ningún `Set*Service`): `token.New` desde `JWT_SIGNING_KEY`, `password.Configure(cfg.Argon2)`, y los ocho servicios (`registration`, `verification`, `login` con su `LockoutConfig`, `refresh`, `logout`, `admin`, `auditlog`, `SetCurrentUserRepository`/`SetTrustedProxies`) — un solo `*store.Store` implementa todas las interfaces `Repository` que cada uno pide. Dos adaptadores nuevos (`loginSecurityEventPublisher`, `refreshSecurityEventPublisher`) traducen `login.SecurityEvent`/`refresh.SecurityEvent` (que solo llevan el `UserID`) a los eventos reales del broker: buscan el usuario por id para completar `email`/`displayName` (que `notify.Render` exige) y, para `security.account_locked`, también `lockedUntil`/`failedAttempts` (exigidos por el AsyncAPI, `specs/04-events/asyncapi.yaml`); si el usuario no se encuentra, se salta la publicación sin fallar la petición (el bloqueo/revocación ya quedó comprometido en la transacción). Decisión Q13: `TRUSTED_PROXIES` pasa a ser la subred fija del compose (`networks.default.ipam`, `172.28.0.0/16`), porque quien reenvía de verdad a la API dentro de esa red es `nginx`. `JWT_SIGNING_KEY` y `TRUSTED_PROXIES` se agregan a `api`/`worker` en el compose con sustitución obligatoria `${VAR:?mensaje}` desde `.env` en la raíz; `.env.example` y el README documentan cómo generarlo.
- Comandos y resultado observado: RED/GREEN de Codex sobre los dos adaptadores nuevos (`backend/cmd/api/main_test.go`). Claude hizo el humo manual real que pedía la tarea, pero no con `make up`: el `make up` completo falla en la construcción de las imágenes `api`/`worker` porque la línea base deliberadamente vulnerable usa `golang:1.22-bullseye`, y el propio `go.mod` exige Go >= 1.25 desde T6 (Q11) — es el mismo problema documentado de VULN-008 (Debian 11 sin paquetes), remediado recién en T27, fuera de alcance acá. En su lugar: levantó `db`, `broker` y `mailpit` con `docker compose up -d` (igual que en T18-T20) y corrió los binarios `cmd/api`/`cmd/worker` locales (Go 1.25 del entorno) contra ellos. Con eso probó de punta a punta: `GET /.well-known/jwks.json` (200), registro real -> correo real en Mailpit -> `verify-email` con el token real del correo, login (cookie `HttpOnly; Secure; SameSite=Strict`, cuerpo solo con `accessToken`), refresh (rota la cookie), reuso de la cookie ya rotada (401, familia revocada, la cookie nueva emitida en ese mismo refresh también queda invalidada), logout (cookie borrada, refresh posterior 401), `GET /admin/users` como no-admin (403) y como admin tras otorgarle el rol por SQL (200, trae también `GET /admin/audit-log`), 6 fallos sobre una cuenta (5 x 401, 6º 423, contraseña correcta también 423 mientras dura) y ráfaga de 20 fallos desde la misma IP contra cuentas distintas e inexistentes (después, la siguiente cuenta válida con contraseña correcta también da 423). `make lint`: solo los 3 hallazgos preexistentes; `make gen`: sin deriva real.
- Ajuste de Claude — dos bugs críticos encontrados por el humo real, ninguno de los dos nuevo de T21 (viven en código de T10/T11/T13/T14/T15, pero nadie los había ejercitado de punta a punta hasta este smoke test):
  1. **`verification.go` nunca decodificaba el token del enlace de correo**: `registration.go` hashea los bytes crudos del token y manda el enlace codificado en base64url; `verification.go` hasheaba el string base64 tal cual, sin decodificar — el hash nunca podía coincidir y `verify-email` devolvía 410 para cualquier enlace real. Los tests existentes nunca lo detectaron porque construían el hash y el string a mano, sin pasar por el límite real de codificación/decodificación.
  2. **`login.go`/`refresh.go` usaban los bytes crudos del refresh token como string de cookie** (`string(refreshRaw)`) en vez de codificarlos: `net/http` descarta en silencio los bytes que no son válidos como cookie-octet al armar `Set-Cookie`, así que la cookie que recibía un cliente real quedaba corrupta y truncada, y nunca podía volver a hashear igual — refresh y logout devolvían 401 siempre para una cookie real (mismo bug de fondo en `logout.go`, que también hasheaba el string sin decodificar). Corregidos los cuatro puntos con `base64.RawURLEncoding`, reescritos los tests que tapaban el bug (`verification_integration_test.go` construía el par hash+string a mano, sin encoder de por medio), y confirmado con el humo real completo de arriba. Commit aparte (`c6d468f`) para no mezclarlo con el propio commit de composición de T21.
  - El hook GGA además rechazó dos veces el commit de composición: (a) el adaptador de `security.account_locked` mandaba `lockedUntil`/`failedAttempts` en cero, incumpliendo el AsyncAPI (`required: [userId, email, lockedUntil, failedAttempts]`) — corregido llevando esos dos valores desde `recordAccountFailure` hasta el evento; (b) los dos tests nuevos de los adaptadores no seguían `Test(RF|RNF)NNN_...`, así que no contaban en `specs/07-traceability.md` — renombrados a `TestRF017_LoginSecurityEventPublisherPublishesRecipient` y `TestRF006_RefreshSecurityEventPublisherSkipsMissingUser`.
- Dudas abiertas: ninguna bloqueante. `make up` con las imágenes de la línea base sigue sin poder construirse hasta la remediación de T27 (Dockerfiles); no es una regresión de T21, es la línea base deliberada. Codex no pudo commitear (mismo bloqueo de sandbox); Claude creó ambos commits tras revisar, verificar contra la infraestructura real y aplicar las dos correcciones.

### T22 · 2026-09-25 · — (solo revisión, sin commit de código)
- Qué se hizo: revisión completa de la Fase 2 (T4 a T21) sin delegar a Codex, como pide la tarea. (1) Barrido del repo completo buscando violaciones de "Seguridad del núcleo IdP" (`AGENTS.md`): sin logs de contraseñas/tokens/secretos en ningún paquete (`grep` de `logger.*password|token|secret|refresh` fuera de tests, cero resultados); sin SQL armado con `fmt.Sprintf`/concatenación fuera de `legacy_auth.go` (línea base, T23); sin `math/rand` fuera de `legacy_auth.go`; sin llamadas `pool.Exec`/`pool.Query` directas en los paquetes de `internal/auth/**` (todo pasa por los adaptadores de `store` generados con sqlc). (2) Suite completa `go test -race -tags=integration -count=1 ./...` contra PostgreSQL/RabbitMQ reales: **todos los paquetes en verde**, incluida la ronda final después de las dos correcciones de T21. (3) `make lint`: solo los 3 hallazgos preexistentes de `legacy_auth.go`. (4) `make gen`: sin ningún diff, ni siquiera en `specs/07-traceability.md` (ya estaba al día desde el commit de T21). (5) Matriz de trazabilidad: el criterio de aceptación de la feature "RF-001 a RF-007, RF-009, RF-010, RF-011 y RF-017 en completo" **se cumple en su totalidad** (los 11 en ✅, confirmado en `specs/07-traceability.md`).
- Hallazgo abierto, no bloqueante para esta tarea pero sí relevante para el cierre de la feature: **cobertura real 46.7 %**, medida con `go test -race -coverprofile=... -covermode=atomic ./...` (el mismo comando de `make test-go`), muy por debajo del 70 % que exige el criterio de aceptación 4 de la feature (RNF-005). Por paquete: `cmd/api` 10.2 %, `cmd/worker` 28.9 %, `internal/auth/admin` 44.7 %, `internal/events` 1.8 % — el resto de `internal/auth/**` está entre 75-83 %. No hay gate de cobertura activo en CI todavía (RNF-005 "🟡 parcial" en la matriz). Esto no es una regresión de ninguna tarea puntual: es la brecha acumulada de no medir cobertura como criterio de cierre tarea por tarea. Queda para que el usuario decida si se abre una tarea dedicada a cerrarla antes de la Fase 3, o si se acepta y se revisa más adelante.
- Dudas abiertas: la decisión de cobertura de arriba, y `RNF-001` (contenerización total) sigue "sin cubrir" en la matriz porque `make up` no construye las imágenes hasta que T27 remedie la línea base (Debian 11/Go 1.22) — coherente con lo ya documentado en T21, no es nuevo.

### T23 · 2026-09-25 · 51a7a4f
- Qué cambió: primera tarea de la Fase 3. Codex quedó bloqueado por tres intentos consecutivos con 401 de la API (clave de cuenta de servicio `sk-svcac...` rechazada por `chatgpt.com/backend-api/codex/responses`; descartado configuración local con `codex doctor` en verde, reinicio del daemon `app-server-broker` y `codex exec` directo sin el plugin, los tres con el mismo error; tampoco lo resolvió un relogin ni el reinicio del límite diario que sugirió el usuario). El usuario decidió que Claude implementara T23 directamente en vez de seguir reintentando con Codex. RED real: `TestRF004_LaRutaLegacyYaNoExiste` (golpea `server.Routes()`, espera 404 en `GET /api/v1/auth/legacy-login`) corrido contra el código sin tocar: 500, no 404 — `LegacyLogin` llama `s.logger.Info(...)` con logger `nil` (convención de los tests de este paquete, `NewServer(nil, "test", nil)`) y el panic lo atrapa `middleware.Recoverer`. GREEN: se borró `legacy_auth.go` entero, se quitó el bloque `r.Route("/api/v1", func(r chi.Router) { r.Get("/auth/legacy-login", s.LegacyLogin) })` de `server.go` (existía solo para esa ruta, nada más dependía de él) y `go mod tidy` sacó `golang-jwt/jwt/v4` (y su indirecta `testify`) de `go.mod`/`go.sum`, sin tocar `jwt/v5` (la sigue usando T8). Se reescribió el comentario de cabecera de `go.mod`: ya no menciona jwt/v4 (retirado), documenta que `pgx v5.5.1` (VULN-022) y `x/text v0.14.0` (VULN-026) siguen fijados a propósito hasta T24.
- Comandos y resultado observado: `go build ./...` limpio. `grep -rn "legacy|md5|math/rand"` sobre `backend` sin `_test.go`: sin resultados. `grep -rn "Access-Control-Allow-Origin"`: sin resultados (sin CORS comodín). `make test-go`: todos los paquetes en verde, cobertura total 47.5 %. `make test-integration` contra PostgreSQL real (contenedor ya corriendo): todos los paquetes en verde, sin bases temporales sobrantes (`pg_database` sin `test_%`). `~/go/bin/golangci-lint run ./...` (v2.13.2): **0 issues** (antes eran exactamente los 3 sembrados de `legacy_auth.go`: 2× G101 + 1× nolintlint). `govulncheck ./...`: ya no aparecen `GO-2024-3250` ni `GO-2025-3553` (los CVE de jwt/v4) en ningún lado de la salida; las 5 vulnerabilidades alcanzables que quedan son todas de `pgx/v5@v5.5.1` (VULN-022, alcance de T24). El hook GGA (pre-commit) revisó `server.go` y `legacy_auth_test.go` contra `AGENTS.md`: `STATUS: PASSED`.
- Ajuste de Claude: además del código, se actualizaron las 7 fichas `security/findings/VULN-{001,002,004,005,006,007,021}-*.md` con el commit de remediación (parte de Codex según `AGENTS.md`, "Protocolo de traspaso", que Claude asumió junto con la implementación). VULN-002, 004, 005, 006, 007 y 021 pasan a `remediado` (su componente entero era `legacy_auth.go` o la dependencia jwt/v4). VULN-001 pasa a `en remediación`, no `remediado`: su ficha cubre tres componentes (`legacy_auth.go`, `docker-compose.yml`, `Dockerfile`) y T23 solo remedia el primero, tal como ya lo aclaraba el propio `Remedia: VULN-001 (código)` de la tarea; el campo anota que los otros dos siguen abiertos y quedan para la tarea que los remedie.
- Dudas abiertas: ninguna bloqueante. Las casillas de "Evidencia (Usuario)" de VULN-001, 002, 004, 005, 006, 007 y 021 siguen sin marcar: falta que el usuario tome las capturas "después" con Claude Desktop contra el run verde de CI de este commit (ver "Protocolo de evidencia"), y para VULN-001 solo corresponde marcarla cuando también se remedie `docker-compose.yml`/`Dockerfile`.

### T23 (evidencia "después") · 2026-09-25/26
- Qué se hizo: dos ajustes de CI encontrados al intentar conseguir un run verde limpio para la evidencia (ninguno es de T23, ambos preexistentes, expuestos porque T23 arregló lo que los tapaba): (1) `specs/07-traceability.md` tenía deriva porque no se corrió `make gen` tras agregar `TestRF004_...` (commit `55404f3`, corrige el job "1 · Deriva entre specs y código"); (2) `frontend/package-lock.json` nunca había existido en el repo desde la Semana 1, lo que rompía el paso de cacheo de `actions/setup-node` en el job "2 · Lint y tipos" — quedaba enmascarado porque antes ese job ya fallaba antes de llegar ahí (por los hallazgos sembrados de `legacy_auth.go`); se generó con `npm install` (lockfileVersion 3, compatible con el npm 10 del CI) y se confirmó `npm run lint`/`typecheck` en verde (mismo commit `55404f3`).
- Se disparó `CI` dos veces por `workflow_dispatch` sobre `feat/idp-semana-2` (no hay PR abierto y `ci.yml` solo dispara por push a `main` o por PR): el run final, `36208104969` sobre `55404f3`, quedó con exactamente los rojos esperados (`3 · Secretos en el historial` → T31, `5 · Dependencias vulnerables` → CVE de pgx, T24, `8 · Configuración de contenedores` → T27) y todo lo demás en verde.
- Desktop capturó el "después" contra ese run y lo volcó a `docs/evidencia/VULN-{002,004,005,006,007,021}/evidencia.json` (campo `despues`) y a las fichas correspondientes: VULN-002/004 por el paso `golangci-lint` ("0 issues."); VULN-021 por el paso de `govulncheck` (GO-2025-3553/GO-2024-3250 ausentes); VULN-005/006/007 (sin gate propio) por el diff del commit `51a7a4f` (`legacy_auth.go` borrado entero, "-131,+0").
- **Discrepancia #6 (Desktop, informe externo)**: las alertas de Security > Code scanning #42, #43 y #81 (Semgrep math-random-used, Semgrep use-of-md5, CodeQL go/log-injection) siguen "Open · On branch main" pese a que el código que las causa ya no existe en `feat/idp-semana-2`. Causa confirmada por Desktop: GitHub referencia el estado de una alerta contra `main` (rama por defecto), no contra la rama donde se hizo el commit, y esta rama todavía no está mergeada — no es un fallo del cierre (Desktop verificó que la alerta #4 sí aparece "closed as fixed" cuando su fix llegó a `main`). Se cerrarán solas en el próximo corte de fase que mergee `feat/idp-semana-2`.
- Dudas abiertas: ninguna. Falta decidir cuándo hacer el próximo corte de fase/PR de la Fase 3 (para que las 3 alertas de Code scanning se cierren solas al llegar a `main`); no es urgente, T23 ya quedó cerrada con su evidencia completa salvo VULN-001 (parcial, a propósito).

### T24 · 2026-09-26 · 001a489
- Qué cambió: Codex intentó primero (bloqueado por DNS del sandbox: `lookup proxy.golang.org: no such host`, sin tocar ningún archivo). Claude lo tomó directo: `github.com/jackc/pgx/v5` v5.5.1 -> v5.11.0 y `golang.org/x/text` v0.14.0 -> v0.41.0 en `backend/go.mod`/`go.sum` (`go get` + `go mod tidy`). Se evitó a propósito `x/text` v0.42.0 (la última): exige `go 1.26.0`, que `go get` intentó subir automáticamente en un primer intento — se revirtió con `git checkout` y se repitió el `go get` fijando v0.41.0, que sigue satisfaciendo el mínimo del hallazgo (>= v0.39.0) sin tocar la directiva `go` (Q11 nunca llegó a activarse: `go.mod` ya declaraba `go 1.25` desde T6). Comentario de cabecera de `go.mod` reescrito para reflejar que VULN-022 y VULN-026 ya no están fijados a propósito.
- Comandos y resultado observado: baseline `go test -race ./...` en verde antes de tocar nada. Tras el upgrade: `go build ./...` limpio; `make test-go` y `make test-integration` (contra PostgreSQL/RabbitMQ reales) en verde; `make lint` (`go vet` + `golangci-lint` v2.13.2 + `eslint`) sin hallazgos; `python3 scripts/traceability.py --check`: matriz al día. `govulncheck ./...`: ya no reporta `GO-2024-2606` (pgx) ni `GO-2026-5970` (x/text) como alcanzables.
- Hallazgo nuevo, fuera de alcance de T24: `govulncheck` reporta `GO-2026-6372` (`github.com/rabbitmq/amqp091-go` v1.9.0, corregido en v1.13.0) como alcanzable — no tiene VULN-NNN asignado todavía (no se inventa aquí, ver "Regla de ids"); queda para que el usuario decida en qué tarea se le crea ficha.
- Dudas abiertas: ninguna bloqueante. Fichas `security/findings/VULN-022-*.md` y `VULN-026-*.md` actualizadas a `remediado` con el commit `001a489`; las casillas de "Evidencia (Usuario)" siguen sin marcar hasta que el usuario confirme la captura "después" contra un run verde de CI (mismo protocolo de T23).

### T25 · 2026-09-26 · 534f13e
- Qué cambió: tomada directo por Claude (necesita `npm install` con red saliente, que Codex no tiene — ver regla nueva en `CLAUDE.md`). `axios` y `lodash` (y `@types/lodash`) retirados de `frontend/package.json` en vez de actualizados: ninguno se importa en `frontend/src` (`client.ts` ya usa `fetch`). `package-lock.json` regenerado con `npm install`; comentario de línea base retirado de `package.json`.
- Comandos y resultado observado: `npm run lint`, `npm run typecheck`, `npm run test` y `npm run build` en verde. `npm audit --audit-level=high` sigue en rojo (exit 1) pero ya no por axios/lodash — `git diff` del lockfile confirma que ninguna otra versión cambió (solo 37 líneas de baja de los tres paquetes retirados).
- Dudas abiertas: el criterio de aceptación literal de T25 ("`npm audit --audit-level=high` sin hallazgos") no se cumple: quedan 15 hallazgos preexistentes desde T23 (`55404f3`), ajenos a VULN-025 — `esbuild`/`vite`/`vitest` (GHSA-67mh-4wv8-2f99), `minimatch` vía `@typescript-eslint/parser` (3 ReDoS), `react-router`/`react-router-dom` (open redirect) y `undici` vía `openapi-typescript` (12 avisos). Todos exigen mayores de versión incompatibles; ninguno tiene ficha ni VULN-NNN asignado. Queda para que el usuario decida en qué tarea se documentan y remedian.

### T26 · 2026-09-26 · afab4e9
- Qué cambió: Codex avanzó bien el compose/Dockerfile (todas las credenciales a `${VAR:?...}` desde `.env`) pero se detuvo con razón en Q21 (`identity_app` `NOLOGIN` sin contraseña; usar `POSTGRES_USER` lo volvería superusuario) y además chocó con "permission denied" de su sandbox contra el socket de Docker al intentar verificar. El usuario eligió la opción de un script en `docker-entrypoint-initdb.d`; Claude terminó: nuevo `deploy/postgres-init/01-identity-app-role.sh`, montado solo en el servicio `db`, da `LOGIN`/contraseña a `identity_app` desde `IDENTITY_APP_PASSWORD` la primera vez que el volumen está vacío.
- Comandos y resultado observado: sin `.env`, `docker compose ps` falla nombrando cada variable faltante (RED real). Con `.env` poblado (el usuario lo escribió con `!` por las reglas `deny` de Claude sobre `.env.*`) y el volumen recreado: `db` queda `healthy`, el log confirma que corrió el script, `migrate` aplica sus 3 migraciones, y una conexión `psql` directa como `identity_app` confirma login y `SELECT` sobre `users`, sin imprimir la contraseña. `make scan-secrets`: mismos 20 hallazgos de siempre, todos del commit de línea base `053e15f` (historial, no árbol de trabajo). `make test` en verde.
- Dudas abiertas: `make up` con el stack completo (api/worker/web) sigue fallando porque `backend/Dockerfile` usa `golang:1.22-bullseye` contra un `go.mod` que exige `go 1.25` desde T6 — ya es alcance explícito de T27 ("builder con versión de Go acorde"), no algo nuevo. Se verificó T26 arrancando solo `db`/`broker`/`mailpit`/`migrate`. Codex etiquetó su pregunta como "Q20", que ya estaba usado por otra decisión (2026-09-21); Claude la renumeró a Q21.

### T27 · 2026-09-26 · a0c64d6
- Qué cambió: tomada directo por Claude (Codex no tiene el socket de Docker; la tarea es casi toda verificación con `docker`/`make build`/`make scan-image`). Builder `golang:1.25-bookworm` y final `gcr.io/distroless/static-debian12:nonroot`, ambos fijados por digest real, para `api` y `worker`. `USER 65532:65532` explícito en las dos (Trivy config no resuelve el `USER` heredado de una base referenciada solo por digest). `ADD` remoto eliminado. `cmd/api` gana un subcomando `healthcheck` (RED/GREEN con `httptest`, 3 pruebas `TestRNF004_*`) porque distroless no tiene `curl`; el compose lo invoca con exec form.
- Comandos y resultado observado: Trivy image encontró CVEs HIGH/CRITICAL corregibles y reales en `golang.org/x/crypto` y `github.com/rabbitmq/amqp091-go` (arrastradas por `go.sum` aunque solo se usa `argon2`) — bloqueaban el propio criterio de aceptación de T27, así que se subieron a `v0.55.0`/`v1.15.0` (ninguna exige `go 1.26`, verificado antes de elegir versión). Tras eso: `make scan-config` sin hallazgos en `backend/Dockerfile`; `make scan-image` en `api` y `worker`: `Total: 0 (HIGH: 0, CRITICAL: 0)`; `docker inspect` confirma `65532:65532`; `make up` deja los 6 servicios arriba con `api` en `healthy`; `make test` y `go test -race ./...` en verde (dos rondas, la segunda tras arreglar `noctx`/`misspell` que marcó `golangci-lint`).
- Dudas abiertas: ninguna bloqueante. `frontend/Dockerfile` sigue con el mismo hallazgo de Trivy config (`USER` root) — es VULN-018, alcance de T28, no se tocó.

### T28 · 2026-09-26 · 8f3d461
- Qué cambió: tomada directo por Claude (mismo motivo que T27). `frontend/Dockerfile`: builder `node:24-bookworm` y final `nginxinc/nginx-unprivileged:stable`, ambos por digest real. `USER 101` explícito (la base ya lo trae por defecto, pero Trivy config no lo resuelve si la base es solo un digest). `frontend/nginx/default.conf` no necesitó cambios: ya escuchaba en 8080, que es donde `nginx-unprivileged` espera.
- Comandos y resultado observado: `npm run build` en verde. `make scan-config`: sin hallazgos en `backend/Dockerfile` ni `frontend/Dockerfile` (los dos Dockerfiles del proyecto quedan limpios). `make scan-image` sobre `identity-hub-web`: `Total: 0 (HIGH: 0, CRITICAL: 0)`. `docker inspect` confirma uid `101`. `make up` + `curl -sI http://localhost:8080/`: `200 OK`, SPA servida. `make test` en verde.
- Dudas abiertas: ninguna.

### T29 · 2026-09-26 · 18dea33
- Qué cambió: tomada directo por Claude (verificación con `make up` real). `frontend/nginx/default.conf`: las cinco cabeceras RNF-009 con `always`, `server_tokens off`, `limit_req_zone` + `location /api/v1/auth/` (más específica que `/api/`) con `limit_req zone=auth burst=5 nodelay` y `limit_req_status 429` (nginx responde 503 por defecto; el contrato ya declara 429). `X-Forwarded-For` de `$proxy_add_x_forwarded_for` a `$remote_addr` en ambas locations de proxy (complementa `TRUSTED_PROXIES`, T6).
- Comandos y resultado observado: `curl -sI http://localhost:8080/` contra el stack real: cinco cabeceras, `Server: nginx` sin versión, SPA y `assets/` sirviendo `200`. Ráfaga real de 30 `POST` a `/api/v1/auth/login`: `429` tras vaciarse el balde. Ciclo real registro → verificación (token de Mailpit) → login por Nginx: `200` con el `Set-Cookie` de `refresh_token` (Path/HttpOnly/Secure/SameSite) intacto. `make test` en verde.
- Dudas abiertas: hallazgo nuevo fuera de alcance — `POST /api/v1/auth/login` con `{}` devuelve `500`, no `400`; es el handler de login (`login.go`, caso `default`), no Nginx. No se toca aquí; anotado para que se decida en qué tarea se corrige.

### T30 · 2026-09-26 · 824be7d
- Qué cambió: tomada directo por Claude (mismo motivo que T27-T29). `postgres:14-bullseye` → `postgres:16-bookworm` y `rabbitmq:3.11-management` → `rabbitmq:4-management`, ambos por digest real (`make clean` antes, por la migración de volumen). `api`/`worker`/`web`: `read_only`, `cap_drop: [ALL]`, `no-new-privileges`, `tmpfs: [/tmp]` donde hacía falta. Comentario "VULN-020" del compose corregido a VULN-024 (Q9). Puertos de `db`/`broker` anotados como decisión deliberada (Q12).
- Comandos y resultado observado: `make up` con los 6 servicios sanos; `docker inspect` confirma `ReadonlyRootfs=true CapDrop=[ALL] SecurityOpt=[no-new-privileges:true]` en los tres contenedores endurecidos. Ciclo real registro → verificación → worker → Mailpit funcionando bajo esa configuración. `make scan-image` en `api`/`worker`/`web`: `Total: 0 (HIGH: 0, CRITICAL: 0)`. `make scan-config`: Trivy config no cubre `docker-compose.yml` en esta versión (solo los 2 Dockerfiles). `make test` en verde.
- Dudas abiertas: VULN-019 queda solo parcial — T30 únicamente cubría `postgres`/`rabbitmq`; `mailpit`, `migrate` y las 4 imágenes de observabilidad siguen en la línea base, fuera de alcance. Trivy sobre el `postgres` nuevo encontró un HIGH en `gosu` (empaquetado por la imagen oficial, no lo controlamos) y otro en un certificado de relleno de Debian; anotado, sin VULN-NNN nuevo.

### T31 · 2026-09-26 · 8054f2e
- Qué cambió: tomada directo por Claude (verificación con `make scan-secrets`, mismo motivo que T27-T30). `.gitleaksignore` con 24 huellas exactas: las 14 ya conocidas (12 de línea base + 2 de Q20) más 10 nuevas encontradas al verificar — 6 falsos positivos de la sintaxis `${VAR:?...}` que T21/T26 metieron en el compose, 3 de asignaciones de struct Go, y 1 (docs/guia-desarrollo.md) con el valor inventado de la línea base copiado en un ejemplo de comando, nunca catalogado.
- Comandos y resultado observado: primera pasada con las 14 huellas conocidas dejó `make scan-secrets` en rojo (10 nuevas); se detuvo la tarea y se preguntó al usuario por la ampliación de alcance (Q8/Q20 no la autorizaban). Autorizado, se añadieron las 10 con su justificación. `make scan-secrets`: `no leaks found` (117 commits). `make test` en verde.
- Dudas abiertas: ninguna sobre T31 en sí. Hallazgo de infraestructura fuera de su alcance: el hook de pre-commit de gitleaks (`.pre-commit-config.yaml`) no está instalado — solo corre `gga run`. Se confirmó con un commit de prueba (secreto con forma de clave AWS) que pasó sin bloquearse; se deshizo de inmediato con `git reset --hard` sin llegar a subirse. Anotado en `CLAUDE.md`; no se toca `.gitleaks.toml` ni los hooks aquí, es decisión de otra tarea.

