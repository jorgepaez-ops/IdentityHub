# Despliegue local con Docker Compose

La topología representa el archivo Compose actual: todos los servicios usan la red predeterminada `identity-hub_default` con subred `172.28.0.0/16`; los componentes de observabilidad se habilitan mediante el perfil `observability`.

```mermaid
flowchart TB
    Host["Host de desarrollo"]
    Network["Red identity-hub_default\n172.28.0.0/16"]
    Web["web\nNginx\nHost: 8080"]
    API["api\nHost: 8081"]
    Worker["worker\nHost: 9091 (/metrics)"]
    Migrate["migrate\nEjecución única"]
    DB[("db: PostgreSQL\nHost: 5432\nVolumen: pgdata")]
    Broker[("broker: RabbitMQ\nHost: 5672 (AMQP), 15672 (administración)\nVolumen: rabbitdata")]
    Mailpit["mailpit\nHost: 1025 (SMTP), 8025 (web/API)"]
    Prometheus["prometheus (perfil observability)\nHost: 9090"]
    Loki["loki (perfil observability)\nHost: 3100"]
    Alloy["alloy (perfil observability)\nSin puerto publicado"]
    Grafana["grafana (perfil observability)\nHost: 3000\nVolumen: grafanadata"]
    PgInit["Bind mount: postgres-init/01-identity-app-role.sh (solo lectura)"]
    Migrations["Bind mount: db/migrations (solo lectura)"]
    PrometheusConfig["Bind mount: observability/prometheus.yml (solo lectura)"]
    AlloyConfig["Bind mounts: alloy.river y docker.sock (solo lectura)"]
    GrafanaConfig["Bind mount: observability/grafana (solo lectura)"]
    RuntimeTmpfs["tmpfs: /tmp (api, worker y web)"]

    Host -->|"identityhub.localhost:8080"| Web
    Host -->|"contabilidad.localhost:8080"| Web
    Host -->|"8081"| API
    Host -->|"5432"| DB
    Host -->|"5672, 15672"| Broker
    Host -->|"1025, 8025"| Mailpit
    Host -->|"9091"| Worker
    Host -->|"9090"| Prometheus
    Host -->|"3100"| Loki
    Host -->|"3000"| Grafana

    subgraph Compose["Docker Compose: identity-hub"]
        Network
        Web
        API
        Worker
        Migrate
        DB
        Broker
        Mailpit
        Prometheus
        Loki
        Alloy
        Grafana
    end

    Web --- Network
    API --- Network
    Worker --- Network
    Migrate --- Network
    DB --- Network
    Broker --- Network
    Mailpit --- Network
    Prometheus --- Network
    Loki --- Network
    Alloy --- Network
    Grafana --- Network
    API --> DB
    API --> Broker
    Worker --> Broker
    Worker --> Mailpit
    Migrate --> DB
    Prometheus --> API
    Prometheus --> Worker
    Alloy --> Loki
    Grafana --> Prometheus
    Grafana --> Loki
    PgInit --> DB
    Migrations --> Migrate
    PrometheusConfig --> Prometheus
    AlloyConfig --> Alloy
    GrafanaConfig --> Grafana
    RuntimeTmpfs --> API
    RuntimeTmpfs --> Worker
    RuntimeTmpfs --> Web
```

La referencia de despliegue en nube para producción se incorpora en las tareas T7–T8; este diagrama no la representa como infraestructura actual.

Fuente: `deploy/docker-compose.yml`, `frontend/Dockerfile`, `frontend/nginx/default.conf` y `odd/tasks/idp-semana-4.md`.
