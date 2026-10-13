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
- [x] **Avisos nuevos que bloqueaban el release (2026-10-12, vistos en el CI del PR #30 de C3).** Sin id de
  VULN por decisión del usuario (cierre del proyecto). Publicados después del último CI verde de `main`
  (2026-10-07): (a) biblioteca estándar de Go en go1.26.8, GO-2026-6603 a GO-2026-6617 en `net/http`,
  `net/textproto` y `crypto/tls` (govulncheck, job 5) y CVE-2026-78667, CVE-2026-78669 y CVE-2026-97031
  (Trivy, imágenes `api`/`worker`), corregidos en 1.26.9; (b) `tiff` 4.7.1-r0 en `nginx-unprivileged`,
  CVE-2026-4775 HIGH (Trivy, imagen `web`), corregido en 4.7.2-r0. Remediación en la rama
  `fix/go-1.26.9-y-nginx`: `golang:1.26.9-bookworm` y `nginx-unprivileged:stable-alpine` por digest
  nuevo; `GO_VERSION` de `ci.yml` y `go-version` de `scheduled-scan.yml` fijados a `1.26.9` (con `1.26`,
  setup-go reutilizaba el go1.26.8 en caché del runner). Ruta: inline (Claude, sesión en vivo).
  Verificación local: build de `api` y `web`, `go version` de la base = go1.26.9, y Trivy 0.70.0 con el
  mismo gate de CI (HIGH/CRITICAL corregibles, `security/.trivyignore`) con salida 0 en ambas.
  (c) Visto en el CI del PR #31: osv-scanner marcó los mismos avisos (GO-2026-6603, 6610, 6611, 6612 y 6617)
  en `golang.org/x/net` 0.59.0 de `tools/go.mod` (dependencia indirecta de las herramientas de desarrollo);
  subido a 0.60.0. Verificación local: las cuatro herramientas de `tools/` compilan y osv-scanner v2.6.0
  `--recursive --all-vulns` sobre `tools/` y `backend/` da "No issues found".
- [x] **C3 — Docker Hub (T17 de `idp-semana-4.md`).** Workflow de release en el tag `vX.Y.Z`: imágenes `api`,
  `worker` y `web` con `vX.Y.Z` y `latest`, SBOM con Syft y firma con Cosign (ADR 0011), environment
  protegido `dockerhub`. Requiere Q1: namespace y token de Docker Hub del usuario. Se prueba con el tag
  `v0.9.0` (y se borra o se deja marcado como prueba) antes de `v1.0.0`.
  Ruta: inline (Claude, sesión en vivo con el equipo, 2026-10-12; un solo archivo nuevo). Avance:
  - [x] Q1 resuelta: namespace `jorgeivp` (variable de repo `DOCKERHUB_NAMESPACE`); environment
    `dockerhub` restringido a tags `v*`; secreto `DOCKERHUB_TOKEN` (PAT Read & Write) cargado por el
    usuario en ese environment.
  - [x] `.github/workflows/release.yml` en la rama `feat/c3-docker-hub-release`: solo tags
    `v[0-9]+.[0-9]+.[0-9]+` (excluye `v0.0.0-vuln-baseline` y `v0.1.0-hardened`); matriz `api`/`worker`/`web`
    con los mismos targets que CI; gate de Trivy idéntico al de CI antes del push; etiquetas `vX.Y.Z` y
    `latest` en `docker.io/jorgeivp/identity-hub-<nombre>`; SBOM CycloneDX con Syft; firma y atestación
    con Cosign keyless sobre el digest. Verificación: actionlint v1.7.7 sin errores; zizmor 1.14.2
    `--offline` sin hallazgos (se corrigió `persist-credentials: false`). Revisión nativa (4 lentes) aprobada
    sobre `5cca7f9`, sin bloqueantes.
  - [x] Corrección de los avisos de la revisión (decisión del usuario, 2026-10-12: un release publica las tres
    imágenes o ninguna; publicar una sola solo tendría sentido si cada contenedor se versionara aparte).
    El workflow pasa a dos fases: `gate` construye y escanea las tres y guarda cada imagen como artefacto;
    `publish` (`needs: gate`, único job con el environment `dockerhub`) carga la misma imagen escaneada, sin
    reconstruirla, y la sube, firma y atesta. Resuelve "la imagen publicada no es la escaneada" y "release a
    medias si falla el gate de una". Riesgo residual (no aceptado por el usuario el 2026-10-12, ver F8): un fallo de red durante `publish` aún puede dejar
    una imagen sin subir (se reintenta el job). Verificación: actionlint v1.7.7 con shellcheck 0.10.0 y zizmor
    1.14.2 `--offline`, sin hallazgos.
  - [x] PR y merge a `main`; tag de prueba `v0.9.0` sobre el merge; run verde; imágenes, firma y SBOM
    visibles en Docker Hub (`cosign verify`). Evidencia (2026-10-12): PR #30 fusionado (`0ea0ea3`, tras el PR
    #31 con los avisos de arriba); tag anotado `v0.9.0` sobre `0ea0ea3`; run Release
    [38319904991](https://github.com/jorgepaez-ops/IdentityHub/actions/runs/38319904991) en verde (6 jobs, 1m 38s).
    Digests: `api` `sha256:52f2988c…86a3`, `worker` `sha256:f9b5443d…d03c`, `web` `sha256:fcca1f1c…6005`
    (completos en el resumen del run). `cosign verify` y `cosign verify-attestation --type cyclonedx` pasan en
    las tres con **Cosign v3.0.6**; con Cosign v2.5.3 dan "no signatures found", porque el instalador trae v3 y
    v3 guarda firma y atestación como OCI referrers (formato bundle), no como etiquetas `.sig`/`.att` (por eso
    la pestaña Tags de Docker Hub no las muestra). Capturas: informe Desktop, sección "5. C3 — Release v0.9.0
    en Docker Hub" (7 capturas; vista pública de Docker Hub, sin sesión). Docker Hub queda cerrado (usuario).
- [x] **E — Cierre documental (decisión del usuario, 2026-10-12: «solucionemos las bajas»).** El informe
  técnico se escribió antes de T21a, D1 y C3 y declara pendiente lo que ya está hecho. Hoy, en la rama
  `docs/cierre-informe`:
  - [x] E1 `docs/informe/informe-tecnico.md`: Docker Hub hecho (secciones de alcance, amenazas abiertas
    AM-008/AM-009/AM-022, fase 5, limitaciones, trabajo futuro, anexos A y B; nota de Cosign v3); quitar las
    menciones al guion como entregable (el documento se conserva y se puede consultar, pero el informe no lo
    anuncia); quitar «Q4 abierta / sin validación del docente» (el equipo eligió el lenguaje con la potestad
    que da el enunciado; no requiere autorización); sacar de trabajo futuro y de desafíos la codificación de
    los correos (T21a) y la regla de Gitleaks (D1), ya hechas; mencionar los avisos del 2026-10-12 resueltos.
  - [x] E2 Revisión de T21 contra el enunciado: bitácora, matriz de trazabilidad, informe de seguridad
    final; tag de versión y PR quedan para C2. Revisión hecha (explorador Sonnet, 2026-10-12, verificada por
    Claude en `README.md:20`, `docs/security-report.html:545` y `scripts/traceability.py:5-7`): el enunciado no
    nombra bitácora, matriz ni PR como requisitos (vienen de T21); sí exige badges de estado, cobertura y versión
    (§4.1), imágenes en Docker Hub con semver (§3.2) y tabla de hallazgos (§6). Texto desactualizado tras C3:
    - [x] `docs/BITACORA.md`: entrada del 2026-10-12 (C3, `v0.9.0`, Cosign v3, PR #30 y #31, F para mañana) y
      quitar "Docker Hub pendiente" (líneas 17, 35-36, 44-53).
    - [x] `docs/security-report.html` (versión 4, 2026-10-06): nota de avisos posteriores sin id (PR #31),
      procedencia de imágenes (firma Cosign y SBOM atestado, `v0.9.0`) y etiquetas.
    - [x] Docker Hub ya no pendiente en `README.md:20`, `docs/README.md:43,77`,
      `docs/manuales/despliegue-y-operacion.md:3,63,98,113`, `docs/manuales/arquitectura.md:64` y estado de
      `specs/adr/0011-docker-hub-como-registry.md`; `docs/video/guion.md` sin la escena condicional de T17.
    - [x] `scripts/traceability.py:5-7`: la razón de RF-018/RF-019 dice "semana 4"; pasar a la planificación del
      2026-10-13 y regenerar `specs/07-traceability.md` (no se edita a mano).
    - [x] `ANALISIS-REQUISITOS.md` (instantánea del 2026-09-26): aviso de que es histórica.
    - [x] Modelo de amenazas: `docs/threat-model/identity-hub.json` y su `README.md` aún dicen *Open* para AM-008,
      AM-009 y AM-022; el informe ya los da por mitigados con C3.
    - Evidencia E2 (2026-10-12): ruta delegada a Sonnet, revisión de Claude. Bitácora con entrada del 2026-10-12;
      `security-report.html` revisión 2026-10-12 (avisos del PR #31 sin id, procedencia de imágenes, `v0.9.0`);
      README, `docs/README.md`, manuales, ADR 0011 (estado implementado) y guion sin Docker Hub pendiente; modelo
      de amenazas con AM-008/009/022 *Mitigated* y `hasOpenThreats` coherente; `ANALISIS-REQUISITOS.md` con aviso
      histórico (está en `.git/info/exclude`: cambio solo local). Matriz: Claude cambió además las cadenas fijas
      "diferido (semana 4)" del generador por "diferido (2026-10-13)" (prueba primero: RED con 1 fallo, GREEN con
      `scripts/traceability_test.py` OK) y regeneró; `python3 scripts/traceability.py --check`: al día;
      `python3 -m json.tool` sobre el modelo: válido.
    - Para mañana (F6, media): RNF-001, 002, 006, 008 y 010 salen "sin cubrir" en la matriz porque el
      generador solo cuenta pruebas Go, Gherkin y E2E; enlazarlos con su evidencia de CI (jobs, release).
  - [x] E3 Badge de cobertura del README sin servicio externo.
  - Evidencia E1/E3 (2026-10-12): ruta delegada a Sonnet (Codex falló al arrancar: su modelo por defecto
    `gpt-5.6-sol` no se admite con la cuenta de ChatGPT; job `buk7uj44r`, sin cambios). Informe: alcance, tabla
    STRIDE (AM-008/009/022 a *Mitigated*), amenazas abiertas, fase 5 (publicación y nota de Cosign v3), fase 6
    (avisos del PR #31), desafíos, limitaciones, trabajo futuro y anexos A y B; sin Q4 ni guion. README: badge
    estático del gate de cobertura (≥ 70 %, job 6b). Revisión de Claude sobre el diff; grep de Q4, T17, T20, T21,
    guion y `cd.yml` sin coincidencias en el informe.
- [x] **F — Mejoras de complejidad media y alta (decisión del usuario, 2026-10-12: se anotan hoy y se hacen
  mañana, 2026-10-13; `v1.0.0` sale después).** Estimación de Claude (2026-10-12):
  - [x] F1 RF-019 exportación del audit log en CSV/JSON (media-baja, ~300 líneas: endpoint spec-first sobre el
    listado existente, botón en la UI, pruebas).
    Evidencia (2026-10-12): rama `feat/f1-rf019-audit-export` (`bacd7ca`, `62a0a4e`), ruta delegada a Sonnet.
    `GET /api/v1/admin/audit-log/export?format=csv|json`, tope 10 000 filas, CSV protegido contra fórmulas,
    exportación auditada; pruebas RED→GREEN con Postgres real, cobertura 80,6 %, golangci-lint 0, vitest 257;
    E2E 54/54 con stack real; revisión nativa aprobada. Sin push.
  - [x] F2 SMTP autenticado con STARTTLS en el worker y hosts configurables (media, ~400 líneas): hoy
    `smtp.SendMail` sin TLS (`backend/cmd/worker/main.go:204`) y cliente OAuth/hosts fijos
    (`backend/internal/config/config.go:66-68`); un `.sh` genera CA y certificado locales (como en el ejemplo
    de la app de mascotas) y Mailpit exige STARTTLS y autenticación. Hace aplicable la arquitectura de AWS (SES).
    Evidencia (2026-10-12): rama `feat/f2-smtp-starttls` (`2561a74`, `b10771c`, `eaf7cc1`, `d9fd3d2`), ruta delegada a
    Sonnet. STARTTLS obligatorio y AUTH solo tras TLS, `scripts/gen-smtp-certs.sh` + `make certs`, Mailpit con
    STARTTLS y credenciales, SES en Terraform, cliente OAuth por variables. E2E 52/52; envío en claro rechazado
    (530). Revisión aprobada; Claude corrigió las comillas del archivo de credenciales. Sin push. Requiere
    `SMTP_PASSWORD` en `.env` viejos. Siguen fijos: `server_name` de Nginx, constantes de Contabilidad y la fila
    del cliente en la BD (documentado).
  - [x] F3 Falco (media, ~300 líneas): servicio en el perfil de observabilidad (modern eBPF), 2-3 reglas propias,
    falcosidekick hacia Loki y una alerta. Riesgo a verificar primero: soporte en Docker Desktop de macOS.
    Evidencia (2026-10-12): rama `feat/f3-falco` (`7a86ae9`, `0697dfe`), ruta delegada a Sonnet. Falco 0.45.0 por digest
    con modern eBPF en Docker Desktop sin `privileged`; 4 reglas propias (AM-020), alertas a Loki vía Alloy, alerta
    y paneles en Grafana, runbook y `make falco-test`. Revisión aprobada; Claude cerró la evasión de la excepción
    de `wget` y la detección de `connect` no bloqueante, verificado con Falco. Sin push.
  - [x] F4 RF-018 claves de servicio (media-alta, ~800 líneas): migración, endpoints, hash de la clave,
    middleware de autenticación por clave, UI de administración, pruebas y E2E.
    Evidencia (2026-10-12/13): PR #39 fusionado (`f3a5c38`), ruta delegada a Sonnet. Clave `ihk_<publicId>_<secreto>`
    (256 bits, solo SHA-256 guardado, comparación en tiempo constante con hash señuelo), alcance `audit-log:read`
    solo en `GET /api/v1/admin/audit-log`, mismo 401 para toda causa, ADR 0014, AM-023, 5 escenarios con E2E.
    Correcciones tras la revisión nativa: solo se auditan rechazos ligados a una clave existente (los anónimos van
    al log, sin datos de la cabecera: CodeQL go/log-injection) y ningún fallo de auditoría se descarta en silencio.
    Pruebas RED→GREEN, Postgres real, golangci-lint 0, vitest 262, E2E 57/57.
  - [x] F5 Decidido por el usuario (2026-10-12): **queda como trabajo futuro** (exposición del proyecto en 5 días;
    no se arriesga el núcleo de autenticación ya probado). Documentado en `docs/manuales/integracion-terceros.md`
    §9.1. Antes: integración de terceros (clientes OAuth en BD, OpenID Connect, refresh tokens
    por cliente, introspección, revocación) y outbox transaccional (alta, >2 000 líneas). Recomendación de
    Claude: dejarlo como trabajo futuro justificado (toca el núcleo de autenticación ya probado).
  - [x] F6 Matriz: RNF-001, 002, 006, 008 y 010 salen "sin cubrir" porque el generador solo cuenta pruebas Go,
    Gherkin y E2E; enlazarlos con su evidencia de CI y del release (origen: revisión de E2).
    Evidencia (2026-10-12): rama `feat/f6-rnf-evidence` (`86c5d85`, `313dc82`). Columna "Controles CI" desde
    `specs/07-controls.yaml`, verificada contra workflows/compose/Makefile (sin comentarios); de 5 RNF sin cubrir a 1
    (RNF-010, sin prueba de latencia). Revisión aprobada. Sin push.
  - [x] F7 Release: `latest` solo después de firmar y atestar (decisión del usuario, 2026-10-12: «latest no puede
    subir antes de la firma»). Hoy `publish` sube `vX.Y.Z` y `latest` y después firma; pasa a subir solo `vX.Y.Z`,
    firmar y atestar el digest, y recién entonces mover `latest` a ese mismo digest. Se prueba con un tag de
    validación (`v0.9.1`) antes de `v1.0.0`.
    Evidencia (2026-10-12): rama `fix/f7-f8-release-latest-and-retries` (`4784bab`, `63a1f98`), ruta inline (Claude).
    `publish` sube solo `vX.Y.Z`, firma y atesta, y luego mueve `latest` si es el tag más alto, verificando el
    digest. actionlint/shellcheck y zizmor sin hallazgos; revisión aprobada. Pendiente: probar con `v0.9.1`
    (requiere push, mañana).
  - [x] F8 Release: reintento ante fallos de red en `publish` (push con reintentos acotados) y retención de los
    artefactos de imagen suficiente para repetir solo el job fallido (hoy 1 día; el usuario no acepta el riesgo).
    Evidencia (2026-10-12): mismo commit que F7. Reintentos (3) en push, cosign y consulta de tags; artefactos 7 días
    (repositorio público: sin costo). Revisión aprobada.
  - [x] F9 VULN-028 (GO-2026-5932): verificado el 2026-10-12 que **no tiene versión corregida ni la tendrá**
    (osv.dev: afecta a `golang.org/x/crypto` desde la versión 0, sin `fixed`; el paquete `openpgp` está abandonado
    por diseño). `go list -deps` no encuentra `x/crypto/openpgp` ni en `backend/` ni en las herramientas de
    `tools/`. Propuesta: reclasificarlo de «riesgo aceptado con fecha» a «no afectado» (código vulnerable no
    presente), con esa evidencia, en la ficha, el informe, la matriz de hallazgos y `osv-scanner.toml`.
    Decisión del usuario (2026-10-12): reclasificar. Hecho: `go -C backend list -deps ./... | grep x/crypto/openpgp` y
    `go -C tools list -deps` de oapi-codegen, gosec, sqlc y govulncheck dan 0 resultados; backend solo usa `argon2` y
    `blake2b`. Cambios: ambos `osv-scanner.toml` (exclusión permanente, sin `ignoreUntil`), ficha VULN-028 (estado
    `no afectado`, leyenda en `security/findings/README.md`), `evidencia.json`, README, informe técnico, informe HTML,
    threat model, manual de seguridad y BITACORA. Commit: `fix(security): reclassify VULN-028 as not affected` (hash en el
    historial de la rama).
  - [x] F10 Badge de cobertura con el valor medido: hoy es estático («gate ≥ 70 %», E3) y siempre se ve verde
    aunque la cobertura real cambie (aviso de dos lentes de la revisión nativa del 2026-10-12). Publicar el
    porcentaje que mide el job 6b sin servicio externo.
    Evidencia (2026-10-12/13): PR #34 fusionado. Job `6c` (solo push a `main`, único con `contents: write`) publica
    `coverage.json` en la rama huérfana `badges`; distingue rama inexistente de error de red y no pone `main` en rojo.
    Verificado tras el merge: `badges/coverage.json` = 78.4 % (yellowgreen).
- [x] **Hallazgos durante los merges (2026-10-12/13).** Sin id de VULN (decisión del usuario).
  - CVE-2026-85091 (`zlib` 1.3.2-r0, MEDIUM) en la imagen `web`: capa temporal `apk upgrade --no-cache zlib`
    hasta que la base de nginx lo incluya (PR #40). Trivy MEDIUM+ sin hallazgos, uid 101.
  - Bug de F2 detectado por el hook de revisión con IA al integrar `main` en F6: si fallaba el `QUIT` después de
    `250` a DATA, el worker reencolaba el evento y el correo se enviaba dos veces. `QUIT` ahora es de mejor
    esfuerzo; prueba RED→GREEN (PR #41).
  - Issue #29 (escaneo semanal del 2026-10-12 con go1.26.8) cerrado con evidencia: escaneo manual sobre `main`
    (run 38337722846) con go1.26.9, govulncheck sin vulnerabilidades.
  - Fallas de CI resueltas en los PR: Checkov CKV_AWS_273/40 justificados (SES SMTP exige usuario IAM), falso
    positivo de Sonar S7552 en `gen-smtp-certs.sh`, prueba de frontend de F1 no aislada (FileReader colado).
  - Merges: PR #32 a #41 en `main`. Ramas restantes: solo `main` y `badges`; worktrees y volúmenes de prueba
    borrados. Matriz: 33 requisitos, 25 completos, 7 parciales, 1 sin cubrir (RNF-010), 0 diferidos.
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
  - Corrección de CI tras el primer run del PR #21 (`c6b86d2`): `tools/doc.go` (el análisis de llamadas de osv-scanner
    necesitaba un paquete en `tools/`) y `sonar.cpd.exclusions` para `gen.go` y la salida de sqlc (la duplicación
    del código generado tumbaba la puerta de calidad). PR #21 fusionado (`0941fdc`); en `main`, SonarCloud con 0
    vulnerabilidades abiertas (antes 20) y puerta aprobada.
  - Cierre de D (2026-10-07): `make up` con las imágenes de la rama y `make e2e`: 52/52 en verde. Entrega en un PR
    nuevo desde `chore/hallazgos-y-sonar` (el PR #20 se fusionó antes, `3d0de78`, por decisión del usuario).

## Deuda menor (no bloquea la entrega)

- **SonarCloud en `main`: code smells** (125 al 2026-10-06; parte de S3776 cubierta por C1). No bloquean.
- Las 20 vulnerabilidades de SonarCloud y la cadena de suministro de los workflows quedaron resueltas en D
  (D4, D5; PR #21). Falco pasó a F3.
- Sugerencias menores de revisiones nativas en `odd/tasks/pulido-frontend.md`.

## Siguiente paso

Mañana (2026-10-13), en este orden:
0. **Filtro de documentación en `ci.yml` (prioridad, pedido explícito del usuario: "esto no puede pasar").**
   El 2026-10-12 el PR #42 (solo `odd/tasks/cierre-entrega.md`) corrió el pipeline completo (E2E, DAST,
   integración, imágenes) porque `ci.yml` usa `pull_request:` y `push: [main]` sin filtro de rutas. Causa
   adicional: se abrió el PR al cierre del día en vez de dejar la rama local. Diseño:
   - No usar `paths-ignore` a nivel de workflow: el ruleset "Protect main" exige 17 checks y quedarían en
     "Expected — waiting", bloqueando el merge.
   - Job inicial `changes` que detecte si el diff toca algo fuera de `**.md`, `odd/**` y `docs/**`; los jobs
     pesados llevan `if: needs.changes.outputs.code == 'true'` (un job *skipped* cuenta como aprobado).
   - `1 · Deriva entre specs y código` y `3 · Secretos en el historial` corren siempre.
   - Verificar que `CodeQL`, `Trivy` y `Semgrep OSS` (checks de code scanning por SARIF) no queden pendientes
     cuando su job se salta; probarlo con un PR documental real antes de dar la tarea por cerrada.
   - Regla de trabajo: notas de fin de día van en rama local; sin push ni PR hasta que el usuario lo pida.
   - **Implementado en el PR #43 (merge `e209138`, 2026-10-13).** Ruta: delegada (Sonnet) + correcciones
     inline. Diferencias con el diseño: los jobs gateados usan `!cancelled() && code != 'false'` (si el job 0
     falla se corre todo, porque *skipped* cuenta como aprobado); el job `image` arranca siempre y gatea cada
     paso (los nombres expandidos de la matriz son obligatorios); `git diff --no-renames`; `.github/`, `specs/`
     y `scripts/` cuentan como código aunque sean `.md`; `sast`, `semgrep`, `iac-scan` e `iac-checkov` también
     corren siempre (alimentan los checks SARIF). Verificación: actionlint limpio, zizmor sin hallazgos nuevos,
     clasificador probado en local, CI completo verde en #43, revisión nativa aprobada (3 pasadas).
   - **Cerrada (2026-10-13).** PR #42 (solo documentación): jobs pesados *skipped*, las tres imágenes *success*
     con pasos omitidos, CodeQL/Semgrep OSS/Checkov/SonarCloud *success*, `Trivy` *neutral* (faltan los SARIF de
     imágenes; no bloquea), PR `CLEAN`, ~1 min contra ~8 min antes. Push a `main` del merge (`32e8359`): el job 0
     decidió `code=false (solo documentación)` y el run terminó *success* con el mismo patrón.
   - Seguimiento (advertencias no bloqueantes de la revisión): mover el clasificador a `scripts/` con prueba;
     la guarda repetida en cada paso de `image` depende de que cada paso nuevo la incluya.
1. Validar F7 con un tag `v0.9.1` sobre `main`: `latest` se mueve recién después de firmar y atestar; `cosign verify`
   (Cosign v3) de las tres imágenes; prompt de capturas para Claude Desktop.
   - **Cerrada (2026-10-13).** Tag `v0.9.1` sobre `32e8359`, run de `release.yml` 38405885275 *success*. Por imagen,
     el paso "Mover latest" empieza después de terminar "Firmar y atestar" (api 13:34:25→13:34:28, worker
     13:34:32→13:34:35, web 13:34:32→13:34:36 UTC). `latest` = `v0.9.1` en Docker Hub (`jorgeivp/identity-hub-*`).
     Verificación local con Cosign v3.1.3 (`cosign verify` + `verify-attestation --type cyclonedx`, identidad
     `^https://github.com/jorgepaez-ops/IdentityHub/`, issuer de Actions): OK en las tres. Digests:
     api `sha256:147b1694…c052`, worker `sha256:14152883…bd71`, web `sha256:f6c911dc…4e56`.
     Capturas: 16 en el informe formal APA, sección "F7 · Release firmado (v0.9.1)" (pp. 19-21, Anexo B B110-B125),
     confirmadas por el usuario el 2026-10-13.
2. C2: `make informe` (incluir en el informe lo hecho en F1-F4, F6-F10 y los hallazgos de arriba), revisión del PDF
   por el usuario, notas del release, tag `v1.0.0` en vivo y `gh release create v1.0.0` con el PDF.
3. Entorno local del usuario: agregar `SMTP_PASSWORD` al `.env` (`openssl rand -hex 24`) y correr `make certs`
   antes de `make up`.
Notas operativas: Codex ya no está disponible (la delegación va a Sonnet); `gh pr merge --delete-branch` también
borra el worktree local de esa rama; Engram rechazó guardados en la sesión del 2026-10-12 (consola nueva).
