# deploy

Todo lo necesario para levantar y observar el sistema con Docker Compose.

- `docker-compose.yml`: el stack de desarrollo (aplicación, PostgreSQL, RabbitMQ, Mailpit y observabilidad). El `docker-compose.yml` de la raíz lo incluye.
- `docker-compose.prod.yml`: la producción simulada, con imágenes por digest, sin build ni puertos de desarrollo.
- `observability/`: Prometheus, Grafana (paneles y alertas), Alloy y las reglas de Falco.
- `postgres-init/`: script que crea el rol de la aplicación en la base.
- `certs/`: certificados SMTP locales que genera `make certs`; no se versionan.

```bash
make up        # stack completo, con observabilidad y Falco
make up-app    # solo la aplicación (lo usa CI)
make down      # detiene conservando los datos
```

Configuración, verificación y problemas frecuentes: [manual de despliegue y operación](../docs/manuales/despliegue-y-operacion.md). Qué hacer ante una alerta: [runbook de Falco](../docs/runbooks/falco-alerta.md). Volver al [mapa del repositorio](../README.md#mapa-del-repositorio).
