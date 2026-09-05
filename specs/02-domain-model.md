# 02 — Modelo de dominio

## Diagrama entidad-relación

```mermaid
erDiagram
    users ||--o{ user_roles         : tiene
    roles ||--o{ user_roles         : concede
    users ||--o{ refresh_tokens     : abre
    users ||--o{ verification_tokens: solicita
    users ||--o{ recovery_codes     : posee
    users ||--o{ audit_log          : origina
    refresh_tokens ||--o{ refresh_tokens : rota_a

    users {
        uuid   id PK
        citext email UK
        text   password_hash
        text   display_name
        text   status        "pending_verification|active|locked|disabled"
        bool   mfa_enabled
        bytea  mfa_secret_enc "nullable, cifrado en reposo"
        int    failed_login_count
        timestamptz locked_until "nullable"
        timestamptz created_at
        timestamptz updated_at
    }
    roles {
        uuid id PK
        text name UK "admin|user"
        text description
    }
    user_roles {
        uuid user_id PK_FK
        uuid role_id PK_FK
        uuid granted_by FK
        timestamptz granted_at
    }
    refresh_tokens {
        uuid   id PK
        uuid   user_id FK
        bytea  token_hash UK "SHA-256 del token opaco"
        uuid   family_id     "raíz de la cadena de rotación"
        uuid   parent_id FK  "nullable"
        text   status        "active|rotated|revoked"
        inet   ip
        text   user_agent
        timestamptz expires_at
        timestamptz created_at
        timestamptz last_used_at
    }
    verification_tokens {
        uuid   id PK
        uuid   user_id FK
        bytea  token_hash UK
        text   purpose "email_verification|password_reset"
        timestamptz expires_at
        timestamptz used_at "nullable"
    }
    recovery_codes {
        uuid   id PK
        uuid   user_id FK
        text   code_hash
        timestamptz used_at "nullable"
    }
    audit_log {
        bigint id PK
        uuid   actor_user_id FK "nullable: eventos anónimos"
        text   action
        text   resource_type
        text   resource_id
        inet   ip
        text   user_agent
        jsonb  metadata
        timestamptz created_at
    }
```

## Invariantes

Reglas que el sistema garantiza siempre. Cada una tiene una prueba con su nombre.

1. **Una contraseña nunca se almacena de forma recuperable.** `users.password_hash` empieza
   siempre por `$argon2id$`. No existe ninguna ruta de código que devuelva ese campo.
2. **Un token nunca se almacena en claro.** Refresh y verificación se guardan como hash; el
   valor original solo existe en la respuesta HTTP o en el correo, y una sola vez.
3. **Rotación monótona.** En una familia de refresh tokens hay como máximo un token en estado
   `active`. Cualquier intento de usar uno en estado `rotated` revoca la familia entera (RF-006).
4. **Una cuenta no activa no obtiene tokens de sesión.** Solo `status = 'active'` produce un par
   de tokens en el login.
5. **El audit log es append-only.** El rol de base de datos de la aplicación tiene `INSERT` y
   `SELECT` sobre `audit_log`, y carece de `UPDATE` y `DELETE`. Se aplica en la migración, no
   por convención.
6. **Siempre existe al menos un administrador habilitado.** Un `UPDATE` que dejaría el sistema
   sin ningún admin activo se rechaza.
7. **Los códigos de recuperación son de un solo uso.** Consumir uno fija `used_at` en la misma
   transacción que emite los tokens de sesión.

## Máquina de estados de la cuenta

```
                registro
                   │
                   ▼
        ┌──────────────────────┐
        │ pending_verification │──── token de verificación ────┐
        └──────────┬───────────┘                              │
                   │ 24 h sin verificar                        ▼
                   ▼                                    ┌──────────┐
              (purga por lote)                          │  active  │
                                                        └────┬─────┘
                        5 fallos en 15 min  ◄────────────────┤
                                │                            │ admin deshabilita
                                ▼                            ▼
                        ┌──────────────┐              ┌────────────┐
                        │    locked    │              │  disabled  │
                        └──────┬───────┘              └────────────┘
                               │ expira locked_until
                               └────────────► active
```

## Notas de diseño

- **`citext` para el correo.** La comparación insensible a mayúsculas se resuelve en la base de
  datos y no en código de aplicación, donde es fácil olvidarla y abrir un registro duplicado.
- **`family_id` en los refresh tokens** es lo que hace posible RF-006: la detección de reuso
  necesita alcanzar a todos los descendientes de una sesión comprometida con un solo `UPDATE`.
- **`mfa_secret_enc` cifrado en reposo**, no solo hasheado: el servidor necesita el secreto en
  claro para verificar cada código TOTP, así que se cifra con una clave del entorno (AES-GCM).
  Esto lo distingue de las contraseñas, que sí son irreversibles.
- **UUID v7 como claves primarias** salvo en `audit_log`, donde un `bigint` secuencial deja
  explícito el orden de inserción y hace más barata la consulta por rango temporal.
