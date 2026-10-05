# Manual de seguridad

La fuente de amenazas es [`specs/05-security/threat-model.md`](../../specs/05-security/threat-model.md); el modelo navegable y sus DFD nivel 0/1 están en [`docs/threat-model/`](../threat-model/). El pipeline existe para detectar, justificar y remediar riesgos reales, incluida la línea base vulnerable deliberada de la [ADR 0007](../../specs/adr/0007-linea-base-vulnerable-deliberada.md).

## Controles por fase del CI

| Fase o job de `ci.yml` | Herramienta | Qué detecta | Umbral real |
|---|---|---|---|
| 2 · Lint y tipos | `gosec` vía `golangci-lint` | Patrones inseguros en Go | Falla ante hallazgos de `golangci-lint`; no hay umbral de severidad separado configurado. |
| 2 · Lint y tipos | Hadolint | Problemas de Dockerfile | `failure-threshold: warning`. |
| 3 · Secretos en el historial | Gitleaks | Secretos en todo el historial | El action falla con los hallazgos que reporta; no se configura un umbral numérico. |
| 4 · SAST | CodeQL | Consultas `security-extended` para Go y JavaScript/TypeScript | Publica resultados SARIF; el workflow no declara un umbral de severidad adicional. |
| 4b · Semgrep (OWASP) | Semgrep | Reglas OWASP Top Ten, Go y React | El informe SARIF se publica y el gate falla con `ERROR`. |
| 5 · Dependencias vulnerables | `govulncheck` | Vulnerabilidades Go alcanzables | El comando no define umbral numérico; un fallo del análisis falla el job. |
| 5 · Dependencias vulnerables | `npm audit` | Dependencias de frontend y Contabilidad | `--audit-level=high`. |
| 5 · Dependencias vulnerables | osv-scanner | Vulnerabilidades de dependencias recursivas, incluidas no invocadas | `--recursive --all-vulns`; no hay un umbral de severidad configurado. |
| 8 · Configuración de contenedores | Trivy config | Misconfiguraciones de configuración e infraestructura | Gate en `HIGH,CRITICAL`; primero publica SARIF y luego falla si las encuentra. |
| 9-10 · Construir y escanear imágenes | Trivy image | CVE en `api`, `worker` y `web` | Gate en `HIGH,CRITICAL` corregibles con `ignore-unfixed: true`. |
| 11 · DAST | OWASP ZAP | Baseline del Hub y Contabilidad y API basada en OpenAPI | `scripts/zap-gate.py` falla ante riesgo medio o alto, salvo `IGNORE` justificado. |
| 7 · E2E | Playwright | Flujos de navegador contra el stack levantado | Cualquier prueba fallida falla el job; informe y trazas se suben al fallar o cancelar. |
| **Externo — no es job de `ci.yml`** | SonarCloud | Análisis de calidad y seguridad del código nuevo; la evidencia registrada incluye reglas S6505/S8543 y S2068. | Revise el resultado de SonarCloud asociado al PR y sus hallazgos. El archivo `ci.yml` no configura este análisis ni declara un umbral general; el historial solo registra que el PR #9 fue bloqueado por una calificación de seguridad del código nuevo de 3 > 1. |

Los SARIF de Semgrep y Trivy se cargan a Code Scanning. Los artefactos de Playwright y ZAP se conservan cuando sus jobs los generan; su retención está definida en el workflow. SonarCloud es un gate externo documentado por el historial del repositorio: interprete su Quality Gate junto con los hallazgos del PR, pero no lo confunda con una definición dentro de `.github/workflows/ci.yml`.

## Lectura local y en CI

- `make scan-secrets` ejecuta Gitleaks sobre el historial.
- `make scan-deps` ejecuta `govulncheck` y `npm audit`; sus salidas locales terminan con `|| true`, por lo que debe leerse la salida y no solo el código de salida.
- `make scan-config` ejecuta Trivy config y Hadolint; `make scan-image` construye y escanea las imágenes; `make scan` agrupa los cuatro objetivos.
- `make scan-dast` ejecuta ZAP contra un stack ya levantado y deja informes JSON/HTML en `security/zap-reports/`, directorio ignorado por Git.
- En CI, consulte los SARIF en Code Scanning y descargue los artefactos `playwright-<run>` o `zap-reports-<run>` cuando el workflow los publique.

## Gestión de vulnerabilidades y evidencia

1. Un hallazgo se documenta en `security/findings/VULN-NNN-*.md` con severidad, componente, amenaza, evidencia y estado.
2. El identificador `VULN-NNN` lo asigna únicamente el usuario al crear la ficha; no se inventa para un hallazgo incidental.
3. La tarea de remediación actualiza la ficha con el commit y conserva evidencia antes/después según el protocolo de [`AGENTS.md`](../../AGENTS.md) y de las tareas ODD.
4. Las capturas externas se referencian desde `docs/evidencia/VULN-XXX/evidencia.json`; el repositorio no sustituye esa captura por una afirmación sin fuente.
5. Un hallazgo fuera de alcance se registra para decisión posterior; no se corrige silenciosamente ni se debilita un gate.

## Excepciones y reglas

| Archivo | Política actual |
|---|---|
| [`.gitleaksignore`](../../.gitleaksignore) | Huellas de falsos positivos conocidos en el historial; no contiene secretos ni habilita una exclusión amplia. |
| [`security/.trivyignore`](../../security/.trivyignore) | Vacío a propósito. Toda excepción futura exige CVE, justificación, responsable, expiración y ficha asociada. |
| [`.zap/rules.tsv`](../../.zap/rules.tsv) | `WARN` documenta alertas bajas aceptadas; solo `IGNORE` con justificación puede suprimir una alerta media o alta. |
| [`backend/osv-scanner.toml`](../../backend/osv-scanner.toml) | Riesgo aceptado de VULN-028: ignora GO-2026-5932 (`golang.org/x/crypto/openpgp`, sin versión corregida y no alcanzable) hasta el 2026-12-25, con la justificación en el propio archivo y en la ficha. |

## Hook local

La defensa previa al commit se incorporó en T16: instale el hook con `pre-commit install`. Ejecuta controles de formato, claves privadas, Gitleaks, build de Go y trazabilidad según [`.pre-commit-config.yaml`](../../.pre-commit-config.yaml). No reemplaza los gates de CI.
