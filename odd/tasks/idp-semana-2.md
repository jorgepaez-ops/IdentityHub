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
| VULN-001 | Credenciales incrustadas (código, compose, Dockerfile) | Gitleaks (`secrets`), gosec G101 (`lint`), Trivy secret (imagen) | | | | T23, T26 |
| VULN-002 | MD5 para contraseñas | gosec G401/G501, CodeQL, Semgrep | | | | T23 (con T7) |
| VULN-003 | Credenciales en el compose | Gitleaks | | | | T26 |
| VULN-004 | `math/rand` para tokens | gosec G404 | | | | T23 |
| VULN-005 | SQL por concatenación | gosec G201, CodeQL, Semgrep | | | | T23 (con T5) |
| VULN-006 | JWT sin validar algoritmo | Semgrep, CodeQL | | | | T23 (con T8) |
| VULN-007 | CORS comodín con credenciales | Semgrep (ZAP en semana 3) | | | | T23 |
| VULN-008 | Base Debian 11 (backend) | Trivy image | | | | T27 |
| VULN-009 | `USER root` (backend) | Hadolint DL3002, Trivy | | | | T27 |
| VULN-010 | `apt-get` sin fijar ni limpiar | Hadolint DL3008/DL3009 | | | | T27 |
| VULN-011 | `ADD` desde URL remota | Hadolint DL3020 | | | | T27 |
| VULN-012 | Secreto en `ENV` | Trivy secret, Gitleaks | | | | T26 |
| VULN-013 | Sin CSP, HSTS, X-Frame-Options, nosniff | ZAP (semana 3; sin gate hoy) | | | | T29 |
| VULN-014 | `server_tokens on` | ZAP (sin gate hoy) | | | | T29 |
| VULN-015 | Sin `limit_req` en `/api/v1/auth/*` | AM-001/AM-017 (sin gate hoy) | | | | T29 |
| VULN-016 | `node:18-bullseye` | Trivy image | | | | T28 |
| VULN-017 | `nginx:latest` | Hadolint DL3007 | | | | T28 |
| VULN-018 | Imagen final del frontend como root | Hadolint DL3002 | | | | T28 |
| VULN-019 | Imágenes base antiguas en compose | Trivy image | | | | T30 |
| VULN-020 | `RealIP` de chi suplantable (GO-2026-5774/5775/5777) | govulncheck (`sca`) | | | | T6 |
| VULN-021 | `golang-jwt/jwt/v4` (GO-2024-3250, GO-2025-3553) | govulncheck | | | | T23 |
| VULN-022 | pgx 5.5.1 (GO-2024-2606) | govulncheck | | | | T24 |
| VULN-023 | Reglas por defecto de Gitleaks insuficientes | comparación manual (ya `remediado`) | | | | T31 (revisión) |
| VULN-024 | Sin endurecimiento de contenedores (en el comentario del compose figura como VULN-020) | Trivy config | | | | T30 |
| VULN-025 | axios 0.21.1 / lodash 4.17.15 | npm audit, osv-scanner | | | | T25 |
| VULN-026 | `golang.org/x/text` (GO-2026-5970) | govulncheck | | | | T24 |

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
- [ ] Estado · Ejecutor: `Claude` (con `gh`/navegador según autorización del usuario) · Cubre: RNF-002, RNF-004 · Remedia: —
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
  - Pendiente: `Escaneo semanal` (`workflow_dispatch`, puede abrir incidencias: requiere autorización
    del usuario); URLs de SARIF en la pestaña Security; corregir `baseline-scan.yml` (Trivy sin socket de
    Docker, informe de Gitleaks dentro del árbol escaneado) y repetir el escaneo de imágenes; volcar los
    run IDs en el "Registro de evidencia"; resolver D3 y D4.
- Commit: —

### T0.3 — Evidencia "antes" por hallazgo (capturas en el informe, `evidencia.json` en el repo)
- [ ] Estado · Ejecutor: `Usuario` con Claude Desktop (navegación y capturas, informe externo) y `Claude` (escribe y commitea los `evidencia.json` con los datos de texto que entrega Desktop) · Cubre: todos los VULN · Remedia: — · Depende de: T0.2 (la parte de imágenes, D2)
- **Reestructurada el 2026-09-20 (Q16):** ya no hay `before.png` en el repo (Desktop no puede escribir en él).
  Para cada fila del "Registro de evidencia": `docs/evidencia/VULN-XXX/evidencia.json` con la sección
  `antes` rellena (workflow, run ID/URL, SHA, artefacto, `captura` = sección del informe) y la captura
  "antes" presente en el informe externo, confirmada por el usuario.
- Dos pasadas: (1) los VULN con evidencia ya disponible en los runs 35473988275 (`CI`) y 35476102444
  (`baseline-scan`); (2) los que dependen de T0.2 pendiente: VULN-008, 016, 019 y la parte de Trivy de 009 y 018
  (D2: sin datos de imágenes hasta corregir `baseline-scan.yml` y repetir el escaneo). Las discrepancias D3 y D4
  se anotan como observadas (no aparece / no se detecta), nunca se inventa la alerta.
- Filas "sin gate hoy" (VULN-013, 014, 015): el usuario ejecuta `curl -sI http://localhost:8080` sobre el tag
  y pega la salida (texto); no las toma Desktop.
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
- [ ] Estado · Ejecutor: `Codex` · Cubre: ADR 0007 · Remedia: — · Solo docs · Depende de: T0.2
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
- Commit:

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
- [ ] Estado · Ejecutor: `Codex` · Cubre: RNF-011, ADR 0003 · Remedia: — · Depende de: T1a
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
- Commit:

### T2 — Gate real de deriva: `make gen`, `schema.d.ts` y `ci.yml`
- [ ] Estado · Ejecutor: `Codex` · Cubre: RNF-011 · Remedia: — · Depende de: T1
- `make gen` regenera `gen.go`, `frontend/src/api/schema.d.ts` (`npm run gen:api`) y la matriz.
- `ci.yml`, job `spec-drift`: regenerar ambos y mantener `git diff --exit-code` (ya existe; el
  comentario del job pide justo esto). Solo endurecer.
- Archivos: `Makefile`, `.github/workflows/ci.yml`, `frontend/src/api/schema.d.ts`,
  `frontend/package.json` (solo si falta un script; no tocar axios/lodash).
- Criterios: tras `make gen` el árbol queda limpio; alterar `specs/03-api/openapi.yaml`
  (p. ej. cambiar un `operationId` en una copia local) y no regenerar hace fallar el paso de deriva.
- Verificación: `make gen && git diff --exit-code`; `cd frontend && npm run lint && npm run typecheck && npm run test`; `python3 scripts/traceability.py --check`.
- Commit:

### T3 — Revisión de la Fase 1
- [ ] Estado · Ejecutor: `Claude (revisión)` · Cubre: T1a, T1, T2
- Revisar los tres commits contra sus handoff; comprobar que el diff de T1a solo cambia lo
  autorizado en `openapi.yaml` (cookie, sin `refreshToken` en cuerpos) y que el gate de deriva falla de verdad.
- Commit: —

---

## Fase 2 — Núcleo del IdP

### T4 — Infraestructura de pruebas de integración
- [ ] Estado · Ejecutor: `Codex` · Cubre: RNF-005, RNF-011 · Remedia: — · Depende de: T1
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
- Commit:

### T5 — sqlc: configuración y consultas base
- [ ] Estado · Ejecutor: `Codex` · Cubre: AM-006, RNF-011, VULN-005 (parcial, la retirada va en T23) · Remedia: — · Depende de: T4
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
- Commit:

### T6 — IP de cliente confiable y chi >= v5.3.0
- [ ] Estado · Ejecutor: `Codex` · Cubre: AM-001, AM-010, frontera T2 · Remedia: VULN-020 (chi) · Bloqueada por: T0.3
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
- Commit:
- [ ] Evidencia (`Usuario`): VULN-020 chi: captura "después" en el informe tras run verde del job `sca` sin esos avisos, y datos de texto para el `despues` de `docs/evidencia/VULN-020/evidencia.json`.

### T7 — Argon2id
- [ ] Estado · Ejecutor: `Codex` · Cubre: RF-001, AM-001, AM-004, AM-017, invariante 1, ADR 0004 · Remedia: — (prepara VULN-002) · Depende de: T5
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
- Commit:

### T8 — JWT Ed25519, JWKS y middleware de autenticación
- [ ] Estado · Ejecutor: `Codex` · Cubre: RF-004, AM-003, RNF-003 · Remedia: — (prepara VULN-006/021) · Depende de: T1
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
- Commit:

### T9 — Registro de auditoría: escritor y migración 000002
- [ ] Estado · Ejecutor: `Codex` · Cubre: RF-011, AM-010, AM-011, invariante 5 · Remedia: — · Depende de: T5, T6
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
- Commit:

### T10 — Registro de cuenta (RF-001)
- [ ] Estado · Ejecutor: `Codex` · Cubre: RF-001, RF-012, AM-004, ADR 0006 · Remedia: — · Depende de: T7, T9
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
- Commit:

### T11 — Verificación de correo (RF-002)
- [ ] Estado · Ejecutor: `Codex` · Cubre: RF-002, AM-016 · Remedia: — · Depende de: T10
- `verifyEmail` (`POST /api/v1/auth/verify-email`, cuerpo `{token}`): token de un solo uso, 24 h, hash SHA-256;
  204 y cuenta `active`; segundo uso o expirado -> 410; desconocido -> 410 o 400 (elegir y documentar sin filtrar información); evento `user.email_verified`; audit.
- Criterios: `TestRF002_VerificarActivaLaCuenta`; `TestRF002_EnlaceDeUnSoloUso` (410); `TestRF002_TokenExpiradoDevuelve410`; el token nunca se registra en logs (RNF-012).
- Archivos: `backend/internal/auth/**`, `backend/internal/api/verify_email.go` (+ pruebas), `db/queries/*.sql`.
- Verificación: `make test-go`; `make test-integration`; `make lint`.
- Commit:

### T12 — Worker: plantillas por tipo de evento
- [ ] Estado · Ejecutor: `Codex` · Cubre: RF-012, RF-002, RF-006, RF-017, AM-016, RNF-012 · Remedia: — · Depende de: T11
- El `deliver` actual del worker es genérico (comentario "en la semana 2 se sustituye por
  plantillas"). Crear `backend/internal/notify` (función pura que devuelve asunto y cuerpo por
  `eventType`): `user.registered` incluye el enlace de verificación (`PUBLIC_BASE_URL`, opcional,
  por defecto `http://localhost:8080`; ruta de la página según Q15: el enlace apunta a `/verify-email?token=...`, la página del SPA llega en semana 3 y aquí se verifica con `curl`), `security.refresh_reuse_detected` y
  `security.account_locked` incluyen aviso de seguridad. El worker no registra el cuerpo del mensaje ni el token.
- Criterios: `TestRF012_PlantillaDeRegistroIncluyeElEnlaceDeVerificacion`; `TestRF012_LosAvisosDeSeguridadNoIncluyenSecretos`; el worker sigue sin loguear `d.Body`.
- Archivos: `backend/internal/notify/**`, `backend/cmd/worker/main.go`, `backend/internal/config/**`.
- Verificación: `make test-go`; `make lint`; humo manual en T21.
- Commit:

### T13 — Inicio de sesión (RF-003)
- [ ] Estado · Ejecutor: `Codex` · Cubre: RF-003, RF-004, AM-004, RF-011 · Remedia: — · Depende de: T8, T9, T11
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
- Commit:

### T14a — Enmendar ADR 0005: retirar la ventana de gracia
- [ ] Estado · Ejecutor: `Codex` · Cubre: RF-006, AM-002, ADR 0005 · Remedia: — · Solo docs · Depende de: —
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
- Commit:

### T14 — Rotación de refresh y detección de reuso (RF-005, RF-006)
- [ ] Estado · Ejecutor: `Codex` · Cubre: RF-005, RF-006, AM-002, AM-015, invariante 3, ADR 0005 · Remedia: — · Depende de: T13, T14a
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
- Commit:

### T15 — Cierre de sesión (RF-007)
- [ ] Estado · Ejecutor: `Codex` · Cubre: RF-007 · Remedia: — · Depende de: T14
- `logout` (`POST /api/v1/auth/logout`, sin cuerpo; el token viene en la cookie `refresh_token`, T1a): revoca el token (204) y
  responde `Set-Cookie` de borrado (`refresh_token=; Max-Age=0` con el mismo `Path` y atributos `HttpOnly; Secure; SameSite=Strict`);
  refresh posterior con la cookie vieja -> 401; sin cookie o token desconocido -> 401; audit `logout`.
- Criterios: `TestRF007_LogoutRevocaElRefreshToken` (verifica el `Set-Cookie` de borrado); `TestRF007_LogoutDeTokenDesconocidoDevuelve401`;
  `TestRF007_LogoutSinCookieDevuelve401`; `TestRF007_RefreshTrasLogoutDevuelve401`.
- Archivos: `backend/internal/auth/**`, `backend/internal/api/logout.go` (+ pruebas), `db/queries/*.sql`.
- Verificación: `make test-go`; `make test-integration`.
- Commit:

### T16 — Perfil propio (RF-008)
- [ ] Estado · Ejecutor: `Codex` · Cubre: RF-008 · Remedia: — · Depende de: T8, T13
- `getCurrentUser` y `updateCurrentUser` (solo `displayName`, 1-100): `RequireAuth`; sin token 401; nunca devuelve `password_hash`
  (invariante 1); esquema `User` del OpenAPI (incluye `roles` leídos de BD).
- Criterios: `TestRF008_MeDevuelveElUsuarioAutenticado`; `TestRF008_SinTokenDevuelve401`; `TestRF008_ActualizaElNombre`; el JSON no contiene el hash.
- Archivos: `backend/internal/api/me.go` (+ pruebas), `db/queries/*.sql`.
- Verificación: `make test-go`; `make test-integration`.
- Commit:

### T17 — RBAC (RF-009)
- [ ] Estado · Ejecutor: `Codex` · Cubre: RF-009, AM-007, AM-021 · Remedia: — · Depende de: T8, T13
- Middleware `RequireRole("admin")` en `/api/v1/admin/*`: roles leídos de la BD en cada petición (AM-007),
  no del claim; token con `roles` alterado -> 401 (firma inválida); `user` -> 403; suspendido/deshabilitado -> 401.
  Sin endpoint de auto-asignación de roles.
- Criterios: `TestRF009_UserRecibe403EnRutasAdmin` (tabla que recorre **todas** las rutas `/admin/*` del OpenAPI, AM-021);
  `TestRF009_AdminRecibe200`; `TestRF009_RolesAlteradosEnElTokenDanEl401`; un usuario cuyo rol cambió en BD pierde el acceso sin esperar la expiración.
  Cómo nace el primer admin: Q5 (decidido: alta manual por SQL en desarrollo, documentada; los tests crean admins directamente en BD).
- Archivos: `backend/internal/api/rbac.go` (+ pruebas), `db/queries/*.sql`.
- Verificación: `make test-go`; `make test-integration`; `make lint`.
- Commit:

### T18 — Administración de usuarios (RF-010)
- [ ] Estado · Ejecutor: `Codex` · Cubre: RF-010, RF-011, AM-006, invariante 6 · Remedia: — · Depende de: T17
- `listUsers` (`q` parametrizado con `LIKE` seguro, `status`, cursor, `limit` 1-100), `getUser`,
  `updateUser` (`status`, `roles`): un admin no puede deshabilitarse a sí mismo (400); nunca queda
  sin admin activo (mecanismo según Q6: comprobación en el servicio con transacción y bloqueo de fila, y trigger en migración nueva si es viable); audit `user_disabled` con el actor; `role_changed` al cambiar roles; un usuario deshabilitado no puede iniciar sesión ni renovar.
- Criterios: `TestRF010_AdminDeshabilitaYElUsuarioNoEntra`; `TestRF010_AdminNoPuedeDeshabilitarseASiMismo` (400); `TestRF010_BusquedaNoEsInyectable` (`' OR '1'='1' --` como `q`); `TestRF010_NoSeDejaElSistemaSinAdmin`.
- Archivos: `backend/internal/auth/**`, `backend/internal/api/admin_users.go` (+ pruebas), `db/queries/*.sql`, posible migración `000003_*` según Q6.
- Verificación: `make gen && git diff --exit-code`; `make test-go`; `make test-integration`; `make lint`.
- Commit:

### T19 — Consulta del registro de auditoría (RF-011)
- [ ] Estado · Ejecutor: `Codex` · Cubre: RF-011, AM-010 · Remedia: — · Depende de: T9, T17
- `listAuditLog` (filtros `action`, `actorId`, `since`, `limit`, `cursor`; solo admin), paginación por `id`.
- Criterios: `TestRF011_UnLoginFallidoCreaUnaFila`; `TestRF011_ListarRequiereAdmin` (403 para user);
  `TestRF011_FiltraPorAccionYActor`; `TestRF011_ElRolDeLaAplicacionNoPuedeModificarLaFila` (reutiliza el enfoque de T9 de extremo a extremo).
- Archivos: `backend/internal/api/audit_log.go` (+ pruebas), `db/queries/audit.sql`.
- Verificación: `make test-go`; `make test-integration`.
- Commit:

### T20 — Bloqueo por fuerza bruta (RF-017)
- [ ] Estado · Ejecutor: `Codex` · Cubre: RF-017, AM-001, VULN-020 (chi, frontera T2), estado `locked` · Remedia: — · Depende de: T13, T6, T12
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
- Commit:

### T21 — Composición, configuración y humo con el stack
- [ ] Estado · Ejecutor: `Codex` · Cubre: RNF-001, RNF-003, RF-001 a RF-011 · Remedia: — · Depende de: T10 a T20
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
- Commit:

### T22 — Revisión de la Fase 2
- [ ] Estado · Ejecutor: `Claude (revisión)` · Cubre: T4 a T21
- Revisar commits y handoff; ejecutar `make test-integration` y el humo; auditar los criterios de
  `AGENTS.md`, "Seguridad del núcleo IdP".
- Commit: —

---

## Fase 3 — Remediación de la línea base (todas `Remedia:`; bloqueadas por T0.3)

Cada tarea de esta fase: el `Commit:` es el commit de remediación; después Codex actualiza la
ficha (T36). Cada `Evidencia` la completa el `Usuario` tras el run verde del gate: captura "después"
en el informe externo (Desktop) y datos de texto para el `despues` del `evidencia.json` (lo escribe Claude).

### T23 — Retirar `legacy_auth.go` y su ruta
- [ ] Estado · Ejecutor: `Codex` · Cubre: AM-003, AM-006, AM-012, RF-004 · Remedia: VULN-001 (código), VULN-002, VULN-004, VULN-005, VULN-006, VULN-007, VULN-021 · Bloqueada por: T0.3, T22
- Borrar `backend/internal/api/legacy_auth.go` entero, la ruta `/auth/legacy-login` de `server.go` y
  el `require github.com/golang-jwt/jwt/v4` (`go mod tidy`); el reemplazo ya existe (T5 sqlc, T7
  Argon2id, T8 JWT/JWKS con jwt v5, `crypto/rand` en los tokens opacos). Un solo commit: el
  archivo se elimina completo (así lo dicta su encabezado y la ficha VULN-001); todos esos VULN
  comparten SHA de remediación.
- Criterios: `TestRF004_LaRutaLegacyYaNoExiste` (404); `grep -rn "legacy\|md5\|math/rand" backend --include=*.go` sin resultados de código de producción; `go.mod` sin `jwt/v4`; no hay CORS comodín.
- Verificación: `make test-go`; `make test-integration`; `make lint` y `cd backend && golangci-lint run ./...` sin hallazgos G101/G201/G401/G404; `govulncheck` sin GO-2024-3250/GO-2025-3553.
- Commit:
- Evidencia (`Usuario`), una casilla por carpeta, todas tras run verde de `CI`:
  - [ ] VULN-001  - [ ] VULN-002  - [ ] VULN-004  - [ ] VULN-005  - [ ] VULN-006  - [ ] VULN-007  - [ ] VULN-021

### T24 — Dependencias Go: pgx y `golang.org/x/text`
- [ ] Estado · Ejecutor: `Codex` · Cubre: AM-006, AM-009, RNF-004 · Remedia: VULN-022 (pgx >= 5.5.4, mejor la última estable), VULN-026 (x/text GO-2026-5970 -> v0.39.0 o superior) · Bloqueada por: T0.3
- Subir pgx y x/text, `go mod tidy`; revisar el resto de avisos de `govulncheck`. Si esto obliga a subir la
  directiva `go` o `GO_VERSION` (ci.yml, baseline-scan.yml, scheduled-scan.yml usan 1.22), detenerse y
  preguntar (decisión Q11: anotarlo en "Preguntas nuevas"); no subirla sin aprobación. Retirar el comentario de línea base de `go.mod` solo al cerrar sus VULN.
- Archivos: `backend/go.mod`, `backend/go.sum`.
- Criterios: `govulncheck ./...` sin avisos alcanzables; `TestRF012_*` y el resto siguen verdes.
- Verificación: `make test-go`; `make test-integration`; `cd backend && go run golang.org/x/vuln/cmd/govulncheck@latest ./...`.
- Commit:
- Evidencia (`Usuario`):  - [ ] VULN-022  - [ ] VULN-026 (x/text)

### T25 — Dependencias del frontend
- [ ] Estado · Ejecutor: `Codex` · Cubre: AM-009, RNF-004 · Remedia: VULN-025 (axios 0.21.1, lodash 4.17.15) · Bloqueada por: T0.3
- Actualizar o retirar axios y lodash (verificar si `src/` los usa: `client.ts` usa `fetch`); versionar
  `frontend/package-lock.json` (hoy no está commiteado); retirar el aviso de línea base del `package.json` al cerrar.
- Criterios: `npm audit --audit-level=high` sin hallazgos; lint, tipos y tests verdes.
- Verificación: `cd frontend && npm install && npm audit --audit-level=high && npm run lint && npm run typecheck && npm run test && npm run build`.
- Commit:
- Evidencia (`Usuario`):  - [ ] VULN-025 (axios/lodash)

### T26 — Secretos fuera del compose y de los `ENV`
- [ ] Estado · Ejecutor: `Codex` · Cubre: RNF-003, AM-012 · Remedia: VULN-003, VULN-012, VULN-001 (compose/Dockerfile) · Bloqueada por: T0.3, T21
- `deploy/docker-compose.yml`: todas las credenciales por `${VAR:?...}` desde `.env` (Postgres, RabbitMQ,
  DSN, migrate); `DATABASE_URL` de la API con el rol `identity_app` (T9; Q7: aquí se le asigna la credencial desde `.env`); `.env.example` sin valores.
  No renombrar aquí el comentario `VULN-020` del compose (eso es T30).
  `backend/Dockerfile`: eliminar los `ENV` con secretos (`DB_PASSWORD`, `API_SIGNING_KEY`). `README.md`/`Makefile` actualizados si mencionan credenciales.
- Criterios: `make up` falla con mensaje claro si falta una variable; con `.env` local arranca; `make scan-secrets` sin hallazgos en el árbol de trabajo (el historial: T31).
- Verificación: `make up && make ps`; `make scan-secrets`; `make test`.
- Commit:
- Evidencia (`Usuario`):  - [ ] VULN-003  - [ ] VULN-012  - [ ] VULN-001 (Gitleaks/Trivy secret)

### T27 — Dockerfile del backend
- [ ] Estado · Ejecutor: `Codex` · Cubre: RNF-004, RNF-008, AM-020 · Remedia: VULN-008, VULN-009, VULN-010, VULN-011 · Bloqueada por: T0.3, T26
- Base distroless (o equivalente sin shell) fijada por digest real (verificado con `docker`), `USER` no-root,
  sin `apt-get` en la imagen final, sin `ADD` remoto; builder con versión de Go acorde; el healthcheck del compose usa `curl`: reemplazarlo por un mecanismo sin shell (p. ej. subcomando de sonda del binario) y ajustar el compose.
- Criterios: `hadolint` sin DL3002/DL3008/DL3009/DL3020; Trivy image sin CRITICAL/HIGH corregibles (`--ignore-unfixed`); la imagen arranca como uid distinto de 0.
- Verificación: `make scan-config`; `make build && make scan-image`; usuario de la imagen con `docker inspect --format '{{.Config.User}}' <imagen>` (en distroless no hay shell para `id`).
- Commit:
- Evidencia (`Usuario`):  - [ ] VULN-008  - [ ] VULN-009  - [ ] VULN-010  - [ ] VULN-011

### T28 — Dockerfile del frontend
- [ ] Estado · Ejecutor: `Codex` · Cubre: RNF-004, RNF-008 · Remedia: VULN-016, VULN-017, VULN-018 · Bloqueada por: T0.3
- Node LTS vigente en el builder, `nginxinc/nginx-unprivileged` fijado por digest real, usuario no-root; puerto 8080 se mantiene.
- Criterios: Hadolint sin DL3007/DL3002; Trivy image sin HIGH/CRITICAL corregibles; `make up` sirve la SPA en 8080.
- Verificación: `make scan-config`; `make build`; `make scan-image`; `cd frontend && npm run build`.
- Commit:
- Evidencia (`Usuario`):  - [ ] VULN-016  - [ ] VULN-017  - [ ] VULN-018

### T29 — Nginx: cabeceras, `server_tokens` y `limit_req`
- [ ] Estado · Ejecutor: `Codex` · Cubre: RNF-009, AM-001, AM-015, AM-017, frontera T2 · Remedia: VULN-013, VULN-014, VULN-015 · Bloqueada por: T0.3, T28
- `frontend/nginx/default.conf`: las cinco cabeceras de RNF-009 con `always` (CSP sin `unsafe-inline`, HSTS,
  `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`), `server_tokens off`, `limit_req` sobre
  `/api/v1/auth/`, y `X-Forwarded-For $remote_addr` (sobrescribe la cabecera del cliente, complementa T6). La CSP debe permitir el SPA de Vite ya construido.
  Nginx no debe alterar ni eliminar el `Set-Cookie` del refresh token (T1a) ni reescribir su `Path`.
- Criterios: `curl -sI http://localhost:8080/` muestra las cabeceras y no `nginx/x.y.z`; ráfaga de peticiones a `/api/v1/auth/login` recibe 429 (el OpenAPI ya declara `429`); la SPA sigue cargando;
  el `Set-Cookie` de `/api/v1/auth/login` llega intacto a través de Nginx.
- Verificación: `make up`; `curl -sI http://localhost:8080/`; ráfaga con `for i in $(seq 1 30); do curl -s -o /dev/null -w '%{http_code}\n' -X POST http://localhost:8080/api/v1/auth/login -H 'Content-Type: application/json' -d '{}'; done`.
- Commit:
- Evidencia (`Usuario`):  - [ ] VULN-013  - [ ] VULN-014  - [ ] VULN-015 (`curl -sI` antes/después; sin gate hoy)

### T30 — Compose: imágenes base y endurecimiento
- [ ] Estado · Ejecutor: `Codex` · Cubre: RNF-008, RNF-004, AM-014, AM-020 · Remedia: VULN-019, VULN-024 (sin endurecimiento de contenedores; el comentario del compose lo llama VULN-020) · Bloqueada por: T0.3, T26, T27
- Actualizar `postgres` y `rabbitmq` a versiones con soporte, fijadas por digest real (avisar: volúmenes previos pueden requerir
  `make clean`); en `api`, `worker` y `web`: `read_only`, `cap_drop: [ALL]`, `security_opt: [no-new-privileges:true]`, `user` no-root, `tmpfs` donde haga falta;
  puertos de `db` y `broker` publicados al host en desarrollo (Q12, decidido: se mantienen y se tratan en `docker-compose.prod.yml`, semana 3; anotar).
- Corregir en el compose los comentarios que dicen `VULN-020` (cabecera y el bloque de endurecimiento) para que digan
  `VULN-024`, ya que `VULN-020` es el hallazgo de chi (Q9; la equivalencia está en la ficha VULN-024, T0.5). Esto solo se hace aquí, al tocar el compose.
- Criterios: `make up` sano con esa configuración; Trivy config sin HIGH/CRITICAL en el compose (si el escáner lo cubre; anotar si no).
- Verificación: `make up && make ps`; `make scan-config`; `make scan-image`; `make test`.
- Commit:
- Evidencia (`Usuario`):  - [ ] VULN-019  - [ ] VULN-024

### T31 — Gitleaks frente al historial: `.gitleaksignore` por huella exacta
- [ ] Estado · Ejecutor: `Codex` (decisión ya tomada por el `Usuario`, Q8) · Cubre: RNF-003, AM-012 · Remedia: relacionada con VULN-023 · Bloqueada por: T26 y por la lista de huellas que entrega el `Usuario` desde el run de Gitleaks de T0.2
- El job `secrets` usa `fetch-depth: 0`: los secretos sembrados permanecerán en el historial aunque T23/T26 los retiren, así que el gate no
  pasará solo. **Decisión del usuario (2026-09-19): opción (a)**, `.gitleaksignore` en la raíz con **huella exacta**, limitado a los **12 hallazgos conocidos de la línea base**. Las opciones (b) allowlist por commit y (c) reescribir historial quedan **descartadas** ((c) también por el ADR 0007: el historial es la evidencia). Esta decisión es la aprobación explícita del usuario para esta excepción concreta; no autoriza ninguna otra.
- Entrada: el usuario entrega a Codex la lista de las 12 huellas (`Fingerprint`, formato `commit:archivo:regla:línea`, sin `Secret` ni `Match`) tomadas del run de T0.2. Si la lista no llega, la tarea sigue bloqueada; si no son exactamente 12 o alguna no corresponde a un VULN, detenerse y anotarlo en "Preguntas nuevas".
- Crear `.gitleaksignore`: una línea por huella, cada una precedida por un comentario con su `VULN-NNN` y una justificación breve ("secreto sembrado de la línea base, ADR 0007; se conserva como evidencia del antes"). Sin comodines, sin rutas ni patrones, sin huellas adicionales; no tocar `.gitleaks.toml`.
- Criterios: `secrets` en verde; exactamente 12 huellas, cada una con su VULN y justificación; un secreto nuevo de prueba (añadido en local y descartado, nunca commiteado) sigue siendo detectado; VULN-023 sigue `remediado` (12 de 12 con `.gitleaks.toml`).
- Verificación: `make scan-secrets` y run de `CI` job `secrets`; `grep -c '^[^#[:space:]]' .gitleaksignore` = 12.
- Commit:
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
| 0 — Línea base y evidencia "antes" | T0.1 a T0.5 (5) | 0 |
| 1 — Contrato ejecutable | T1a, T1 a T3 (4) | 0 |
| 2 — Núcleo del IdP | T4 a T22 y T14a (20) | 0 |
| 3 — Remediación | T23 a T32 (10) | 0 |
| 4 — Cierre | T33 a T38 (6) | 0 |
| **Total** | **45** | **0** |

Tareas nuevas respecto a la versión anterior (43): `T1a` (enmienda OpenAPI, cookie) y `T14a`
(enmienda ADR 0005, sin ventana de gracia). Los ids existentes no cambian.

## Evidencia de verificación

(Codex añade aquí, por tarea, `<comando>: <resultado observado>` cuando cierre cada una;
las líneas de RED/GREEN van en el handoff.)

- T1a · `python3 -m openapi_spec_validator specs/03-api/openapi.yaml`: OK; `python3 scripts/traceability.py --check`: matriz al día.
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

### Preguntas nuevas (Codex)

(Ninguna. Codex anota aquí cualquier ambigüedad nueva y detiene esa tarea; no hay nada abierto hoy.)

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
