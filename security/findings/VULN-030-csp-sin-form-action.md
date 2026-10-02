# VULN-030 — CSP sin `form-action`

| | |
|---|---|
| **Severidad** | MEDIUM |
| **Confianza** | HIGH |
| **Estado** | remediado |
| **Detectado por** | OWASP ZAP baseline (10055) · job 11 · DAST (OWASP ZAP) |
| **Componente** | `frontend/nginx/default.conf`, ambos bloques `server` (Hub y Contabilidad) |
| **Amenaza** | AM-015 (XSS que roba el token del `localStorage`) |
| **Sembrada** | no |
| **Evidencia antes** | `docs/evidencia/VULN-030/evidencia.json` — run 37055543540 |
| **Commit de remediación** | pendiente |
| **Evidencia después** | pendiente (run verde de CI posterior a la remediación) |
| **Run de Actions (antes/después)** | https://github.com/jorgepaez-ops/IdentityHub/actions/runs/37055543540 / pendiente |

## Evidencia

Salida literal de OWASP ZAP baseline en el run 37055543540 (`133aa8b`):

```text
CSP: Failure to Define Directive with No Fallback (10055)
URL: http://identityhub.localhost:8080
Count: 3
Other Info: The directive(s): form-action is/are among the directives that do not fallback to default-src.

CSP: Failure to Define Directive with No Fallback (10055)
URL: http://contabilidad.localhost:8080
Count: 3
Other Info: The directive(s): form-action is/are among the directives that do not fallback to default-src.
```

La alerta 10055 corresponde a la directiva `form-action`, que no tiene respaldo en
`default-src`. El artefacto de Actions `zap-reports-37055543540` conserva los informes del
baseline; no se registró captura para esta evidencia.

## Por qué importa en esta aplicación

Los formularios del Hub manejan inicio de sesión, MFA, contraseña e invitaciones. Sin
`form-action`, un HTML inyectado mediante XSS puede intentar enviar esos datos a un origen
externo. Contabilidad también expone un formulario; limitar las acciones de formulario a
su propio origen evita que una inyección convierta un envío normal en una exfiltración
entre orígenes. Esto complementa la CSP estricta que mitiga AM-015.

## Remediación

T15b añade `form-action 'self'` a la variable `$csp` de ambos bloques `server` en
`frontend/nginx/default.conf`. Los formularios React previenen el envío nativo y usan
solicitudes programáticas; el flujo OAuth `/oauth/authorize` es una navegación y el canje
del código usa `fetch`, por lo que no requiere permitir un `form-action` entre orígenes.

El commit de remediación y la evidencia después quedan pendientes del run de CI en verde.
