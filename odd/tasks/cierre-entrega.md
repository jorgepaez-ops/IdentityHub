# cierre-entrega — Refactorización, versión y publicación antes de la entrega del 2026-10-23

## Objetivo

Dejar el código de producción legible (KISS, una responsabilidad por función), publicar la versión
`vX.Y.Z` con el informe adjunto y cerrar T17 (Docker Hub), en ese orden (decisión del usuario,
2026-10-06).

## Tareas

- [ ] **C1 — Refactorización por complejidad cognitiva (Sonar S3776).** Extraer pasos con nombre propio
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
    `golangci-lint` limpios.
  - [ ] C1c `mfa.go` · [ ] C1d `config.go` · [ ] C1e `rbac.go`
- [ ] **C2 — Tag `vX.Y.Z` y release** con el PDF del informe adjunto (`make informe`; el PDF no se
  versiona). En vivo frente al equipo (T21 de `idp-semana-4.md`).
- [ ] **C3 — Docker Hub (T17 de `idp-semana-4.md`).** Workflow de release en el tag: imágenes con `vX.Y.Z`
  y `latest`. En vivo.

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

C1c, `mfa.go`.
