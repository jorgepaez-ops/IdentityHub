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

- **D8 · Sin `apply` en LocalStack (2026-10-05):** probarlo complica de más; la IaC se valida con
  `terraform validate` y Checkov, y la configuración y el despliegue se muestran con un diagrama.
  La ADR 0012 queda enmendada.

- **D9 · Diseño de la grilla de roles (2026-10-05, aprobado por el usuario):** cada aplicación
  declara sus permisos (tabla `permissions` ligada a `applications`, sembrada por migración); los
  roles de aplicación son datos editables (`roles.application_id`, `role_permissions`), con
  nombre `<app>.<nombre>`; `contabilidad.senior` y `contabilidad.analista` pasan a filas con
  permisos (`movimientos.ver_todos`, `movimientos.aprobar`, `cierre.ejecutar`, `reportes.ver` y,
  por ajuste de la revisión de T11, `movimientos.registrar`: hoy registrar no depende de ningún
  permiso y el auditor de la demo debe ser de solo lectura)
  sin cambiar su comportamiento. El token de la aplicación lleva `permissions` resueltos además de
  `roles`; Contabilidad decide por permisos. Grilla solo para `admin`. Controles: (1) solo se
  crean roles de aplicación (`admin`/`user` son del sistema, no editables ni borrables); (2) un
  admin no edita los permisos de un rol que él tiene; (3) un rol solo lleva permisos de su
  aplicación, permiso desconocido = 400; (4) no se borra un rol asignado; (5) auditoría
  `role_created`/`role_updated`/`role_deleted`. Un cambio de permisos rige desde el siguiente
  token (≤ 15 min). RF-021 nuevo; RF-009 y RF-020 enmendados. Escena de demo:
  `contabilidad.auditor` con `reportes.ver` y `movimientos.ver_todos`.

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
- [x] **T5 — Guía de integración de terceros.** Registro como cliente OAuth (`redirect_uri`, PKCE,
  CORS), endpoints y JWKS, declaración de permisos de la aplicación (se completa tras la fase 3),
  cambios en el frontend del tercero y checklist de seguridad.
  Ruta: delegada (Sonnet; Codex sin cuota hasta las 16:20), revisión de Claude. Evidencia (2026-10-05):
  `docs/manuales/integracion-terceros.md` con las nueve secciones (qué ofrece el Hub, registro del cliente
  y cómo agregar un segundo, flujo con diagrama Mermaid, referencia de endpoints, claims y validación,
  cambios en el frontend del tercero, checklist y limitaciones); `docs/README.md` enlaza la guía como
  parcial. La sección 7 (declaración de permisos) queda como marcador y se completa tras T15.
  Sección 7 completada (2026-10-06) — Ruta: inline (Claude, un archivo de documentación): modelo de
  permisos, declaración por migración (`000009`), claves de Contabilidad, claim `permissions` con
  ejemplo del auditor, endpoints de la grilla y los cinco controles; también el claim en la tabla de la
  sección 5, «Guardas por permiso» en la 6, dos ítems del checklist y una limitación en la 9.
  `docs/README.md` marca la guía como completa. Verificación estructural: todos los enlaces relativos
  existen; datos contrastados con el ADR 0013, `token.go`, `jwt.ts` y la migración.
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
- [x] **T8 — Terraform.** Módulos en `infraestructura/` (o la carpeta que fije T1) para la
  arquitectura de referencia; `terraform validate`; `plan`/`apply` contra LocalStack donde se pueda.
  - T8a — Ruta: delegada (Sonnet: necesita red para terraform init), revisión de Claude. Evidencia (2026-10-05): Terraform en `infraestructura/terraform/` (14 archivos .tf, 1379 líneas, sin módulos externos; `README.md` en español, `terraform.tfvars.example`, `.gitignore`, `.terraform.lock.hcl` versionable): red (VPC, 2 públicas y 2 privadas, NAT), security groups de mínimo privilegio, ALB con reglas por Host y HTTP→HTTPS, ECS (api, worker, web, broker, mailpit solo con `localstack`, migrate de un solo uso), RDS cifrado, Secrets Manager con `random`, KMS, CloudWatch Logs, Cloud Map, ACM/Route 53/WAFv2/SES. Verificación: `terraform init -backend=false` OK, `terraform fmt -check -recursive` OK, `terraform validate` Success; grep sin secretos literales ni credenciales AWS. Sin aplicar nada. Pendiente T8b (apply con LocalStack); incertidumbre: DNS de Cloud Map en tareas locales.
  Revisión de Claude: `fmt`/`validate` repetidos en verde; RDS cifrado y sin acceso público,
  tareas sin IP pública, ALB con `drop_invalid_header_fields`; `.terraform/` fuera de git. Brechas
  del código frente a producción añadidas a la ADR 0012 (hosts fijos, `X-Forwarded-For` detrás del
  ALB anula el límite por IP, rol `identity_app` manual en RDS). Gitleaks (hook T16): 7 falsos
  positivos (referencias `random_password.*.result`) registrados como huellas sin commit en
  `.gitleaksignore`, con aprobación del usuario.
  - Correcciones de la revisión nativa (review-1d206709c8c0146b, aprobada) — Ruta: delegada (Sonnet), revisión de Claude: (1) README usa la salida nueva `private_subnet_ids_csv` con `-raw`; (2) mailpit con volumen y montaje `tmp`; (3) broker con `deployment_minimum_healthy_percent = 0` y máximo 100; (4) web con `force_new_deployment` y `triggers` sobre la task definition de api, y viñeta en la ADR; (5) despliegue en dos fases documentado (apply con `-target` a task definitions y RDS, `run-task` migrate, apply completo) y diagrama de secuencia corregido; (6) api con `wait_for_steady_state`; (7) variable `nat_gateway_per_az` (NAT y tabla de rutas por zona); (8) `api_desired_count` renombrada a `app_desired_count`; (9) comentario de Cloud Map actualizado y viñeta en la ADR; (10) avisos sobre números de línea en rds.tf y secrets.tf y siete huellas de `.gitleaksignore` actualizadas (rds 22; secrets 33 a 38).
  Revisión de Claude de las correcciones: `validate` y render OK; el hook pasa con los archivos en
  stage. El escaneo del historial completo (como el job 3 de CI) falló con 3 hallazgos del commit
  original `64330d5`, porque Sonnet reemplazó las huellas de línea vieja por las nuevas; se
  agregaron las 7 huellas con commit de `64330d5` y el historial vuelve a 0 hallazgos.
  - [x] T8b — Diagrama de la configuración y el despliegue en AWS (sustituye el `apply` en
    LocalStack por D8). Ruta: inline. `docs/diagramas/uml/despliegue-aws.md`: infraestructura
    (flowchart) y despliegue de una versión (secuencia), con datos tomados del Terraform; ambos
    renderizan con mermaid-cli 11.17.0. Gitleaks del historial completo (v8.24.3, como CI): 0
    hallazgos con las huellas sin commit.
- [x] **T9 — Producción simulada local.** `docker-compose.prod.yml` (imágenes por digest, sin
  puertos de desarrollo, secretos por archivo).
  Ruta: delegada (Sonnet), revisión de Claude. Evidencia (2026-10-05): `deploy/docker-compose.prod.yml` + `make up-prod`/`down-prod`; `docker compose config` OK, solo `web` publica puerto (también con perfil observability), api/worker/web sin build; AM-014 pasa a Mitigated (4 Open). Secretos siguen por env (sin `*_FILE`); falta correr el stack real.
  Prueba real de Claude: proyecto aislado `idhprod` con las imágenes locales y el override:
  servicios `healthy`, Hub y JWKS 200 a través de `web`; desde el host 5432, 5672, 15672, 8081,
  9091, 1025 y 8025 cerrados (solo `web` publica 8080); Mailpit no arranca. Luego `down -v` y el
  stack del usuario restaurado (`/readyz` 200). AM-014 pasa a `Mitigated` en el modelo de amenazas.
  Confirmado con el binario local de Trivy que `trivy config` no analiza Compose, así que la
  evidencia de AM-014 que cita el spec ("Trivy config sobre el compose de producción") no existe;
  la evidencia real es esta prueba (anotado en `docs/threat-model/README.md`, el spec queda sin tocar).
- [x] **T10 — Checkov en CI.** Job nuevo sobre Terraform, Dockerfiles y workflows; falla ante
  hallazgos de severidad alta.
  Ruta: delegada (Sonnet), revisión de Claude. Evidencia (2026-10-05): job `12 · IaC (Checkov)` en `ci.yml` (Checkov 3.3.23 por pip, no `checkov-action`: su release v12.1347.0 trae Checkov 2.0.930; `soft_fail` no existe, SARIF categoría `checkov`, `if: always()`); `make scan-iac` en `scan`. Corregidos: flow logs de la VPC, RDS (Performance Insights, monitoreo extendido, parameter group con `log_statement`, `log_min_duration_statement` y `rds.force_ssl`), logging del WAF, regla `AWSManagedRulesAnonymousIpList` en modo count (CKV2_AWS_76 exige ese grupo además de KnownBadInputs), AZ fijadas con filtro `zone-name`, HEALTHCHECK en `api` y `web`. Omisiones en línea: CKV_AWS_260, CKV_AWS_378, CKV_AWS_111/356/109, CKV_AWS_336, CKV2_AWS_57, CKV2_AWS_5, CKV2_AWS_38, CKV2_AWS_39, CKV_AWS_91 y CKV_GHA_7 (baseline-scan, ADR 0007). Resultado local: terraform 327 pasan/0 fallan/23 omitidas, dockerfile 139/0, github_actions 471/0/1 omitida.

  Correcciones por el CI del PR #10 — Ruta: delegada (Sonnet), revisión de Claude: Trivy config (AWS-0104 x7, AWS-0053) y SonarCloud. Egress 0.0.0.0/0 restante (HTTPS 443 por la NAT y SMTP 587 a SES) con `#trivy:ignore:AWS-0104` y justificación (VPC endpoints y espejo de ECR, ADR 0012); `#trivy:ignore:AWS-0053` en `aws_lb.main` (ALB público por diseño: WAF, TLS 1.3, redirección). Excepciones de Trivy aceptadas: AWS-0104 y AWS-0053 (más AWS-0132 en el bucket de logs: ELB solo admite SSE-S3). Logs de acceso del ALB a un bucket S3 nuevo (`alb_logs.tf`: bloqueo público, BucketOwnerEnforced, SSE-S3, versionado, expiración = `log_retention_days`, política TLS-only) y se quita la omisión CKV_AWS_91 (4 omisiones nuevas de Checkov justificadas en el bucket). `ci.yml` job 12: `pip install --only-binary :all:`. `scripts/setup_env.py`: --template y --output deben quedar dentro de la raíz del repositorio (parámetro `root` de `main()` solo para pruebas; prueba RED primero) y `# NOSONAR` en la subred de compose.

  Verificación de Claude: Checkov 3.3.23 en los tres frameworks, 0 fallos (terraform 327/0/23
  omitidos, dockerfile 139/0, github_actions 471/0/1); Hadolint OK en los dos Dockerfiles;
  `terraform validate` OK; imágenes reconstruidas con `make up`: `api` y `web` quedan `healthy` con
  los HEALTHCHECK nuevos (el worker no tiene, justificado en el Dockerfile). Desviación aceptada:
  el job instala `checkov==3.3.23` con pip porque `bridgecrewio/checkov-action` corre una imagen
  2.0.930 sin los chequeos de AWS actuales.

## Fase 3 — Grilla de roles configurable

- [x] **T11 — Spec.** RF nuevo, ADR (modelo permisos por aplicación → roles → usuarios; qué viaja en
  el token), escenarios Gherkin y enmienda del OpenAPI.
  Evidencia (2026-10-05): ruta delegada a Codex; `RF-021`, ADR 0013 y los escenarios diferidos
  documentan D9. La correspondencia Gherkin↔E2E exige pruebas que se crean en T15, por lo que los
  escenarios quedan en la ADR y la matriz registra RF-021 como diferido.
  Revisión de Claude: requisitos y ADR coherentes con D9 (rutas por aplicación, códigos
  400/403/404/409, GRANT mínimos, auditoría, razonamiento de los controles 1 y 2); verificado en el
  código que el analista ya ve el Resumen (por eso lleva `reportes.ver`). Corrección: «Registrar
  movimiento» no dependía de ningún permiso, así que el auditor «de solo lectura» habría podido
  registrar; se agrega `movimientos.registrar` (senior y analista) y se ajusta el escenario.
  `traceability.py --check` y `traceability_test.py` en verde.
- [x] **T12 — Backend.** Migraciones (permisos y roles editables), store, API de roles y permisos,
  auditoría, controles (solo `admin`, sin autoasignación, sin permisos desconocidos) y token con los
  permisos de la aplicación.
  T12a — Ruta: delegada (Codex), revisión de Claude. Evidencia (2026-10-05): RED observado con
  `GOCACHE=/private/tmp/identity-hub-gocache go test ./internal/auth/token -run
  'TestRF020_Token(ParaAplicacionTieneAudienciaYRolesAcotados|DelHubOmitePermisos)'`: la nueva
  firma y `Claims.Permissions` aún no existían. GREEN: pruebas focalizadas de token, OAuth, admin,
  employee, roles y API pasan; `go vet -tags=integration ./...` compila las pruebas de PostgreSQL.
  Corrección de revisión: RED confirmó que un token de aplicación sin permisos omitía el claim;
  GREEN conserva la omisión en tokens Hub e incluye `permissions: []` para la audiencia de aplicación.
  T12a — Revisión de Claude: build, vet (también `-tags=integration`), unitarias con `-race` y
  golangci-lint (0) en verde; frontend typecheck, lint y 148/148. La integración local se había
  «aprobado» sin base (`TEST_DATABASE_URL` vacío, pruebas omitidas, cobertura 56,9 %); con un
  Postgres desechable igual al de CI pasa entera con cobertura 78,9 % (`main`: 77,6 %). Bug
  encontrado y corregido: el trigger de roles del sistema devolvía `OLD` también en `UPDATE`, lo
  que en PostgreSQL descarta el cambio en silencio, así que ningún rol de aplicación habría sido
  editable en T12b; además permitía crear roles `system = true` o promover uno. RED
  (`TestRF021_TriggerSoloProtegeRolesDelSistema`: el UPDATE no persistía) y GREEN tras devolver
  `NEW` y cubrir `INSERT`. `mfa_test.go` (fuera de las superficies) solo cambió una constante
  eliminada por un literal, sin tocar aserciones.
  T12a — Revisión nativa (`review-35646284ed805e4a`, alto, 902 líneas, 4 lentes): **aprobada**.
  Observaciones que pasan a T12b: probar el error de la consulta de permisos en el canje OAuth y
  los errores del validador en alta de empleado y en admin; quitar la etiqueta engañosa
  `permissions,omitempty` de `Claims`; corregir los comentarios de columnas de la migración 000009.
  Y a T13: el cajón de usuario sigue listando roles fijos; debe cargarlos de la API.
  T12b — Ruta: delegada (Codex inició, sin cuota; completó Sonnet), revisión de Claude. Evidencia
  (2026-10-06): lo de Codex (OpenAPI, sqlc, esqueleto de servicio/store/handlers) venía sin `gofmt`,
  con `Application.key`/`Permission.applicationId` fuera de lo pedido, sin conteo de asignaciones ni
  `description` al crear, sin pruebas más allá de una; se reescribieron servicio, store y handlers y
  se conservaron las rutas, los parámetros y la composición en `Server`. RED observado:
  `go test ./internal/auth/rolegrid` y `./internal/api -run RF021` no compilaban (`Description`,
  `ActionRoleCreated`, `MaxDescriptionLength`, `ErrInvalidDescription`, `ErrEmptyUpdate` y la
  forma de `Application` inexistentes); GREEN tras reescribir: cada control (1 nombre y rol del
  sistema, 2 rol propio con el 403 antes del 409 al borrar, 3 permiso desconocido, 4 asignado,
  5 auditoría `role_created/updated/deleted` en la misma transacción con IP y agente), duplicado,
  404, éxito, fallo de auditoría que aborta, y handlers (mapeo de estados, 403 a no admin, 401
  anónimo, 503 sin servicio, renombrar = 400). Seguimientos de T12a: pruebas del canje OAuth con
  fallo de permisos, del validador en alta de empleado y en admin (verifican comportamiento ya
  correcto: sin RED posible), etiqueta `Claims.Permissions` sin `omitempty` (la lee `Validate`;
  `MarshalJSON` decide), comentarios de columnas de 000009 corregidos. Integración con Postgres
  desechable (`make test-integration`): todo en verde, cobertura total 80,3 %. Cubre listado con
  conteos, grants de `identity_app`, ciclo crear/actualizar/borrar con auditoría, rol propio,
  asignado, trigger y FK mapeados, y un rol de otra aplicación como 404.
  T12b — Verificación independiente de Claude: build, vet (también integración), unitarias `-race`,
  golangci-lint 0, frontend typecheck y lint, trazabilidad; integración contra Postgres desechable
  en verde con cobertura 80,3 % (T12a: 78,9 %). Servicio revisado: control 1 por prefijo del
  `client_id`, controles 2 antes que 4 en el borrado (403 antes de 409, documentado), rol buscado
  dentro de su aplicación (404 entre aplicaciones), auditoría en la misma transacción. Composición
  real: con `make up` la base del usuario queda en la migración 9 con 5 permisos y los endpoints
  de la grilla responden 401 sin sesión (no 501).
  Correcciones de la revisión nativa de T12b (review-637b55288a3cf589, aprobada) — Ruta: delegada
  (Sonnet), revisión de Claude: build, vet de integración, unitarias `-race`, lint 0 y trazabilidad repetidos en verde; Sonnet corrió la integración contra Postgres (80,4 %). Desviación aceptada en (4): las constantes de auditoría viven en `rolegrid` y `audit` las reutiliza, porque al revés hay un ciclo de imports. Se corrigieron siete avisos: (1) los 500 del admin de roles
  registran el error con request_id sin filtrarlo al cliente (RED observado); (2) OpenAPI: patrón y
  descripciones de `ApplicationRoleName` alineados con Go y 409/403 con `application/problem+json`;
  (3) `mapRoleGridError` distingue por nombre de restricción (`roles_name_key`,
  `user_roles_role_id_fkey`, `roles_application_id_fkey`) y envuelve el resto con `%w` (RED observado);
  (4) las acciones de auditoría se definen una sola vez en rolegrid y `audit` las referencia (rolegrid
  no puede importar audit: ciclo audit -> api -> rolegrid); (5) bloqueo `FOR UPDATE` de la fila del rol
  en Update/Delete antes de evaluar el control 2 y el conteo (RED por compilación; prueba de integración
  de bloqueo real); (6) pruebas de que el token del Hub no lleva `permissions` y el de aplicación con
  lista vacía lleva `[]` (sin RED posible: el comportamiento ya existía); (7) motivo de RF-021 en
  `DEFERRED` actualizado (API en T12; faltan interfaz y E2E, T13 a T15).
- [x] **T13 — Consola del Hub.** Grilla roles × permisos por aplicación: crear, editar y borrar
  roles, y asignarlos a usuarios.
  Ruta: delegada (Sonnet; Codex sin cuota), revisión de Claude. Evidencia (2026-10-06): RED con
  `RolesPage` vacío: 11 de 12 pruebas nuevas de `roles.test.tsx` fallan (la del no admin ya pasaba) y
  8 de `admin.test.tsx` fallan al exigir roles de aplicación desde la API; GREEN tras implementar
  `RolesPage` (grilla roles × permisos, guardar con PATCH solo de lo cambiado, crear con nombre
  validado, borrar con confirmación, solo lectura del rol propio, detalle RFC 7807) y el cajón de
  usuario con los roles de `GET /admin/applications` agrupados por aplicación con sus permisos.
  `npm run typecheck` y `npm run lint` limpios, `vitest run` 6 archivos / 162 pruebas en verde,
  `npm run build` correcto, trazabilidad al día (matriz sin cambios). Pendiente: probarlo en el
  stack real.
  Revisión nativa de T13 (`review-86ef83509fad2952`, medio, 618 líneas): **aprobada**. Corregido:
  la fila de la grilla no salía del estado «guardando» si el servidor devolvía el mismo rol (no hubo
  RED posible con datos de prueba; arreglo de una línea).
  Prueba en navegador real de Claude (el usuario hará la suya después): Playwright temporal contra el
  stack, con admin sembrado y MFA vía Mailpit: crea `contabilidad.auditor-<id>` con `reportes.ver` y
  `movimientos.ver_todos` desde la grilla, lo asigna a un empleado desde el cajón y el rol asignado
  queda con «Eliminar» deshabilitado. Pasó; la suite E2E existente sigue 46/46 con T12 y T13. Ajuste
  visual: los encabezados de permisos se cortaban a mitad de palabra; ahora `white-space: nowrap`
  (un `<wbr>` rompía el nombre accesible de la columna). Roles de prueba borrados de la base local.
- [x] **T14 — Contabilidad.** Autoriza por permisos en lugar de por nombre de rol.
  Ruta: delegada (Sonnet), revisión de Claude. Evidencia (2026-10-06): RED con la implementación original y las pruebas nuevas: 7 TestRF021_* fallan (53 pasan); GREEN: npm run typecheck, lint y build limpios, vitest 60/60 (53 previas + 7 nuevas TestRF021_: 2 de jwt y 5 de App; los TestRF009_* intactos).
  Verificación de Claude: typecheck, lint, 60/60, build y trazabilidad; `web` reconstruido y la
  suite E2E completa contra el stack sigue 46/46 (senior, analista y la demo sin cambios).
  Revisión nativa de T14 (`review-a0b97eb37397a2f5`, alto, 219 líneas, 4 lentes): **aprobada**,
  sin bloqueantes. Ocho avisos informativos, a decidir como trabajo aparte: R3-001 (la prueba del
  auditor dice verificar el cierre bloqueado pero no abre esa vista), R3-002 (el Resumen bloqueado no
  tiene prueba de render), R4-001 (un token emitido antes de T12a, sin `permissions`, degrada a un
  senior a «propios» hasta renovar la sesión), R2 (etiqueta derivada del nombre de rol; la grilla de
  permisos repetida en `testing.ts` y `SEEDED_HINTS`; literales de permiso en `Closing.tsx` y
  `Summary.tsx`; el centinela `undefined` de `signInAs`).
- [x] **T15 — E2E y trazabilidad.** Escena de la demo (crear `contabilidad.auditor` y verlo en
  Contabilidad), matriz al día.
  Ruta: delegada (Sonnet: necesita Docker y el stack), revisión de Claude. Evidencia (2026-10-06):
  los seis escenarios del ADR 0013 pasan a `specs/06-acceptance/roles-y-permisos.feature` (el ADR
  queda con un puntero); `e2e/tests/roles-y-permisos.spec.ts` con un E2E por escenario (la escena del
  auditor por la interfaz de la consola y de Contabilidad; el resto por la API) y `e2e/support/roles.ts`;
  RF-021 sale de `DEFERRED` y la matriz queda «completo» (6 escenarios, 31 pruebas Go, 6 E2E). RED: la
  escena del auditor falló primero por un localizador ambiguo; las otras cinco pasaron a la primera
  porque la API (T12) y la interfaz (T13) ya existían. Avisos de la revisión de T14: R3-001 (el botón de
  Cierre está deshabilitado, así que la prueba de App verifica el candado y que no se abre) y R3-002
  (`views/Locked.test.tsx` renderiza las vistas bloqueadas; mutación comprobada).
  Verificación: `make e2e` 52/52, typecheck de e2e, typecheck/lint y vitest 63/63 de Contabilidad,
  `make spec-drift` al día, sin roles de prueba en la base. Corrección de Claude: `purgeRole` se negaba
  solo a nombres fuera de `contabilidad.*`; ahora también rechaza los roles sembrados (senior,
  analista). Re-corrida: los 6 E2E de RF-021 pasan. Desviación aceptada: el E2E no decodifica el token;
  verifica el texto «Tu rol permite…», que `access.ts` deriva de sus permisos (el contenido exacto del
  claim lo cubren las pruebas de integración de T12a). Ojo: el E2E borra `contabilidad.auditor` al
  empezar y al terminar (corregido después: ahora usa un nombre único, ver abajo).
  Commit `ac3922f`. Revisión nativa (`review-c9e878db05b47f89`, alto, 451 líneas, 4 lentes): **aprobada**,
  sin bloqueantes. Avisos informativos que coinciden en dos puntos: (1) el nombre fijo
  `contabilidad.auditor` que el E2E purga directo en la base (R1-001, R4-001, R2-002, R3-003): borra un
  auditor hecho a mano y choca entre corridas concurrentes; (2) el escenario promete «el token contiene
  solo…» y el E2E no lo verifica (R1-002, R2-001, R3-001). Menores: R3-002 (la prueba del rol de
  directorio solo mira `admin`, no `user`), R2-003, R2-004/R3-004 (el clic sobre un botón deshabilitado
  no prueba nada nuevo).
  Correcciones de los avisos (2026-10-06) — Ruta: inline (Claude): el E2E del auditor usa un nombre
  único (`contabilidad.e2e-auditor-<hex>`) y ya no purga `contabilidad.auditor`; lee la respuesta de
  `/oauth/token` y comprueba que el token trae el rol y exactamente `movimientos.ver_todos` y
  `reportes.ver` (mutación: esperar también `cierre.ejecutar` hace fallar la prueba); la prueba del rol
  de directorio mira `admin` y `user`; el escenario dice «un rol auditor» en vez del nombre fijo; se
  quita el clic inútil de la prueba de App. Verificación: 6/6 E2E de RF-021, typecheck de e2e y de
  Contabilidad, lint, vitest 63/63, `make spec-drift` al día, sin roles de prueba en la base.
  **Hallazgo nuevo fuera de alcance (CI del PR #11, 2026-10-06):** el job «5 · Dependencias
  vulnerables» (`npm audit --audit-level=high`) falla por GHSA-68fv-2mgg-jv7q (alta, DoS del event
  loop) en `source-map-js` 1.2.1 de `frontend` (solo desarrollo: `@vitest/coverage-v8` → `magicast` y
  `jsdom` → `css-tree`); Contabilidad ya tiene 1.2.2. Aviso publicado después de abrir la fase, no lo
  introduce este PR. Sin id VULN asignado. Remediado en el mismo PR por decisión del usuario (inline,
  Claude, necesita red): `source-map-js` 1.2.2 en `frontend/package-lock.json`; `npm audit` 0, vitest
  con cobertura 162/162, build y lint en verde. SonarCloud: quality
  gate aprobado con 12 code smells nuevos (props de solo lectura y `role="status"` en `RolesPage.tsx`,
  literales repetidos en `admin_roles.go`, complejidad en `rolegrid_integration_test.go`, `ASC` en
  `roles.sql`).

## Fase 4 — Publicación y entrega

- [x] **T16 — VULN-019.** Actualizar las imágenes restantes del compose.
  Ruta: delegada (Sonnet: necesita red y Docker), revisión de Claude. Evidencia (2026-10-06): commit
  `14047c7`, `mailpit` v1.31.4, `migrate` v4.20.1, `prometheus` v3.15.0, `loki` 3.7.8, `alloy` v1.20.1 y
  `grafana` 13.2.3, cada una por digest de su índice multiarquitectura (Claude comprobó los 6 digests
  contra sus tags con `docker buildx imagetools`); también la imagen de mailpit en Terraform.
  `deploy/observability/*` sin cambios. Verificación (Sonnet): `docker compose config` con y sin
  observabilidad, `make up-obs` sano, migrate con salida 0, `/readyz` 200, Prometheus con api, worker,
  rabbitmq y prometheus `up` (Claude lo repitió), Grafana 13.2.3 con sus dos fuentes de datos y el
  dashboard, Loki con logs del api vía Alloy, `make e2e` 52/52, `terraform fmt -check` y `validate`.
  RED no aplica (cambio de imágenes): la prueba es el escaneo antes/después.
  Evidencia «después»: run `baseline-scan` 37473616659 (`workflow_dispatch`, `ref=924b581`), total de
  las 8 imágenes 710 → 108 (`mailpit` y `loki` 0); residual detallado en
  `security/evidence/actions-37473616659/README.md`; ficha, `evidencia.json` y README raíz al día
  (28 fichas remediadas). Capturas de Claude Desktop confirmadas (informe, capturas 93 a 95).
  Revisión nativa (`review-dad3dc9c349c9821`, medio, 26 líneas, 1 lente): **aprobada**; avisos
  informativos R3-001 a R3-005 (cambios de versión mayor sin prueba en el candidato, E2E con el nuevo
  mailpit, digests multiarquitectura), todos cubiertos por la verificación anterior.
  Revisión nativa de la evidencia (`review-0acc883077aedd36`, alto, 191 líneas, 4 lentes): **aprobada**.
  R1-001, R2-001 y R3-001 coincidían en que 51 de los 117 hallazgos de `postgres` tenían parche: se
  vuelve a fijar `postgres:16-bookworm` en su digest reconstruido (misma 16.15) en el compose y en el
  servicio de CI (`924b581`, Trivy local 117 → 88, con parche 51 → 22; base sana, `/readyz` 200,
  `make e2e` 52/52) y se relanza el run (37473616659 reemplaza a 37471823657). R2-002: el párrafo de T30
  queda marcado como histórico. R3-002 (forma de `captura` en null): sigue el protocolo de evidencia.
- [ ] **T17 — Docker Hub.** Workflow de release en tag `vX.Y.Z`: imágenes con `vX.Y.Z` y `latest`,
  SBOM con Syft y firma con Cosign (ADR 0011, Q1).
  Pospuesta al final de la fase 4 por decisión del usuario (2026-10-06), a la espera de Q1 (namespace
  y token de Docker Hub). Propuesta de Claude: repositorios `api`, `worker` y `web` (`web` ya sirve el
  Hub y Contabilidad), environment protegido `dockerhub`, tag de prueba `v0.9.0` y `v1.0.0` en T21.
- [x] **T18 — Manual de usuario con capturas** (incluye la grilla de roles).
  Ruta: delegada (Sonnet: necesita el stack y un navegador), revisión de Claude. Evidencia (2026-10-06):
  `docs/manuales/usuario.md` por tipo de usuario (empleado, administrador, Contabilidad por rol), con la
  historia del auditor para la sustentación (enunciado 4.6) y problemas frecuentes; 48 capturas en
  `docs/manuales/img/usuario/` (3,5 MB, la mayor 188 KB) generadas por `e2e/manual/capturas.ts`
  (`make capturas`, fuera de `make e2e` y de la matriz); vista previa de Mailpit enmascarada (lleva el
  código y el enlace). `docs/README.md` marca el manual como hecho. Verificación (Sonnet): `make capturas`
  1/1 (48 PNG), typecheck de e2e, `make e2e` 52/52, `make spec-drift` al día, 48 imágenes referenciadas
  y existentes, sin roles del script en la base. Revisión de Claude: tres capturas abiertas (grilla,
  Mailpit, resumen del auditor) sin datos sensibles; enlaces relativos del manual existen. Nota: la grilla
  muestra «Asignado a 97/37 usuarios» por las cuentas que dejan las corridas E2E en la base local.
  **Hallazgo nuevo fuera de alcance (T18):** los correos del Hub no declaran codificación:
  `buildRawMessage` (`backend/cmd/worker/main.go`) no envía `MIME-Version` ni
  `Content-Type: text/plain; charset=utf-8` (ni codifica el asunto según RFC 2047), así que Mailpit
  muestra «GÃ³mez» en vez de «Gómez». Error funcional, no de seguridad; sin id; falta decidir la tarea.
  Al commitear: gitleaks marcó tres líneas del script (`password: <identificador>` y un literal de prueba);
  se reescribieron con alias cortos, sin tocar `.gitleaks.toml` ni `.gitleaksignore`. iCloud había
  renombrado o borrado 15 capturas: se regeneraron en una carpeta limpia (48/48, nombres exactos).
  Commit `6496a92`. Revisión nativa (`review` de T18, medio, 882 líneas, 1 lente): **aprobada**; corregido
  R3-stale-toast-save (el script ahora espera el aviso «guardado», no cualquier aviso); la captura
  vigente ya era correcta. R3-stage-leak-on-failure (carpeta temporal si la corrida falla) queda como
  sugerencia.
- [x] **T19 — Informe técnico PDF** con las 8 secciones del enunciado.
  Ruta: delegada (Sonnet: necesita Docker, navegador y red para npm), revisión de Claude. Evidencia
  (2026-10-06): `docs/informe/informe-tecnico.md` → `informe-tecnico.pdf` (44 páginas, 2,2 MB) con
  `make informe` (`build.mjs`: Markdown a HTML, Mermaid en Chromium, PDF en dos pasadas para el índice);
  diagramas y fragmentos de workflow leídos de los archivos reales al compilar; capturas de Grafana,
  Loki, Prometheus y sus objetivos (`capturas-grafana.mjs`, sin credenciales en pantalla). Secciones en el
  orden del enunciado, tabla VULN-001 a VULN-031, Anexo A (puesta en marcha: `make setup` antes de
  `make up`, direcciones, objetivos de make, producción simulada) y Anexo B (índice de artefactos).
  Portada con la materia y los cinco integrantes que dio el usuario; queda «[completar: docente]».
  `npm audit` 0 en `docs/informe` (override de `katex`), porque osv-scanner recorre todo el repo.
  Revisión de Claude: páginas 1, 2, 27 y 37 renderizadas (portada, índice, tabla de hallazgos, anexo A);
  el encabezado de la portada sale del enunciado (línea 8).
  Al commitear: el PDF (2,2 MB) supera el límite de 512 KB del hook de archivos grandes; sin subir el
  límite, el PDF queda en `.gitignore`, se regenera con `make informe` y se adjunta al release en T21.
  Gitleaks marcó `const password = …` en `capturas-grafana.mjs` (valor leído en ejecución, no escrito):
  variable renombrada.
  Commit `2315f09`. Revisión nativa (medio, 2681 líneas, 1 lente): **aprobada**. Corregido después:
  el PDF final se escribe en un temporal y solo se copia si la paginación de las dos pasadas coincide;
  los rangos de líneas citados de los workflows fallan el build si quedan fuera del archivo o vacíos; el
  enmascarado de gitleaks cubre también `Line`; el anexo dice que `make informe` reinstala con `npm ci`.
  Sugerencias no aplicadas: limpieza con try/finally, índice por subcadena, espera fija en las capturas.
  **Hallazgo nuevo fuera de alcance (T19):** el dashboard «Identity Hub — Seguridad» tiene paneles sin
  datos: `identity_login_attempts_total`, `identity_events_published_total` e
  `identity_refresh_reuse_detected_total` están definidas en `backend/internal/observability/metrics.go`
  pero ningún código las incrementa (Claude contó 0 usos fuera de pruebas); el panel de DLQ consulta una
  etiqueta `queue` que RabbitMQ no expone. El informe muestra la captura real y lo explica. Afecta al
  bono de observabilidad (+8 %) y al video; sin id; falta decidir la tarea.
  **Corregido (2026-10-06, por decisión del usuario)** — Ruta: delegada (Codex), revisión de Claude.
  `Broker.Publish` cuenta `identity_events_published_total` (`published`/`failed`, un incremento por
  intento); el reuso de refresh incrementa `identity_refresh_reuse_detected_total`; los handlers de
  login y verificación MFA cuentan `mfa_required`, `failed`, `locked` y `succeeded` (no cuentan los
  rechazos por límite de IP); Prometheus raspa `/metrics/detailed` de RabbitMQ (`queue_coarse_metrics`)
  y el panel de DLQ usa `rabbitmq_detailed_queue_messages`. RED observado por Claude (3 pruebas
  TestRNF007_ fallan contra el código previo; `go.mod` necesitó `go-spew` indirecto para `testutil`,
  resuelto por Claude con red). GREEN (Codex): build, vet, `go test ./...`, golangci-lint 0, trazabilidad
  al día. Verificación de Claude con el stack real tras `make e2e` 52/52: login 76 `mfa_required`,
  69 `succeeded`, 3 `failed`; 204 eventos `published`; 2 reusos; serie de `notifications.dlq` presente.
  Limitación aceptada: el intento que provoca el bloqueo se cuenta como `failed` (el dominio devuelve
  credenciales inválidas); los siguientes, como `locked`.
  Commit `fa6d307`. Revisión nativa (alto, 286 líneas, 4 lentes): **aprobada**. Corregido después:
  el contador de reuso sube solo tras el commit de la revocación (R3-001/R4), comentarios en el handler
  de login sobre el límite de IP y el intento que bloquea (R2), y `Publish` sin retorno con nombre.
  Sugerencias no aplicadas: constantes para las etiquetas, prueba de serialización duplicada.
  iCloud volvió a reemplazar 7 capturas del manual por versiones viejas y a crear copias « N»: se
  restauraron desde git (`6496a92`) y las copias se movieron fuera del repo, sin borrarlas. Causa
  (aclarada por el usuario): la sincronización continua del Mac con iCloud al reescribir archivos
  generados, no ediciones simultáneas desde otro equipo.
- [x] **T20 — Guion del video** de 10-15 minutos (ciclo completo: app, pipeline, despliegue,
  observabilidad) (Q3).
  Ruta: delegada (Codex), revisión de Claude. Evidencia (2026-10-06): `docs/video/guion.md`, 10 escenas
  en 13:00 (apertura, arquitectura, flujo del empleado, historia RF-021 del auditor, pipeline con
  VULN-019 de 710 a 108 y el hook de gitleaks en clip pregrabado, despliegue local, producción simulada
  con T17 condicional, Terraform sin apply, observabilidad y cierre), reparto sugerido entre los cinco
  integrantes, checklist de preparación (`make setup` antes de `make up-obs`, sin mostrar `.env`) y plan
  de contingencia. Verificación (Codex): 22 enlaces relativos existen, los objetivos de make citados
  existen, tiempos 13:00. Revisión de Claude: escenas 1 a 5 leídas; `docs/README.md` conserva el guion
  de la demo en vivo (`guion-demo.md`, ~5 min) junto al del video. Pendiente: grabar (Q3).
- [ ] **T21 — Cierre.** Bitácora, matriz, informe de seguridad final, tag de versión y PR.
  Decisión del usuario (2026-10-06): dejar todo terminado y verificado antes de Docker Hub; T17, el tag
  `vX.Y.Z` y el release (con el PDF del informe adjunto) se hacen al final, en vivo frente al equipo.
  - [x] **T21a — Codificación de los correos.** `buildRawMessage` declara `MIME-Version`,
    `Content-Type: text/plain; charset=utf-8` y codifica el asunto (RFC 2047); prueba primero.
    Ruta: delegada (Codex), revisión de Claude. Evidencia (2026-10-06): RED con tres TestRF012_ (faltaba
    `MIME-Version` y un CR/LF en el asunto inyectaba una cabecera `Bcc`); GREEN con `MIME-Version`,
    `charset=utf-8`, `Content-Transfer-Encoding: 8bit` (quoted-printable cambiaba LF por CRLF en el
    cuerpo), asunto con `mime.QEncoding`, CR/LF eliminados de las cabeceras y destinatario con CR/LF
    rechazado en `notify.Render`. Build, vet, `go test ./...`, golangci-lint 0 y matriz al día (Codex).
    Claude: worker reconstruido y correo real en Mailpit con las tres cabeceras. La inyección no era
    explotable desde fuera (el correo del destinatario se valida al crear la cuenta y los asuntos son
    fijos), así que no se abre un VULN; queda como defensa en profundidad.
  - [ ] **T21b — Documentación de cierre.** Entrada de la semana 4 en `docs/BITACORA.md`, versión final
    de `docs/security-report.html` (VULN-019 remediado), coherencia de `docs/README.md`, README raíz y
    matriz; informe técnico regenerado con el docente (Jaider Ospina Navas).
  - [ ] **T21c — PR de la fase 4** (con confirmación del usuario). Tag, release y T17 quedan para el final.

## Progreso

| Fase | Tareas | Hechas |
|---|---|---|
| 1 — Documentación, UML e integración | T1 a T6 (6) | 6 (T1 a T6) — fase cerrada |
| 2 — IaC de referencia | T7 a T10 (4) | 4 (T7 a T10) — fase cerrada |
| 3 — Grilla de roles configurable | T11 a T15 (5) | 5 (T11 a T15) — fase cerrada |
| 4 — Publicación y entrega | T16 a T21 (6) | 4 (T16, T18, T19 y T20) |
| **Total** | **21** | **19** (T1 a T16 y T18 a T20) |

## Siguiente paso

**2026-10-06.** Fases 1 a 3 cerradas y en main (PR #11, merge `8b4a18a`, CI 21/21). T16 hecha
(VULN-019 remediado, capturas de Desktop incluidas). T17 pospuesta al final (Q1). T18 a T20 hechas, más la corrección de
las métricas del dashboard. Siguiente: T21a a T21c; al final, en vivo, T17 con tag y release.
Codex: cuota diaria limitada; Sonnet como respaldo.

## Cambios de spec propuestos

La definición aprobada está en `specs/01-requirements.md` (RF-021) y
`specs/adr/0013-roles-y-permisos-configurables-por-aplicacion.md`; los fragmentos para actualizar
OpenAPI en T12 están en la sección «Cambios de OpenAPI propuestos» de esa ADR.

## Notas de handoff Codex

—
