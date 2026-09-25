# VULN-001 — Credenciales incrustadas en el código y en la orquestación

| | |
|---|---|
| **Severidad** | HIGH |
| **Estado** | en remediación |
| **Detectado por** | Gitleaks · job `secrets` · gosec G101 · job `lint` |
| **Componente** | `backend/internal/api/legacy_auth.go:38-45`, `deploy/docker-compose.yml`, `backend/Dockerfile` |
| **Amenaza** | AM-012 (divulgación de información) |
| **Sembrada** | sí |
| **Evidencia antes** | |
| **Commit de remediación** | `51a7a4f` (T23, solo el componente `legacy_auth.go`; `docker-compose.yml` y `Dockerfile` siguen abiertos, remedia otra tarea) |
| **Evidencia después** | |
| **Run de Actions (antes/después)** | |

## Evidencia

Seis secretos en tres archivos distintos:

| Ubicación | Secreto | Regla que lo detecta |
|---|---|---|
| `legacy_auth.go` | `legacySigningKey` | gosec G101, gitleaks generic |
| `legacy_auth.go` | `legacyAWSKey` / `legacyAWSSecret` | gitleaks `aws-access-token` |
| `legacy_auth.go` | `legacySlackToken` | gitleaks `slack-bot-token` |
| `docker-compose.yml` | `POSTGRES_PASSWORD`, `RABBITMQ_DEFAULT_PASS` | gitleaks generic |
| `Dockerfile` | `ENV DB_PASSWORD`, `ENV API_SIGNING_KEY` | trivy secret, gitleaks |

Los valores son inventados y no dan acceso a nada; lo que se demuestra es la
**detección**, no la fuga.

## Por qué importa en esta aplicación

Tres consecuencias distintas, y la tercera es la que la gente subestima:

1. La clave de firma incrustada permite a cualquiera con acceso al repositorio
   emitir tokens válidos: control total sobre la autenticación.
2. `ENV` en un Dockerfile **persiste en el manifiesto de la imagen**. Cualquiera
   que pueda hacer `docker pull` lee el secreto con `docker history`, sin
   siquiera arrancar el contenedor.
3. Git no olvida. Borrar el secreto en un commit posterior no lo elimina del
   historial, y el secreto sigue siendo válido hasta que se rote. Por eso el job
   `secrets` usa `fetch-depth: 0` y no solo el último commit: escanear la punta
   de la rama da una falsa sensación de limpieza.

## Remediación

- Toda credencial pasa a `config.Load()`, que **falla el arranque** si falta
  (RNF-003). Sin valores por defecto.
- El compose lee de `.env`, que está en `.gitignore`; se versiona `.env.example`
  sin valores.
- Se elimina `legacy_auth.go` entero.
- En CI las credenciales vienen de GitHub Secrets.
- Se instala el gancho de pre-commit de Gitleaks para que el siguiente secreto
  no llegue a commitearse.
