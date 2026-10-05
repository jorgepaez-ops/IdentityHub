# Diagrama de componentes de Identity Hub

Este diagrama muestra los componentes desplegados localmente y los paquetes internos que componen la API y el procesamiento asíncrono de notificaciones.

```mermaid
flowchart LR
    Browser["Navegador"]
    Web["web: Nginx\nFrontend Hub y Contabilidad"]
    API["api: cmd/api"]
    Worker["worker: cmd/worker"]
    Store["internal/store"]
    Auth["internal/auth\nemployee, invitation, login, mfa, oauth,\npasswordreset, refresh, session, admin y auditlog"]
    Events["internal/events"]
    Notify["internal/notify"]
    Obs["internal/observability"]
    DB[("PostgreSQL")]
    Broker[("RabbitMQ")]
    Mailpit["Mailpit"]
    Prometheus["Prometheus"]
    Grafana["Grafana"]
    Loki["Loki"]
    Alloy["Grafana Alloy"]

    Browser --> Web
    Web --> API
    API --> Auth
    API --> Store
    API --> Events
    API --> Obs
    Store --> DB
    Events --> Broker
    Broker --> Worker
    Worker --> Notify
    Notify --> Mailpit
    Worker --> Obs
    Prometheus --> API
    Prometheus --> Worker
    Alloy --> Loki
    Grafana --> Prometheus
    Grafana --> Loki
```

Fuente: `specs/adr/0001-stack-y-contenerizacion.md`, `backend/cmd/api/main.go`, `backend/cmd/worker/main.go`, `backend/internal/`, `deploy/docker-compose.yml`, `deploy/observability/prometheus.yml`, `deploy/observability/alloy.river`, `frontend/Dockerfile` y `frontend/nginx/default.conf`.
