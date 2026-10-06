# Estados del ciclo de vida de una cuenta

Una cuenta nace por alta administrativa en `pending_verification` y pasa a `active` cuando la persona acepta la invitación recibida por correo y elige su contraseña. Cinco fallos de contraseña o de código MFA en 15 minutos la bloquean (`locked`); el bloqueo se levanta al vencer `locked_until`, al confirmar un restablecimiento de contraseña o por decisión de un administrador, que también puede deshabilitar (`disabled`) y habilitar la cuenta. Solo `active` obtiene sesión. Los roles del directorio (`user` y `admin`) y los de aplicación (grilla de roles, ADR 0013) son ortogonales al estado: no lo cambian.

```mermaid
stateDiagram-v2
    [*] --> pending_verification: alta administrativa e invitación por correo

    pending_verification --> pending_verification: admin reenvía la invitación
    pending_verification --> active: acepta la invitación

    active --> locked: 5 fallos en 15 min
    locked --> active: vence locked_until
    locked --> active: restablece la contraseña
    locked --> active: admin lo pasa a active

    active --> disabled: admin deshabilita
    disabled --> active: admin habilita

    note right of pending_verification
        No inicia sesión. La invitación vence a las 24 h
        y la cuenta sigue pendiente; no hay purga automática.
        Restablecer la contraseña no la activa.
    end note

    note right of active
        Roles del directorio: user (siempre) y admin.
        Roles de aplicación (p. ej. contabilidad.senior)
        asignados por un admin desde la grilla.
        Siempre debe quedar al menos un admin activo.
    end note

    note left of disabled
        No inicia sesión; el restablecimiento
        de contraseña no la reactiva.
    end note
```

Fuente: `backend/internal/auth/{employee,invitation,invitationresend,login,lockout,passwordreset,admin}`, `db/queries/users.sql`, `specs/02-domain-model.md` y `specs/03-api/openapi.yaml`. Vista visual equivalente: [`03-ciclo-de-vida-usuario.html`](../03-ciclo-de-vida-usuario.html).
