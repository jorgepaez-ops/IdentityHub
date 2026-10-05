# Secuencia de inicio de sesión con MFA por correo

El flujo separa la validación de contraseña de la verificación MFA: la primera crea y envía el desafío, mientras que la segunda emite los tokens y establece la cookie de refresh.

```mermaid
sequenceDiagram
    participant Browser as Navegador
    participant Web as web / Nginx
    participant API as api
    participant DB as PostgreSQL
    participant MQ as RabbitMQ
    participant Worker as worker
    participant Mailpit as Mailpit

    Browser->>Web: POST /api/v1/auth/login (email, password)
    Web->>API: Proxy de autenticación
    API->>DB: Cuenta fallos por IP y busca usuario
    alt Cuenta ya bloqueada o límite por IP
        API-->>Web: 423
        Web-->>Browser: Inicio de sesión bloqueado
    else Contraseña inválida
        API->>DB: Registra login_failed en audit_log
        opt Se alcanza el umbral de cuenta
            API->>DB: Bloquea cuenta y registra account_locked
            API->>MQ: Publica security.account_locked
            MQ->>Worker: Entrega evento
            Worker->>Mailpit: Envía notificación por SMTP
        end
        API-->>Web: 401
        Web-->>Browser: Credenciales no autorizadas
    else Contraseña válida
        API->>DB: Crea desafío en mfa_challenges y audita emisión
        API->>MQ: Publica security.mfa_challenge_issued
        MQ->>Worker: Entrega evento
        Worker->>Mailpit: Envía código MFA por SMTP
        API-->>Web: 202 con mfaToken
        Web-->>Browser: Desafío MFA

        Browser->>Web: POST /api/v1/auth/mfa/verify (mfaToken, code)
        Web->>API: Proxy de autenticación
        API->>DB: Lee y consume mfa_challenges
        alt Código MFA incorrecto
            API->>DB: Registra mfa_code_rejected y evalúa bloqueo
            opt Se alcanza el umbral de cuenta
                API->>DB: Bloquea cuenta y registra account_locked
                API->>MQ: Publica security.account_locked
                MQ->>Worker: Entrega evento
                Worker->>Mailpit: Envía notificación por SMTP
            end
            API-->>Web: 401
            Web-->>Browser: Código no autorizado
        else Código MFA correcto
            API->>DB: Crea refresh_tokens y hub_sessions, audita login_succeeded
            API-->>Web: 200 con accessToken y Set-Cookie refresh_token
            Web-->>Browser: Access token y cookie HttpOnly de refresh
        end
    end
```

Fuente: `backend/internal/api/login.go`, `backend/internal/api/mfa.go`, `backend/internal/auth/login/login.go`, `backend/internal/auth/mfa/mfa.go`, `backend/cmd/api/main.go`, `backend/cmd/worker/main.go`, `backend/internal/notify/notify.go`, `db/migrations/000001_initial_schema.up.sql`, `db/migrations/000006_mfa_challenges.up.sql`, `db/migrations/000007_oauth_authorization_code.up.sql`, `deploy/docker-compose.yml` y `frontend/nginx/default.conf`.
