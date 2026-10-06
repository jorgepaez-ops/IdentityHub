# Secuencia del SSO con Authorization Code y PKCE

Contabilidad delega su inicio de sesión en Identity Hub con OAuth 2.0 Authorization Code y PKCE S256 ([ADR 0009](../../../specs/adr/0009-sso-entre-dominios-con-authorization-code-y-pkce.md)). Contabilidad es un cliente público: no tiene secreto, genera un `code_verifier` y nunca ve la contraseña. Si no hay cookie `hub_session`, el Hub pide contraseña y código MFA por correo antes de emitir el código de autorización, de un solo uso y con vigencia de un minuto. El canje devuelve un JWT firmado con EdDSA (`aud`, `roles` y `permissions`) que la SPA verifica con el JWKS del Hub.

```mermaid
sequenceDiagram
    autonumber
    actor U as Usuario (navegador)
    participant C as Contabilidad SPA
    participant W as web / Nginx (Hub)
    participant A as api (Hub)
    participant M as RabbitMQ, worker y Mailpit

    U->>C: Pulsa Continuar
    C->>C: Genera code_verifier y state, calcula code_challenge = BASE64URL(SHA-256(verifier)) y los guarda en sessionStorage
    C->>W: Redirige a GET /oauth/authorize (client_id, redirect_uri, response_type=code, state, code_challenge, code_challenge_method=S256)
    W->>A: Proxy de /oauth/
    A->>A: Valida client_id, redirect_uri exacto, state y PKCE S256

    alt Sin cookie hub_session válida
        A-->>U: 302 a /login con continue igual a la ruta de authorize
        U->>W: POST /api/v1/auth/login (correo y contraseña)
        W->>A: Proxy de /api/
        A->>M: Publica security.mfa_challenge_issued
        M-->>U: Código MFA por correo
        A-->>U: 202 con mfaToken
        U->>W: POST /api/v1/auth/mfa/verify (mfaToken y código)
        W->>A: Proxy de /api/
        A-->>U: 200 con Set-Cookie hub_session
        U->>W: Vuelve a GET /oauth/authorize
        W->>A: Proxy de /oauth/
    end

    A->>A: Emite code aleatorio de un solo uso (1 minuto) y guarda su hash y el code_challenge
    A-->>U: 302 a redirect_uri con code y state
    U->>C: GET /oauth/callback?code&state
    C->>C: Quita code y state de la URL y compara state

    C->>A: POST /oauth/token con CORS (grant_type, code, redirect_uri, client_id, code_verifier)
    A->>A: Consume el código en una transacción y exige SHA-256(code_verifier) = code_challenge
    A-->>C: 200 con access_token (JWT EdDSA), token_type Bearer y expires_in 900

    C->>A: GET /.well-known/jwks.json
    A-->>C: Clave pública Ed25519 con su kid
    C->>C: Verifica firma EdDSA, iss, aud y exp, y lee roles y permissions

    Note over C,A: El token vive solo en memoria. Al vencer (15 min) la SPA repite el flujo y, con hub_session viva, no se pide login.
```

Fuente: `docs/manuales/integracion-terceros.md`, `backend/internal/auth/oauth/oauth.go`, `backend/internal/api/oauth.go`, `backend/internal/auth/token/token.go`, `contabilidad/src/auth/` y `frontend/nginx/default.conf`. Vista visual equivalente: [`02-flujo-sso.html`](../02-flujo-sso.html).
