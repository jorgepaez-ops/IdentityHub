# cierre-entrega — Refactorización, versión y publicación antes de la entrega del 2026-10-23

## Objetivo

Dejar el código de producción legible (KISS, una responsabilidad por función), publicar la versión
`vX.Y.Z` con el informe adjunto y cerrar T17 (Docker Hub), en ese orden (decisión del usuario,
2026-10-06).

## Tareas

- [x] **C1 — Refactorización por complejidad cognitiva (Sonar S3776).** Extraer pasos con nombre propio
  sin cambiar comportamiento, uno por commit, con las pruebas existentes (unitarias, integración contra
  Postgres y E2E) como red y revisión nativa por commit:
  `backend/internal/auth/admin/admin.go:126` (33), `backend/internal/auth/login/login.go:128` (32;
  conservar la igualación de tiempos con la contraseña señuelo y la transacción),
  `backend/internal/config/config.go:87` (23), `backend/internal/auth/mfa/mfa.go:220` (21),
  `backend/internal/api/rbac.go:18` (16). Opcional: `frontend/src/features/admin/UserDrawer.tsx:97`
  (18) y `contabilidad/src/auth/flow.ts:54` (16). Las pruebas y los scripts quedan como están.
  - [x] C1a `login.go` — `Login` extraído en `authenticate`, `ipRateLimited`, `rejectUnknownUser`,
    `resolveLock`, `rejectKnownUser`, `rehashIfNeeded`, `publishLockEvent` y `tolerateInvalidPassword`.
    Ruta: delegado (Codex sin capacidad → Sonnet), revisado por Opus. Evidencia: `gocognit` máximo 9
    (antes 60 con `gocognit`, 32 en Sonar); `go test` del paquete con Postgres real 0 omitidas, 80,4 %;
    `go vet`, `gofmt` y `golangci-lint` limpios.
    Commit `8bb5238`. Revisión nativa: riesgo alto, concedida, 4 lentes, aprobada y acusada
    (`review-81cbd5bb8a740407`); 1 sugerencia no bloqueante (R2-001, legibilidad, `login.go:228-230`).
  - [x] C1b `admin.go` — `UpdateUser` extraído en `rejectSelfRoleAssignment`, `validateStatusChange`,
    `applyUserUpdate`, `validateRequestedRoles`, `lockAndLoadTarget`, `removesActiveAdmin`,
    `applyStatusChange` y `applyRoleChange`. Ruta: delegado (Codex rechazó `gpt-6.1-sol` por la cuenta;
    el usuario eligió Sonnet), revisado por Opus. Evidencia: `gocognit` máximo 8 (antes 33 en Sonar);
    pruebas del paquete con Postgres real 0 omitidas, 85,4 %; `./internal/api/` ok; `go vet`, `gofmt` y
    `golangci-lint` limpios. Commit `5aa27f9`. Revisión nativa: riesgo alto, concedida, 4 lentes, aprobada
    y acusada (`review-90987d64715a5523`); 1 sugerencia no bloqueante (R2-001, legibilidad, `admin.go:130-132`).
  - [x] C1c `mfa.go` — `Verify` extraído en `decodeVerifyInput`, `verifyChallenge`, `issueSession`,
    `createRefreshToken`, `createHubSession` y `finishVerify` (resultado en `verifyOutcome`). Ruta: delegado
    (Sonnet), revisado por Opus. Evidencia: `gocognit` de `Verify` 4 y máximo 9 en los nuevos (antes 53 con
    `gocognit`, 21 en Sonar); pruebas con Postgres real 0 omitidas, 80,8 %; `./internal/api/` y
    `./internal/auth/...` ok; `go vet`, `gofmt` y `golangci-lint` limpios. `Issue` (23) y `Resend` (21)
    superan 15 en `gocognit` pero Sonar no los marcó: fuera de alcance.
    Commit `156895f`. Revisión nativa: riesgo alto, concedida, 4 lentes, aprobada y acusada
    (`review-04145e4b6b15b1ae`), sin sugerencias.
  - [x] C1d `config.go` — los cierres de `load` pasan a `problemCollector` (métodos `required`, `optional`,
    `duration`, `integer`, `boundedInteger`, `positiveInteger`, `positiveDuration`) y la carga se divide en
    `loadServerSecrets`, `loadPasswordConfig` y `loadBootstrapAdminEmail`, con el mismo orden de mensajes.
    Ruta: delegado (Sonnet, junto con C1e), revisado por Opus. Evidencia: `load` sale del top 5 de
    `gocognit` (antes 25; 23 en Sonar); `./internal/config/` 95,0 %, `./internal/api/` y `./cmd/...` ok,
    0 omitidas; `go vet`, `gofmt` y `golangci-lint` limpios.
    Commit `ef43a75`. Evaluación nativa: riesgo medio, bajo el presupuesto; queda pendiente en el corte y se
    revisa junto con C1e.
  - [x] C1e `rbac.go` — `RequireRole` delega en `authorizeRole` (devuelve `nil` o la respuesta de error),
    con `writeAuthorizationLoadFailed` y `hasRole`. Ruta: delegado (Sonnet, junto con C1d), revisado por Opus.
    Evidencia: `authorizeRole` 6 y `RequireRole` 3 en `gocognit` (antes 22; 16 en Sonar); `./internal/api/`
    81,9 %, 0 omitidas; `go vet`, `gofmt` y `golangci-lint` limpios.
    Commit `917327b`. Evaluación nativa: riesgo medio, bajo el presupuesto (corte C1d+C1e pendiente).
  - Cierre de C1 (2026-10-07): `make up` con las imágenes nuevas y `make e2e`: 52/52 en verde (3,6 min).
- **Hallazgo fuera de alcance (C1):** `gocognit -over 15` marca además, en producción, `bootstrap.Ensure` (46),
  `rolegrid.Update` (31), `refresh.Refresh` (31), `employee.CreateEmployee` (30), `mfa.Issue` (23),
  `mfa.Resend` (21), `notify.Render` (20), `passwordreset.Confirm` (19), `invitation.Accept` (17),
  `oauth.Exchange` (16), `invitationresend.Resend` (16) y `testdb.New` (17), más el generado `gen.go`.
  Sonar no los marcó (cuenta los cierres de transacción con menos peso). Decidir si se atienden.
- **Orden invertido (decisión del usuario, 2026-10-07):** C3 va antes que C2. Motivo: no existe workflow de
  release (solo `ci.yml`, `baseline-scan.yml` y `scheduled-scan.yml`); si `v1.0.0` saliera antes de C3, ese
  tag nunca tendría imágenes en Docker Hub y C3 obligaría a un `v1.0.1`. Con el orden nuevo, un solo `v1.0.0`
  publica en vivo el release con el PDF y las imágenes. **Revisarlo con el equipo en la sesión en vivo.**
- [ ] **C3 — Docker Hub (T17 de `idp-semana-4.md`).** Workflow de release en el tag `vX.Y.Z`: imágenes `api`,
  `worker` y `web` con `vX.Y.Z` y `latest`, SBOM con Syft y firma con Cosign (ADR 0011), environment
  protegido `dockerhub`. Requiere Q1: namespace y token de Docker Hub del usuario. Se prueba con el tag
  `v0.9.0` (y se borra o se deja marcado como prueba) antes de `v1.0.0`.
- [ ] **C2 — Tag `v1.0.0` y release** con el PDF del informe adjunto (`make informe`; el PDF no se
  versiona). En vivo frente al equipo (T21 de `idp-semana-4.md`). Requiere el merge del PR #20.
  Pasos: (1) merge del PR #20 y `main` local al día con CI en verde; (2) `make informe` y revisión del PDF;
  (3) notas del release revisadas por el usuario; (4) en vivo: `git tag -a v1.0.0` sobre el merge,
  `git push origin v1.0.0` (dispara el workflow de C3), `gh release create v1.0.0` con el PDF adjunto;
  (5) evidencia: URL del release y de las imágenes en Docker Hub, en T17, T21 y aquí.

- [x] **D — Hallazgos sueltos y deuda de Sonar (decisión del usuario, 2026-10-07: «soluciona el 4 y 5 y luego
  merge del PR20»).** Cambio posterior del usuario: el PR #20 se fusiona solo con C1 y la tarea D va en un PR nuevo. Ejecutor: Sonnet; revisión de Opus; un commit por
  tarea y evaluación nativa por commit.
  - [x] D1 — Regla `contrasena-en-variable-de-entorno` de `.gitleaks.toml`: `\s*` tras `[:=]` → `[ \t]*`, para
    que no cruce saltos de línea. Comprobar que sigue detectando los secretos sembrados de la línea base.
    Evidencia: gitleaks v8.24.3 sobre `v0.0.0-vuln-baseline`: 11 hallazgos con la regla vieja y con la nueva,
    mismos (regla, archivo, línea); árbol actual 62 → 61 (desaparece el falso positivo multilínea). Commit `a7494bc`.
  - [x] D2 — Trivy local frente a CI: **ya resuelto antes de esta sesión**. `Makefile:189` fija
    `aquasec/trivy:0.70.0` por digest, la misma versión que `trivy-action` v0.36.0 en CI. La nota de
    `pulido-frontend.md` estaba desactualizada.
  - [x] D3 — Estilo inline bloqueado por la CSP en `/invitations/accept` (visto en WebKit): encontrar qué lo
    inyecta y corregirlo sin relajar la CSP.
    Resultado: **no es un defecto de la aplicación.** Exploración (Sonnet, solo lectura): `frontend/src` no crea
    `<style>`, no usa `insertRule`, `cssText` ni `style={{}}`; el build tiene un solo `<link rel="stylesheet">` y
    ningún CSS inyectado en tiempo de ejecución; no hay rutas perezosas. Prueba en tiempo de ejecución (Opus,
    2026-10-07, stack con `make up`): Playwright WebKit y Chromium sobre `/login`, `/invitations/accept` y
    `/password/reset`, enfocando y llenando la contraseña: 0 `<style>`, 0 atributos `style`, 0 eventos
    `securitypolicyviolation`. Explicación más probable: el autocompletado «Contraseña segura» de Safari en campos
    `autocomplete="new-password"`, que inyecta su propio estilo y la CSP bloquea; Playwright WebKit no lo tiene,
    así que no se reproduce. No se relaja la CSP ni se quita `new-password` (empeoraría los gestores de
    contraseñas). Se documenta como ruido del navegador.
  - [x] D4 — 11 falsos positivos de secretos en Sonar: `.sonarcloud.properties` con
    `sonar.exclusions=security/evidence/**` (salidas de escáneres conservadas como evidencia del «antes»,
    no código; `sonar.issue.ignore.*` no está soportado en análisis automático) y el ejemplo de
    `.gitleaks.toml:8` sin la contraseña literal. Documentarlo en «Alertas abiertas conocidas» del `README.md`.
    Commits `a7494bc` (ejemplo de `.gitleaks.toml`) y `cb9caa0` (`.sonarcloud.properties` y `README.md`). El cierre
    real en SonarCloud se confirma tras el análisis de `main` después del merge.
  - [x] D5 — 9 alertas reales de cadena de suministro en workflows: `go install …@latest`/`@vX` (S8545) por
    herramientas fijadas con `go.sum`; `pip install` (S8541) con hashes y `--only-binary :all:`;
    `npm install --no-package-lock` (S8543) por `npm ci --ignore-scripts`; `npm install --package-lock-only`
    (S6505) con `--ignore-scripts`. Sin cambiar qué analiza `baseline-scan.yml` (la línea base con Go 1.22).
    Hallazgo al revisar (Opus): el módulo `tools/` hacía fallar el paso osv-scanner de CI (`--recursive`):
    stdlib de `go 1.26.2`, `grpc` 1.83.1, kin-openapi 0.133.0 y GO-2026-6016 en oapi-codegen v2.5.1
    (inyección de código desde `servers[].description` de una spec no confiable; riesgo real bajo aquí, pero
    alcanzable). **Decisión del usuario (2026-10-07): subir oapi-codegen a v2.7.1** (kin-openapi 0.144.0,
    regenerar `gen.go`); solo x/crypto GO-2026-5932, sin versión corregida, queda ignorado con fecha y motivo
    en `tools/osv-scanner.toml`, igual que en `backend/`.
    Evidencia (commit `8f57c76`): `tools/go.mod` (go 1.26.8; oapi-codegen v2.7.1, sqlc v1.31.1, govulncheck v1.8.0,
    gosec v2.29.0) instalado sin `@versión`; `requirements-openapi.txt` con hashes; `npm ci --ignore-scripts`;
    Dependabot para `/tools`; `make scan-deps` desde `tools/`. `gen.go` regenerado (runtime v1.4.0): los campos
    opcionales anulables se omiten en vez de enviarse como `null`; ambos clientes ya usan `?? null` o veracidad.
    `go test ./...` con Postgres real: 27 paquetes ok; `./internal/api` 160 PASS, 0 SKIP; `golangci-lint` 0;
    osv-scanner v2.3.5 sobre `./tools` exit 0; actionlint 0. Hallazgo aparte (sin id): osv-scanner local v2.3.5
    marca 21 avisos de stdlib en `backend/go.mod` (`go 1.26.0`) que el v2.6.0 de CI no marca; pendiente de decidir.
    Revisión nativa del rango `3d0de78..8f57c76` (alto, 20 archivos, 4 lentes): **aprobada** y acusada
    (`review-1a76f102a78bff85`), 10 avisos no bloqueantes: el de parámetros requeridos vacíos en `/oauth/authorize`
    lo cubre `ValidateAuthorizeInput` (`oauth.go:180-187`); `omitempty` verificado en los clientes; `AGENTS.md:49`
    corregido; el resto (simetría de `GOTOOLCHAIN` en `ci.yml`, `make scan-deps` sin `|| true` al instalar) queda
    como sugerencia.
  - [x] D6 — Complejidad fuera de alcance de C1 (`gocognit -over 15`): `bootstrap.Ensure`, `rolegrid.Update`,
    `refresh.Refresh`, `employee.CreateEmployee`, `mfa.Issue`, `mfa.Resend`, `notify.Render`,
    `passwordreset.Confirm`, `invitation.Accept`, `testdb.New`, `oauth.Exchange` e `invitationresend.Resend`
    (`gen.go` es generado: fuera). Mismo método que C1, un commit por función.
    Ruta: delegado (Sonnet, tres lotes de cuatro; cada lote commitea por función), revisado por Opus (diffs de
    `refresh`, `bootstrap`, `passwordreset` y `oauth` leídos completos). Evidencia por lote: `gofmt`, `go vet`,
    pruebas con Postgres real 0 omitidas, `golangci-lint` 0 (salvo un `errcheck` preexistente en `testdb.go:76`,
    ya presente en `main`); suite completa 27 paquetes ok (28 con `-tags integration`).
    | Función | Commit | `gocognit` antes → después |
    |---|---|---|
    | `bootstrap.Ensure` | `e4f4c72` | 46 → 5 |
    | `refresh.Refresh` | `dd90fe6` | 31 → 4 |
    | `rolegrid.Update` | `66298cd` | 31 → 4 |
    | `employee.CreateEmployee` | `a8dd48f` | 30 → 4 |
    | `mfa.Issue` | `01b4772` | 23 → 5 |
    | `mfa.Resend` | `cb299e3` | 21 → 3 |
    | `notify.Render` | `07d7c91` | 20 → 5 |
    | `passwordreset.Confirm` | `a1c9701` | 19 → 5 |
    | `invitation.Accept` | `207b66c` | 17 → 3 |
    | `testdb.New` | `5558b54` | 17 → 6 |
    | `oauth.Exchange` | `11fae1e` | 16 → 4 |
    | `invitationresend.Resend` | `a411526` | 16 → 4 |
    `gocognit -over 15` sobre `internal/` y `cmd/` (sin pruebas ni `gen.go`): vacío. Revisiones nativas (alto, 4
    lentes): lote A **aprobado** (`review-763db682c5cd9269`, 2 sugerencias), lote B **aprobado**
    (`review-5bc1a743cfbc8ddb`, 1 sugerencia), lote C **aprobado** (`review-c3cce3fa9e198d96`, 1 sugerencia en
    `testdb.go:88-104`). Todas acusadas.
  - Cierre de D (2026-10-07): `make up` con las imágenes de la rama y `make e2e`: 52/52 en verde. Entrega en un PR
    nuevo desde `chore/hallazgos-y-sonar` (el PR #20 se fusionó antes, `3d0de78`, por decisión del usuario).

## Deuda menor (no bloquea la entrega; decidir cuál se atiende)

- **SonarCloud en `main` (2026-10-06): 125 code smells.** Principales: S3776 (18, cubiertos en parte por
  C1), S6819 roles ARIA (15, justificados en P9e), S4666 CSS duplicado (11), S8196 nombres de interfaz
  (11, convención hexagonal), S108 en código generado (6).
- **SonarCloud: 20 vulnerabilidades.**
  - 10 «bloqueantes» `secrets:S6698`/`S6736` en `security/evidence/gitleaks-baseline.json`: credenciales
    sembradas de la línea base vulnerable, conservadas como evidencia del «antes» de VULN-001; el usuario
    confirmó (2026-10-06) que ninguna es real ni se reutilizó. Acción: marcarlas en SonarCloud como
    aceptadas con justificación y sumarlas a «Alertas abiertas conocidas» del `README.md`.
  - 1 `secrets:S2068` en `.gitleaks.toml:8` (ejemplo de la propia regla): mismo tratamiento.
  - 9 de cadena de suministro en workflows (reales): `pip`/`npm` sin archivo de bloqueo (S8545, S8543),
    sin `--ignore-scripts` (S6505, `baseline-scan.yml:79`) y sin `--only-binary :all:` (S8541,
    `ci.yml:49`). Acción: fijar versiones con hashes o archivos de bloqueo.
- **Sugerencias menores de revisiones nativas** anotadas en `odd/tasks/pulido-frontend.md` (por ejemplo la
  prueba del enlace para saltar al contenido, que jsdom no puede simular).
- **AM-019:** hecho en P9e (alerta y runbook; amenaza mitigada). Nada pendiente.
- **Falco:** fuera de alcance, documentado en el informe como trabajo futuro.

## Siguiente paso

Merge del PR #20 (usuario) y Q1 (credenciales de Docker Hub); luego C3 con `v0.9.0` de prueba y, en vivo, C2
con `v1.0.0`. Pendientes fuera de C1-C3: grabar el video (Q3) y decidir los hallazgos sueltos sin tarea
(regla de gitleaks con `\s*`, Trivy 0.56.2 local frente a CI, estilo inline bloqueado por la CSP en
`/invitations/accept`, complejidad fuera de alcance de C1).
