# Scripts de automatización

Este directorio reúne las herramientas que invocan el `Makefile`, los workflows de CI y el hook de
pre-commit, junto con las pruebas que las protegen. Todo está escrito con la biblioteca estándar de
Python o con shell POSIX/bash: no hay dependencias que instalar. Ejecútalos desde la raíz del repositorio.

Para correr todas las pruebas de este directorio (sin stack levantado y sin red, salvo
`docker compose config`):

```bash
make spec-drift
```

Ese objetivo ejecuta cada `*_test.py` y termina con `traceability.py --check`. En CI lo hace el job
`spec-drift`, un paso por prueba. Una prueba suelta se corre con `python3 scripts/<nombre>_test.py`.

## Herramientas

| Archivo | Qué hace y por qué existe | Quién lo llama | Entradas | Salidas y códigos de salida |
| --- | --- | --- | --- | --- |
| `setup_env.py` | Genera `.env` a partir de `.env.example` con secretos aleatorios, para que ningún secreto se versione (AM-012). | `make setup` | Argumentos `--template` (por defecto `.env.example`), `--output` (`.env`) y `--force`. Sin `--force` no sobrescribe un `.env` existente. | Escribe `.env`. 0 si termina bien. Con `--force` regenera todos los secretos y los volúmenes de PostgreSQL/RabbitMQ conservan los antiguos (ver guía de desarrollo). |
| `gen-smtp-certs.sh` | Crea una CA local y el certificado de Mailpit (SMTP con STARTTLS), más `smtp-auth.txt` con las credenciales SMTP. | `make certs` | `--force`, `--dir <directorio>` y `--env-file <.env>` (lee `SMTP_USERNAME` y `SMTP_PASSWORD`). | `deploy/certs/` (ignorado por git): `ca.pem`, `ca-key.pem`, `server.pem`, `server-key.pem`, `smtp-auth.txt`. Idempotente salvo `--force`. 2 si falta el valor de una opción. |
| `traceability.py` | Genera `specs/07-traceability.md` (matriz requisitos, escenarios y pruebas) y comprueba la correspondencia uno a uno entre escenarios Gherkin `@RF` y títulos de prueba E2E, los controles de `specs/07-controls.yaml` y el enlace `# Verificación:` de los escenarios `@RNF`. | `make gen`, `make spec-drift` (con `--check`), paso «Regenerar la matriz de trazabilidad» del job `spec-drift`, hook de pre-commit `matriz de trazabilidad` | `--check` (no escribe, falla si el archivo está desactualizado). Lee `specs/`, `e2e/tests/`, `Makefile` y `.github/workflows/ci.yml`. | Escribe la matriz. Distinto de cero y lista de errores accionables si hay deriva. |
| `zap-gate.py` | Decide el resultado del DAST: falla si algún informe JSON de ZAP trae una alerta de severidad Media o superior no suprimida con `IGNORE` justificado en `.zap/rules.tsv`. ZAP corre con `-I`; este script es el único punto de decisión. | `make scan-dast` (job `dast` de CI) | `--rules <archivo>` (por defecto `.zap/rules.tsv`) y uno o más informes JSON. | 0 la puerta pasa, 1 la puerta se rompe, 2 error de uso o de entrada. |
| `ci_changes.sh` | Job 0 de CI: clasifica el cambio como código (`code=true`) o solo documentación (`code=false`) para saltar los jobs pesados. Ante cualquier duda decide `code=true`. | Job `changes` de `ci.yml` (`bash scripts/ci_changes.sh`) | Variables `EVENT_NAME`, `PR_BASE_SHA`, `PR_HEAD_SHA`, `PUSH_BEFORE`, `PUSH_SHA`. | `code=...` en `$GITHUB_OUTPUT` y resumen en `$GITHUB_STEP_SUMMARY` (solo si están definidos). Termina en 0 una vez tomada la decisión. |
| `with-raised-login-limit.sh` | Ejecuta un comando con `LOGIN_IP_MAX_FAILURES` elevado en la API (solo el límite por IP, RF-017) y lo restaura al terminar, pase o falle. Existe porque las suites locales hacen logins fallidos a propósito desde una sola IP. | Macro `WITH_RAISED_LIMIT` del `Makefile`: `make e2e`, `make capturas`, `make scan-dast`, `make load-test` | `COMPOSE` y `LOCAL_TEST_LOGIN_IP_MAX_FAILURES` (obligatorias), más el comando y sus argumentos. | Conserva el código de salida del comando; si no puede restaurar la API sale con 1 (o el del comando si ya era distinto de cero). Al final invoca `ip-lockout-warning.sh`. La restauración corre con trap en EXIT. |
| `ip-lockout-warning.sh` | Avisa si alguna IP quedó en o sobre el límite de fallos de login tras una suite local (`audit_log` es append-only, no se puede limpiar). Solo lee. | `with-raised-login-limit.sh` | Opcionales: `E2E_DB_CONTAINER`, `E2E_DB_USER`, `E2E_DB_NAME`, `E2E_DOCKER`, `E2E_API_CONTAINER`, `LOGIN_IP_LIMIT`, `LOGIN_FAILURE_WINDOW_MINUTES`. | Mensaje por stderr/stdout. Siempre sale con 0 para no alterar el resultado de quien lo invoca. |
| `load-test.sh` | Prueba de carga de latencia de autenticación (RNF-010) con k6 en contenedor (imagen fijada por digest). Siembra una cuenta desechable con invitación pendiente, ejecuta `tests/load/auth-latency.js` y deshabilita la cuenta al terminar. Se niega a correr contra la producción simulada (`docker-compose.prod.yml`, vía `COMPOSE` o `COMPOSE_FILE`). | `make load-test` (bajo `with-raised-login-limit.sh`) y el paso de carga del job `e2e` de CI | `COMPOSE` (obligatoria); opcionales `K6_IMAGE`, `E2E_DB_USER`, `E2E_DB_NAME`, `HUB_URL`, `MAILPIT_URL`, `P95_MS`, `DURATION`. Requiere el stack levantado (`make up-app`). | 0 si se cumplen los umbrales de k6; el código de k6 si no; 2 si se detecta producción; 130 en INT y 143 en TERM. La limpieza corre en EXIT y conserva el código real. |

Ejemplos manuales:

```bash
python3 scripts/setup_env.py
make certs
python3 scripts/traceability.py --check
python3 scripts/zap-gate.py --rules .zap/rules.tsv security/zap-reports/hub.json
EVENT_NAME=pull_request PR_BASE_SHA=<sha> PR_HEAD_SHA=<sha> bash scripts/ci_changes.sh
make up-app && make load-test
```

### Gramática de `# Verificación:`

`traceability.py` lee la línea una sola vez de izquierda a derecha y admite exactamente esta gramática (el resto del texto es libre):

- una ruta de archivo (.py, .js, .ts, .sh, .yml, .yaml o .go) que debe existir;
- `(Nombre)`: clase o función que debe existir como palabra completa en ese archivo;
- `job <id>`: id de un job de ci.yml, con id = letras, dígitos, `_` y `-` (la puntuación final no es parte del id);
- `paso "<nombre>"`: paso de ci.yml con `- name:` exactamente igual;
- `e2e "<título>"`: prueba de Playwright con ese título exacto (`test(`, `test.only(` o `test.skip(`, con comillas simples, dobles o invertidas) en `e2e/tests/**/*.spec.ts`;
- `make <objetivo>`: objetivo del Makefile, suelto, entre comillas invertidas o entre comillas dobles del mismo tipo al abrir y cerrar. Un `make <palabra>` suelto siempre es referencia y debe existir, así que los `.feature` no usan «make» como prosa;
- cualquier otro texto entre comillas dobles es un error que nombra los prefijos aceptados (paso, e2e, make);
- un escenario sin ninguna referencia nombrada falla y el error lista todas las formas aceptadas.

## Pruebas

Todas usan `unittest` de la biblioteca estándar y no dejan `__pycache__` (el job `spec-drift` exige un
árbol limpio). Cada una corre como paso propio del job `spec-drift` de CI y desde `make spec-drift`.

| Archivo | Qué protege | Respalda |
| --- | --- | --- |
| `traceability_test.py` | El verificador de trazabilidad: correspondencia escenario-E2E, controles, matriz, y la gramática de `# Verificación:` (ver arriba; archivo, `(Nombre)`, `job <id>`, `paso "<nombre>"`, `e2e "<título>"`, `make <objetivo>` suelto, entre comillas invertidas o dobles; texto entre comillas sin prefijo es error), con casos negativos. | RNF-011 (deriva de specs) y el enlace de los escenarios `@RNF`. |
| `pipeline_controls_test.py` | Propiedades reales de los contenedores y del pipeline (compose renderizado, Dockerfile, workflows) con una prueba negativa por cada comprobación. Clases `Rnf001Containerization`, `Rnf002Pipeline`, `Rnf006Provenance`, `Rnf008Hardening`. | RNF-001, RNF-002, RNF-006 y RNF-008. |
| `compose_ports_test.py` | Que Prometheus, Loki y Grafana (perfil `observability`) publiquen puertos solo en `127.0.0.1`. Usa `docker compose config` con valores ficticios y sin leer un `.env` real. | Puertos de observabilidad en loopback. |
| `ci_changes_test.py` | El clasificador del job 0 sobre repositorios git desechables (documentación frente a código, fallos hacia «correr todo») y un lint de las guardas del job `image`. Clases `ClassifierTest` e `ImageJobGuardTest`. | Job 0 de CI (`changes`). |
| `zap_gate_test.py` | La puerta del DAST: umbral Medio, `IGNORE` con justificación, `WARN` que no suprime, errores de entrada. | `zap-gate.py` y `make scan-dast`. |
| `setup_env_test.py` | Que `setup_env.py` genere secretos aleatorios, no sobrescriba un `.env` existente y complete los valores no secretos que exige el compose. | `make setup`, AM-012. |
| `load_test_script_test.py` | La guarda de producción (`COMPOSE` y `COMPOSE_FILE`) y los códigos de salida de `load-test.sh` (k6 falla, INT 130, TERM 143, limpieza siempre), con `docker` sustituido por un stub en `PATH`: no necesita stack. | RNF-010, tarea S5 de `odd/tasks/seguimiento-pre-v1.md`. |
| `informe_excerpts_test.py` | Que cada directiva `{{code:...:A-B}}` de `docs/informe/informe-tecnico.md` apunte a un archivo existente y a un rango válido que empiece y termine en un límite de bloque (sin línea en blanco en los extremos ni cortes a mitad de un bloque indentado). Incluye casos sintéticos del verificador. | El informe técnico: editar un workflow desplaza las líneas sin avisar. |

Al añadir un script nuevo: documéntalo aquí, escríbele una prueba `*_test.py` con casos negativos y
engánchala a `make spec-drift` y al job `spec-drift` de `ci.yml`.
