# Casos de uso de Identity Hub

Mermaid no incorpora un tipo nativo para casos de uso; por eso este diagrama usa un flujo con la notación habitual: actores fuera de los límites de cada sistema, casos de uso como óvalos dentro, relaciones «include» y «extend» punteadas, y los casos de uso que amplían a otro, con borde discontinuo. El administrador es también un empleado: hereda sus casos de uso.

```mermaid
flowchart LR
    Employee(["👤 Empleado"])
    Admin(["👤 Administrador"])
    Client(["🖥 Aplicación cliente: Contabilidad"])
    Mail(["✉ Sistema de correo"])
    Siem(["🖥 Integración máquina: SIEM o sistema de auditoría"])

    subgraph Hub["Sistema: Identity Hub"]
        direction TB
        C2(["Aceptar invitación"])
        C3(["Iniciar sesión con correo y MFA"])
        C6(["Bloquear por fuerza bruta"])
        C4(["Restablecer contraseña"])
        C5(["Consultar y revocar sesiones activas"])
        C9(["Autorizar SSO: /oauth/authorize"])
        C10(["Canjear código: /oauth/token"])
        C1(["Crear empleado e invitar"])
        C7(["Gestionar usuarios y roles"])
        C8(["Consultar registro de auditoría"])
        C12(["Grilla configurable de roles y permisos"])
        C13(["Exportar registro de auditoría: CSV o JSON"])
        C14(["Emitir y revocar claves de servicio"])
        C15(["Leer registro de auditoría con clave de servicio"])
    end

    subgraph App["Sistema: Contabilidad"]
        C11(["Mostrar vistas según rol"])
    end

    Admin -. "es un" .-> Employee

    Employee --- C2
    Employee --- C3
    Employee --- C4
    Employee --- C5
    Employee --- C9
    Employee --- C11
    Admin --- C1
    Admin --- C7
    Admin --- C8
    Admin --- C12
    Admin --- C13
    Admin --- C14

    C3 -. "«include»" .-> C6
    C9 -. "«include»" .-> C3
    C11 -. "«include»" .-> C10
    C12 -. "«extend»" .-> C7
    C13 -. "«extend»" .-> C8
    C15 -. "requiere una clave emitida en" .-> C14

    Client --- C10
    Siem --- C15
    C1 --- Mail
    C3 --- Mail
    C4 --- Mail

    classDef actor fill:#eef2ff,stroke:#2d4f8f,color:#1f2937,stroke-width:2px;
    classDef ext fill:#fff3cd,stroke:#9a6700,color:#5c3b00,stroke-width:2px,stroke-dasharray: 5 5;
    class Employee,Admin,Client,Mail,Siem actor;
    class C12,C13 ext;
```

Fuente: `specs/03-api/openapi.yaml`, `backend/cmd/api/main.go`, `backend/internal/api/server.go`, `backend/internal/auth/login/login.go`, `backend/internal/auth/mfa/mfa.go`, `backend/internal/auth/session/session.go`, `backend/internal/auth/oauth/oauth.go`, `backend/internal/auth/auditlog/`, `backend/internal/auth/servicekey/`, `specs/adr/0014-claves-de-servicio-para-integraciones-maquina.md` y `contabilidad/src/auth/flow.ts`.
