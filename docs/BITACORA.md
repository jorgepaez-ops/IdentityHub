# Bitácora del proyecto

Registro de avance, decisiones tomadas sobre la marcha y hallazgos. Se añade una
entrada por sesión de trabajo, la más reciente arriba.

Esta bitácora es también **evidencia de proceso**: junto con el historial de git
y las fichas de `security/findings/`, documenta cómo se llegó al resultado y no
solo cuál fue.

**Convención:** cada entrada cierra con *Estado* y *Siguiente paso*, para que
retomar el trabajo no exija reconstruir el contexto desde cero.

---

## 2026-09-05 · Semana 1 — Specs, esqueleto y línea base vulnerable

### Hecho

**Specs como fuente de verdad (22 archivos).**
31 requisitos (19 RF + 12 RNF), cada uno con criterio de aceptación verificable.
Contrato OpenAPI 3.0.3 con 22 operaciones, cada una declarando su
`x-requirement`. Contrato AsyncAPI con 5 eventos y la topología de RabbitMQ.
Modelo de amenazas STRIDE con 22 amenazas, donde cada mitigación nombra el gate
del pipeline que impide que se pierda. 15 escenarios Gherkin. 8 ADR.

La matriz de trazabilidad **no se escribe a mano**: `scripts/traceability.py` la
deriva del OpenAPI, de las etiquetas de los `.feature` y de los nombres de las
pruebas. Estado actual, sin maquillaje: 0 completos, 17 parciales, 14 sin
cubrir.

**Aplicación.**
Seis contenedores operativos: `db`, `broker`, `mailpit`, `api`, `worker`, `web`.
Esquema de base de datos con los invariantes aplicados por PostgreSQL, no por
disciplina del código.

**Pipeline.**
`ci.yml` con 8 gates bloqueantes · `baseline-scan.yml` en modo informe sobre el
tag vulnerable · `scheduled-scan.yml` semanal que abre incidencia ante CVE
nuevos en código que no ha cambiado.

### Verificado contra el sistema en ejecución

No son afirmaciones: se comprobaron una a una.

| Comprobación | Resultado |
|---|---|
| Recorrido completo navegador → Nginx → API → PostgreSQL y RabbitMQ | correcto |
| Publicar evento → worker → SMTP → Mailpit | correo entregado |
| Republicar el mismo `eventId` | descartado, sigue habiendo 1 correo |
| `INSERT` de un hash MD5 en `users` | rechazado por `CHECK` |
| `UPDATE` y `DELETE` sobre `audit_log` | rechazados por disparador |
| Segunda sesión activa en la misma familia | rechazada por índice único |
| Inyección SQL en el endpoint sembrado | llega literal a la consulta |
| Cabeceras de seguridad en Nginx | ausentes, como se esperaba |
| Usuario de los tres contenedores | `uid=0` |

### Decisiones tomadas sobre la marcha

**Las vulnerabilidades se siembran en el esqueleto, no en el núcleo de auth.**
El plan las ponía en la primera pasada de la semana 2 para remediarlas
reescribiendo en la semana 3. Escribir el IdP dos veces no cabe en un mes.
Concentradas en el esqueleto disparan exactamente los mismos gates, y el núcleo
se escribe bien desde el principio. *Motivo: coste, no criterio de seguridad.*

**OpenAPI 3.0.3 en lugar de 3.1** — `kin-openapi`, sobre el que se apoya
`oapi-codegen`, tiene soporte parcial de 3.1. Ver `adr/0003`.

**El registro de auditoría se protege con disparador, no solo con permisos.**
Un disparador se aplica a todo el mundo, incluido quien se conecte con
credenciales de más. La revocación de `UPDATE`/`DELETE` al rol `identity_app` se
añade en la semana 2, encima y no en lugar del disparador.

**Se eliminó el repositorio git de `~/Documents/Proyectos`.** Tenía cero commits
y englobaba todos los proyectos del usuario; GitHub Actions exige que los
workflows estén en la raíz del repositorio. Movido a `.git.bak-20260905-vacio`
(reversible) tras verificar que no tenía commits, ramas, stash, remotos ni
reflog.

### Hallazgos

Sembrados a propósito, funcionando como se esperaba: VULN-001 (credenciales
incrustadas), VULN-002 (MD5), VULN-005 (SQL concatenado), y los defectos de
Dockerfile y Nginx.

**Los tres que no habíamos previsto —y que justifican tener el pipeline:**

- **VULN-020** · `middleware.RealIP` de chi acepta `X-Forwarded-For` sin
  validar. Rompe a la vez el bloqueo por fuerza bruta (el contador va por IP) y
  la credibilidad del registro de auditoría. Es la frontera de confianza T2 de
  nuestro propio modelo de amenazas, mal implementada.
- **VULN-022** · Inyección SQL **en el driver pgx**, por debajo de nuestro
  código. Desmonta el "uso consultas parametrizadas, luego estoy a salvo".
- **VULN-023** · Gitleaks con reglas por defecto detectaba **2 de 12** secretos:
  identifica por formato (AWS, Slack), no por contexto (`POSTGRES_PASSWORD`).
  Con `.gitleaks.toml` propio pasó a 12 sin falsos positivos. Sin la línea base
  deliberada, ese job habría estado en verde y lo habríamos leído como "no hay
  secretos".

### Estado

- Repositorio: `main` + tag `v0.0.0-vuln-baseline`. Árbol limpio.
- `ci.yml` **en rojo a propósito**: los gates encuentran lo sembrado.
- El pipeline aún **no se ha ejecutado en GitHub Actions** (falta subir el repo).

### Entregables publicados

- **Informe de seguridad de la línea base** —
  https://claude.ai/code/artifact/c652b965-baba-49f8-a283-c57ca7fae1c4
  Fuente versionada en `docs/security-report.html`; se republica desde ese mismo
  archivo, de modo que el documento y el repositorio no pueden divergir.
  La columna `main` está vacía a propósito hasta que exista la medición.

### Siguiente paso

1. Crear el repositorio en GitHub y subirlo. Confirmar que los tres workflows
   se ejecutan y que `baseline-scan.yml` produce el inventario completo.
2. Empezar el núcleo del IdP (semana 2): `sqlc`, Argon2id, JWT Ed25519 con
   JWKS, rotación de refresh con detección de reuso, RBAC y audit log.
3. Generar `backend/internal/api/gen.go` con `oapi-codegen` y activar el gate
   real de deriva de specs.
