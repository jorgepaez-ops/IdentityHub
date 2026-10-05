# idp-semana-4 — Entrega final: documentación, IaC de referencia, grilla de roles y publicación

Documento de feature (ODD). Es el único archivo de tareas de esta feature; el espejo en memoria
(`odd/idp-semana-4/tasks`) lo reconcilia Claude. Codex edita este archivo (casillas, `Commit:`,
handoff, preguntas); lee también `AGENTS.md` en la raíz del repo.

## Objetivo

Cerrar la entrega final del **2026-10-23** con todo lo que pide `ENUNCIADO-TRABAJO-FINAL.md`:
documentación y diagramas UML organizados como los pide el profesor, despliegue de producción en
la nube descrito como IaC (ejemplo de referencia, sin presupuesto), una grilla de roles y permisos
configurable desde el Hub (pedida por el profesor en la demo en vivo), una guía de integración para
terceros, las imágenes publicadas en Docker Hub, el informe técnico en PDF y el video.

## Problema y motivo

- Pesos de la nota (`ENUNCIADO-TRABAJO-FINAL.md:184-191`): aplicación 20 %, CI/CD 20 %,
  herramientas de seguridad 10 %, **documentación y UML 20 %**, **publicación en GitHub y Docker
  Hub 20 %**; observabilidad +8 % y video +8 % opcionales. Lo que más falta está en los dos 20 %
  marcados.
- El enunciado pide IaC con Terraform o Ansible (líneas 67, 96, 146, 166) y Checkov (278); hoy no
  hay nada de eso.
- En la demo en vivo el profesor preguntó si se podía crear un rol distinto y configurar sus
  permisos. Hoy los roles son un enum fijo del OpenAPI (D8 de la semana 3: `admin`, `user`,
  `contabilidad.senior`, `contabilidad.analista`) y los permisos están cableados en Contabilidad.
- Falta explicar cómo se integra otra plataforma al Hub (qué expone, qué registra, qué cambia en su
  frontend).
- Faltan: diagramas de despliegue y casos de uso, DFD de Threat Dragon, manuales de arquitectura,
  despliegue, seguridad y usuario, el informe PDF de 8 secciones (líneas 172-181) y el video de
  10-15 minutos (el guion actual dura ~5).

## Referencias

- `ENUNCIADO-TRABAJO-FINAL.md` (requisitos, entregables, rúbrica) y `ANALISIS-REQUISITOS.md`.
- Cierre de la semana 3: `odd/tasks/idp-semana-3.md`, `docs/BITACORA.md`, PR #9 (`6982301`).
- `specs/adr/0011-docker-hub-como-registry.md` (publicación en tag `vX.Y.Z`, Cosign keyless, SBOM
  con Syft).
- `specs/05-security/threat-model.md` (STRIDE con DFD en texto).

## Alcance

**Dentro**, en el orden que pidió el usuario (de más fácil a más difícil, Docker Hub al final):
1. Documentación y UML organizados según el enunciado, más la guía de integración de terceros.
2. IaC de referencia: cómo sería producción en la nube, como ejemplo y no como despliegue real
   obligatorio (sin presupuesto); se valida contra LocalStack donde se pueda, y Checkov en CI.
3. Grilla de roles configurable (decisión del usuario: alcance A).
4. Cierre de entrega: VULN-019, Docker Hub con SBOM y firma, manual de usuario con capturas,
   informe PDF, guion del video y tag de versión.

**Fuera salvo que sobre tiempo:** Falco (único faltante del bonus de observabilidad), RF-018
(claves de servicio) y RF-019 (exportación del audit log); siguen **diferidos** en la matriz.

**Fuera:** despliegue real en una nube de pago.

## Restricciones

- Las mismas de la semana 3 (`odd/tasks/idp-semana-3.md`, sección "Restricciones"): `specs/` manda,
  `gen.go` no se edita a mano, nunca debilitar un gate, nunca secretos reales en el repo.
- **Portabilidad:** quien clone el repo ejecuta `docker compose up -d` y funciona solo con Docker.
  La IaC de referencia y LocalStack no pueden volverse requisito para levantar el proyecto.
- Codex no tiene red ni Docker (`CLAUDE.md`): Terraform contra LocalStack, `docker build`, Docker
  Hub, capturas y E2E los cierra Claude.
- Tamaño orientativo por tarea: ~400 líneas de cambio (heurística, no tope).
- Estrategia de entrega: `ask-on-risk`. Commits de unidad de trabajo en `feat/idp-semana-4`; push,
  PR y merge los decide el usuario (un PR por corte de fase, merge commit).
- Plazo: **2026-10-23**. Las fases van en orden; si el plazo aprieta se recorta desde el bloque
  "fuera salvo que sobre tiempo", nunca la publicación ni el informe.

## Modo TDD resuelto

- TDD estricto para código (fase 3 y scripts): RED, GREEN y REFACTOR observados por tarea con
  `make test-go`, `make test-integration`, `npm run test` y `make e2e`.
- Documentación, diagramas e IaC de referencia: sin RED posible; se verifican con comprobaciones
  estructurales (enlaces, `terraform validate`/`plan`, Checkov, render de diagramas).

## Criterios de aceptación de la feature

1. La documentación sigue la estructura del enunciado (manuales y UML pedidos) y hay una tabla de
   equivalencias entre el layout del enunciado y el del repo.
2. Un admin crea un rol nuevo de una aplicación (por ejemplo `contabilidad.auditor`), le marca
   permisos en una grilla, se lo asigna a un empleado, y Contabilidad lo respeta sin cambiar código.
   Cada control (quién puede editar roles, autoasignación, permisos desconocidos) tiene prueba.
3. La guía de integración explica, paso a paso, cómo una plataforma nueva se registra como cliente
   OAuth, qué endpoints usa, cómo declara sus permisos y qué cambia en su frontend.
4. La IaC de referencia pasa `terraform validate` y Checkov en CI; lo soportado por LocalStack se
   comprueba con `terraform plan`/`apply` local y queda documentado qué no.
5. Las imágenes están en Docker Hub con `vX.Y.Z` y `latest`, SBOM y firma verificable.
6. Informe PDF con las 8 secciones del enunciado y guion de video de 10-15 minutos.
7. `ci.yml` en verde; ningún gate debilitado; matriz de trazabilidad al día.

## Decisiones tomadas (2026-10-05, con el usuario)

- **D1 · Fecha de entrega:** 2026-10-23.
- **D2 · Orden:** de más fácil a más difícil; Docker Hub al final, cuando todo lo demás esté listo.
- **D3 · IaC:** referencia de producción en la nube, como ejemplo y no como despliegue obligatorio
  (sin presupuesto); LocalStack para validar localmente lo que soporte.
- **D4 · Grilla de roles:** alcance A, configurable (no solo lectura): las aplicaciones declaran sus
  permisos y el admin arma roles marcándolos en una grilla.
- **D5 · Integración de terceros:** se documenta (API compartida más ajustes en el frontend del
  tercero); no se construye una segunda aplicación cliente.

- **D6 · LocalStack (2026-10-05):** el usuario tiene licencia de LocalStack de un año por el
  GitHub Student Pack, así que la IaC de referencia puede validarse contra servicios de pago de
  LocalStack (por ejemplo ECS o RDS). Q2 pasa a verificar qué cubre esa licencia en T7. El token
  de licencia es un secreto: va en el entorno local, nunca en el repo ni en CI sin decisión del
  usuario.

- **D7 · RabbitMQ en la nube de referencia (2026-10-05):** servicio ECS Fargate autogestionado
  (como en el compose), no Amazon MQ: LocalStack no soporta Amazon MQ para RabbitMQ y así toda la
  arquitectura se aplica y prueba en local. El informe justifica la desviación respecto de un
  servicio gestionado.

## Preguntas abiertas

- **Q1 · Docker Hub:** cuenta y namespace, repositorios (`api`, `worker`, `web` y ¿`contabilidad`?),
  quién crea el token y el environment protegido de GitHub (ADR 0011), y si el primer tag publicado
  es `v1.0.0`. Se decide antes de T17.
- **Q2 · LocalStack:** la edición gratuita no incluye varios servicios (por ejemplo ECS o RDS);
  falta confirmar qué recursos de la arquitectura de referencia se pueden validar con ella. Se
  investiga en T7.
- **Q3 · Video:** el enunciado lo marca obligatorio (sección 5) y a la vez opcional +8 % (línea
  191); formato y alojamiento. Se decide antes de T20.
- **Q4 · Stack Go:** el enunciado sugiere Python/Node; ¿hay validación escrita del profesor? Si la
  hay, queda en un ADR (T4).


## Fase 1 — Documentación, UML e integración

- [x] **T1 — Mapa de la entrega.** Índice de documentación en `docs/README.md` con la tabla de
  equivalencias layout del enunciado → repo (`infraestructura/`, `orquestacion/`, `servicios/`) y
  cada entregable del enunciado con su ubicación. Ruta: por definir.
  Ruta: delegada (Codex), revisión de Claude. Evidencia (2026-10-05): `docs/README.md` con el
  mapeo de estructura, entregables y requisitos del enunciado; 50 enlaces relativos, 0 rotos.
  Correcciones de la revisión: líneas exactas de las secciones del PDF (174-181), el diagrama de
  despliegue ya no enlaza el compose como si fuera el diagrama, Falco anotado en la fase 6 y tabla
  de entregables propios (grilla de roles, guía de integración).
- [x] **T2 — Diagramas UML como código.** Casos de uso, componentes, despliegue y secuencia de
  autenticación en Mermaid o PlantUML versionados (los HTML de Archify actuales quedan como apoyo).
  Ruta: delegada (Codex), revisión de Claude. Evidencia (2026-10-05): diagramas Mermaid versionados en
  `docs/diagramas/uml/` y enlaces del índice actualizados. Claude renderizó los cuatro con
  `@mermaid-js/mermaid-cli` 11.17.0 (todos OK tras corregir la secuencia: un `;` cortaba el mensaje,
  comillas literales, camelCase de la API y el evento `security.mfa_challenge_issued`) y rehízo el
  de casos de uso con límites de sistema, óvalos e «include»/«extend».
- [x] **T3 — Threat Dragon.** Modelo `.json` versionado con DFD nivel 0 y 1, alineado con el STRIDE
  de `specs/05-security/threat-model.md`.
  Ruta: delegada (Codex), revisión de Claude. Evidencia (2026-10-05): `docs/threat-model/` con
  `identity-hub.json` (Threat Dragon v2, DFD nivel 0 y 1, 35 celdas, 23 amenazas STRIDE) y su guía.
  Formato copiado de los demos oficiales de Threat Dragon v2.6.2; valida contra
  `threat-dragon-v2.schema.json` oficial sin errores salvo `strokeDasharray: null`, que los demos
  oficiales también tienen; sin nodos superpuestos. Corrección de la revisión: Codex marcó las 23
  como `Mitigated`; cinco no lo están y quedan `Open` con lo que falta: AM-008 y AM-022 (Cosign,
  OIDC y `cd.yml`, T17), AM-009 (SBOM, T17), AM-014 (compose de producción, T9) y AM-019 (no hay
  regla de alerta ni runbook de la DLQ, solo el panel).
  - Hallazgo fuera de alcance (sin corregir): `specs/05-security/threat-model.md` repite el id
    **AM-017** para dos amenazas distintas (códigos MFA en Spoofing y Argon2id como amplificador en
    DoS). El modelo de Threat Dragon las distingue como "AM-017 (MFA)" y "AM-017". Decidir en T4 o
    con el usuario si se renumera.
  - AM-019 no tiene tarea asignada en este plan (alerta de Grafana y runbook de la DLQ).
- [x] **T4 — Manuales.** Arquitectura (consolida `specs/00-vision.md` y los ADR), despliegue y
  operación, seguridad (cómo leer los reportes de Trivy, ZAP, Gitleaks, Semgrep) y desarrollo
  (convención de ramas, commits y revisión, sobre `docs/guia-desarrollo.md`).
  Evidencia (2026-10-05): ruta delegada a Codex; `docs/manuales/arquitectura.md`,
  `docs/manuales/despliegue-y-operacion.md` y `docs/manuales/seguridad.md` documentan solo
  evidencia versionada; `docs/guia-desarrollo.md` incorpora contribución y `docs/README.md` enlaza
  los manuales. Comprobaciones estructurales: enlaces relativos, objetivos Make y variables de
  entorno verificadas contra sus fuentes (18 objetivos Make, 41 variables, 0 enlaces rotos).
  Correcciones de la revisión de Claude: el manual de seguridad decía que `osv-scanner.toml` no
  existía (está en `backend/osv-scanner.toml`, riesgo aceptado de VULN-028 hasta 2026-12-25), y la
  causa del 502 de Nginx era incorrecta (es la IP vieja de `api` tras recrearlo; se arregla con
  `docker restart identity-hub-web-1`, no esperando a la API).
- [ ] **T5 — Guía de integración de terceros.** Registro como cliente OAuth (`redirect_uri`, PKCE,
  CORS), endpoints y JWKS, declaración de permisos de la aplicación (se completa tras la fase 3),
  cambios en el frontend del tercero y checklist de seguridad.
  Ruta: delegada (Sonnet; Codex sin cuota hasta las 16:20), revisión de Claude. Evidencia (2026-10-05):
  `docs/manuales/integracion-terceros.md` con las nueve secciones (qué ofrece el Hub, registro del cliente
  y cómo agregar un segundo, flujo con diagrama Mermaid, referencia de endpoints, claims y validación,
  cambios en el frontend del tercero, checklist y limitaciones); `docs/README.md` enlaza la guía como
  parcial. La sección 7 (declaración de permisos) queda como marcador y se completa tras T15.
  Revisión de Claude: Mermaid renderiza (mermaid-cli 11.17.0); verificado contra el código que el
  cliente está fijo en `backend/internal/config/config.go:66-68` y sembrado en la migración
  000007. Corrección: la guía decía que el logout del Hub solo limpia la cookie; también revoca en
  el servidor todas las sesiones del Hub del usuario (`RevokeHubSessions`, `logout.go:74`).
  - Hallazgo fuera de alcance (sin corregir): `specs/adr/0009` dice que un segundo cliente "exige
    tocar configuración y desplegar", pero el código exige cambios de código (cliente único en
    `oauth.Service`, `IssueForAudience`, `oauthCORS` y catálogo de roles). Decidir si se enmienda el
    ADR o si la fase 3 lo resuelve.
  - OpenAPI no declara los `500`/`503` que emiten `/oauth/authorize` y `/oauth/token`.
- [x] **T6 — Compose en la raíz.** `docker compose up -d` desde la raíz sin `.env` previo, como
  promete el README (portabilidad). Decisión del usuario (2026-10-05): opción C, `make setup`
  genera el `.env` (respeta AM-012: ningún secreto versionado).
  Ruta: delegada (Sonnet; Codex sin cuota), revisión de Claude. Evidencia (2026-10-05): RED, `python3 scripts/setup_env_test.py` falló por falta de `setup_env.py`; GREEN, 8 pruebas OK tras implementarlo. Creados `scripts/setup_env.py` (+ test), objetivo `make setup` (también en `spec-drift`), `docker-compose.yml` raíz con `include` de `deploy/docker-compose.yml`, y README/manual de despliegue actualizados.
  Revisión de Claude con la plantilla real (la pasó el usuario; la sesión no puede leer
  `.env.example`): los secretos vienen vacíos con comentario al final de la línea y las URLs usan
  `${VAR}`, así que se quitó la lógica de reemplazo dentro de las URLs (nunca hacía nada útil) y se
  corrigió un bloqueante: `TRUSTED_PROXIES` vacío hacía fallar el `:?` del compose; ahora toma
  `172.28.0.0/16` como en `.github/actions/stack-up`. RED (falla solo
  `test_empty_trusted_proxies_gets_the_compose_subnet`) y GREEN 11/11; `traceability_test.py` 4/4.
  Prueba real de portabilidad: copia limpia del árbol sin `.env`, `make setup` + `docker compose up
  -d` (proyecto aislado): `/readyz` 200, Hub, Contabilidad y JWKS 200, servicios `healthy`; luego
  `down -v` y el stack del usuario restaurado (`/readyz` 200).
  - Hallazgo fuera de alcance (sin corregir): la regla `contrasena-en-variable-de-entorno` de
    `.gitleaks.toml` usa `\s*` tras el `=`, que también cruza saltos de línea: una clave vacía
    (`JWT_SIGNING_KEY=`) seguida de otra línea `CLAVE=` se reporta como secreto. El hook de T16 lo
    detectó en la plantilla de prueba; se evitó con una línea de comentario. Cambiar a `[ \t]*`
    afinaría la regla (decidir en qué tarea).
  - Revisión nativa (alto, 7 archivos, 306 líneas, 4 lentes): **aprobada** y acusada
    (`review-e11c51b694747de8`), 9 observaciones informativas. Aplicadas tres: `os.fchmod` antes de
    escribir (con `--force` sobre un `.env` 0644 los secretos quedaban legibles un instante; la
    prueba nueva comprueba el modo final, la ventana en sí no es observable de forma determinista),
    aviso de que `--force` exige `make clean` por las contraseñas guardadas en los volúmenes, y el
    `\s` inválido de la plantilla de prueba (`python3 -W error` limpio, 12/12).
  - Revisión nativa de `0773fb5` (alto, 25 líneas, 4 lentes): **aprobada** y acusada
    (`review-6d25da34448098d1`). Sus dos advertencias eran correctas: `fchmod` tras `O_TRUNC`
    dejaba el `.env` vacío si fallaba (Windows con Python < 3.13, archivo de otro dueño). Corregido
    con escritura atómica: archivo temporal nuevo `O_EXCL` 0600 en el mismo directorio y
    `os.replace`; el `.env` existente nunca se trunca. RED (falla
    `test_failed_replace_keeps_the_existing_file_and_leaves_no_temp`) y GREEN 13/13.
  - Revisión nativa de `8952c0d` (alto, 40 líneas, 4 lentes): **aprobada** y acusada
    (`review-b5c230077a2a232a`), solo sugerencias informativas (fsync antes de `os.replace`, un
    `.env` enlazado simbólicamente se reemplaza por un archivo, parche global de `os.replace` en
    la prueba, fd sin cerrar si falla `fdopen`). No se aplican: es un generador de desarrollo local
    de un solo uso; se dejan anotadas.

## Fase 2 — IaC de referencia (producción en la nube, como ejemplo)

- [x] **T7 — Investigación y diseño.** Arquitectura de referencia en AWS y qué valida LocalStack
  (Q2); ADR con la decisión.
  Investigación (2026-10-05, fuentes primarias consultadas ese día, la mayoría sin fecha):
  - Plan Student de LocalStack = cobertura de Ultimate (500 MB de Cloud Pods, soporte básico),
    "mientras seas estudiante" (no figura "un año"; confirmar en la cuenta del usuario). Permite CI
    (1.000 créditos mensuales). Token: `LOCALSTACK_AUTH_TOKEN`; desde marzo de 2026 la imagen
    `latest` lo exige. Fuentes: docs.localstack.cloud/aws/licensing/,
    localstack.cloud/localstack-for-students, blog.localstack.cloud (2025-10-28 y 2026-03-17).
  - Ejecución real: ECS Fargate (contenedores en Docker local), RDS PostgreSQL 13-17, ALB
    (forward/redirect/fixed-response). Emulados: S3, Secrets Manager, SSM, KMS, CloudWatch Logs,
    IAM (con enforcement opcional), SES v1 (puede reenviar a un SMTP). Solo CRUD o mock: ACM, Route
    53, WAFv2, security groups. ECR local con push de capas sin confirmar.
  - **Amazon MQ para RabbitMQ no está soportado** (solo ActiveMQ). Resuelto en D7.
  - Terraform: `tflocal` 0.26.0 o `lstk terraform` generan un override de endpoints (el `.tf` de
    producción queda limpio). AWS provider v6.67.0 (2026-09-30).
  - Checkov: `bridgecrewio/checkov-action` con `directory` y `framework`; escanea también
    Dockerfiles y workflows. Versión a fijar al implementar (PyPI y GitHub no coinciden).
  Diseño: `specs/adr/0012-arquitectura-de-referencia-en-aws-con-localstack.md` (estado
  `propuesta`, a la espera de que el usuario la acepte). Ruta: inline (decisión de arquitectura del
  orquestador). Hallazgo para el ADR: el worker envía SMTP sin autenticación ni TLS
  (`backend/cmd/worker/main.go:198-199`), así que SES real exigiría cambiar código.
- [ ] **T8 — Terraform.** Módulos en `infraestructura/` (o la carpeta que fije T1) para la
  arquitectura de referencia; `terraform validate`; `plan`/`apply` contra LocalStack donde se pueda.
- [ ] **T9 — Producción simulada local.** `docker-compose.prod.yml` (imágenes por digest, sin
  puertos de desarrollo, secretos por archivo).
- [ ] **T10 — Checkov en CI.** Job nuevo sobre Terraform, Dockerfiles y workflows; falla ante
  hallazgos de severidad alta.

## Fase 3 — Grilla de roles configurable

- [ ] **T11 — Spec.** RF nuevo, ADR (modelo permisos por aplicación → roles → usuarios; qué viaja en
  el token), escenarios Gherkin y enmienda del OpenAPI.
- [ ] **T12 — Backend.** Migraciones (permisos y roles editables), store, API de roles y permisos,
  auditoría, controles (solo `admin`, sin autoasignación, sin permisos desconocidos) y token con los
  permisos de la aplicación.
- [ ] **T13 — Consola del Hub.** Grilla roles × permisos por aplicación: crear, editar y borrar
  roles, y asignarlos a usuarios.
- [ ] **T14 — Contabilidad.** Autoriza por permisos en lugar de por nombre de rol.
- [ ] **T15 — E2E y trazabilidad.** Escena de la demo (crear `contabilidad.auditor` y verlo en
  Contabilidad), matriz al día.

## Fase 4 — Publicación y entrega

- [ ] **T16 — VULN-019.** Actualizar las imágenes restantes del compose.
- [ ] **T17 — Docker Hub.** Workflow de release en tag `vX.Y.Z`: imágenes con `vX.Y.Z` y `latest`,
  SBOM con Syft y firma con Cosign (ADR 0011, Q1).
- [ ] **T18 — Manual de usuario con capturas** (incluye la grilla de roles).
- [ ] **T19 — Informe técnico PDF** con las 8 secciones del enunciado.
- [ ] **T20 — Guion del video** de 10-15 minutos (ciclo completo: app, pipeline, despliegue,
  observabilidad) (Q3).
- [ ] **T21 — Cierre.** Bitácora, matriz, informe de seguridad final, tag de versión y PR.

## Progreso

| Fase | Tareas | Hechas |
|---|---|---|
| 1 — Documentación, UML e integración | T1 a T6 (6) | 5 (T1 a T4 y T6; T5 parcial) |
| 2 — IaC de referencia | T7 a T10 (4) | 1 (T7) |
| 3 — Grilla de roles configurable | T11 a T15 (5) | 0 |
| 4 — Publicación y entrega | T16 a T21 (6) | 0 |
| **Total** | **21** | **6** |

## Siguiente paso

T8 (Terraform de la arquitectura de referencia), cuando el usuario acepte la ADR 0012. T5 queda abierta hasta T15 (permisos).

## Cambios de spec propuestos

Ninguno todavía (la fase 3 los define en T11).

## Notas de handoff Codex

—
