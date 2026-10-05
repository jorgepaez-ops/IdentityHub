# 0012 — Arquitectura de referencia en AWS, validada con LocalStack

Estado: propuesta · 2026-10-05

## Contexto

El enunciado pide despliegue automatizado con IaC (Terraform o Ansible) y escaneo con Checkov
(`ENUNCIADO-TRABAJO-FINAL.md`, líneas 96, 146, 166 y 278). El proyecto no tiene presupuesto para
una nube real (D3 de `odd/tasks/idp-semana-4.md`), pero el usuario tiene el plan Student de
LocalStack (D6), con la cobertura de servicios del plan Ultimate. La investigación de T7 (fuentes
en el archivo de tareas) muestra que en ese plan **ejecutan de verdad** ECS Fargate (contenedores
en el Docker local), RDS PostgreSQL y el ALB; Secrets Manager, KMS, CloudWatch Logs, IAM y SES se
emulan; ACM, Route 53, WAFv2 y los security groups son solo CRUD. **Amazon MQ para RabbitMQ no
está soportado.**

La portabilidad sigue mandando: quien clone el repositorio levanta el proyecto con
`make setup && docker compose up -d`. La IaC es un ejemplo de producción, no un requisito para
ejecutar el proyecto.

## Decisión

Una arquitectura de producción de referencia en **AWS**, escrita en **Terraform** en
`infraestructura/terraform/`, que reproduce la topología del compose con servicios gestionados
donde LocalStack permite probarlos:

| Pieza del compose | En AWS | Se valida en LocalStack |
|---|---|---|
| `web` (Nginx con las dos SPA) | Servicio ECS Fargate detrás de un **ALB** con reglas por `Host` (`identityhub.<dominio>` y `contabilidad.<dominio>`); el ALB redirige HTTP a HTTPS | Sí (ECS y ALB reales) |
| `api`, `worker` | Servicios ECS Fargate en subredes privadas | Sí |
| `db` | **RDS PostgreSQL**, cifrado con KMS, sin acceso público, con backups y protección contra borrado | Sí (Postgres real) |
| `broker` | **RabbitMQ autogestionado en ECS Fargate** (D7), no Amazon MQ | Sí |
| `migrate` | Tarea ECS de un solo uso antes de desplegar `api` | Sí |
| `mailpit` | **SES** (interfaz SMTP) en producción; Mailpit en ECS solo cuando `var.localstack = true` | Parcial (SES emulado) |
| `.env` | **Secrets Manager** (contraseñas, semilla JWT) referenciado por las task definitions; nunca en variables de Terraform ni en el estado en claro | Sí (emulado) |
| Logs de contenedores | **CloudWatch Logs** con retención y cifrado KMS | Sí (emulado) |
| Imágenes | **Docker Hub por digest** (ADR 0011); no se usa ECR | — |
| TLS, DNS, WAF | **ACM**, **Route 53** y **WAFv2** asociados al ALB | No: solo CRUD; marcados en el código como no aplicados en local |

Redes: una VPC con subredes públicas (solo el ALB) y privadas (ECS y RDS) en dos zonas de
disponibilidad, y security groups de mínimo privilegio (ALB → `web` → `api` → `db`/`broker`;
`worker` → `broker`/`db`). Los nombres de servicio del compose (`api`, `broker`, `db`) se
resuelven con **Cloud Map** (DNS privado); si LocalStack no lo soporta, T8 lo documenta y usa
la alternativa que funcione.

Validación:
- `terraform fmt -check` y `terraform validate` en CI.
- **Checkov** en CI sobre `infraestructura/`, los Dockerfiles y los workflows (T10).
- `terraform apply` contra LocalStack con `tflocal` (que genera un override de endpoints, así el
  código de producción no lleva endpoints locales), mediante un objetivo `make` local. El token
  `LOCALSTACK_AUTH_TOKEN` vive solo en el entorno del usuario.
- El estado de Terraform es local y no se versiona; el backend remoto (S3 con bloqueo) queda
  descrito, no configurado.

## Consecuencias

- **Una desviación de lo "gestionado":** RabbitMQ en ECS en lugar de Amazon MQ. Es lo que permite
  probar toda la arquitectura en local; el informe lo justifica. En una nube real convendría
  evaluar Amazon MQ.
- **El correo real exige cambiar código.** El worker envía por SMTP sin autenticación ni TLS
  (`backend/cmd/worker/main.go:198-199`, pensado para Mailpit). La interfaz SMTP de SES exige
  credenciales y STARTTLS. Hasta que el worker los soporte, la referencia usa SES solo como
  destino documentado.
- **Lo que no se valida en local:** TLS (ACM), DNS público (Route 53), filtrado de tráfico (WAFv2)
  y reglas de security groups. Terraform los aplica sin error en LocalStack, pero no prueban
  nada; el código los marca y el informe lo dice.
- **Checkov va a señalar simplificaciones** (por ejemplo, logs de acceso del ALB o réplicas de RDS).
  Cada excepción se documenta con `skip` y su justificación, nunca con `soft_fail`.
- **Dependencia de una licencia personal.** El `apply` local requiere el token del plan Student;
  `validate` y Checkov no. Si la licencia vence, la IaC sigue siendo válida y escaneada.
- **Ejecutar LocalStack en CI queda pendiente de decisión del usuario** (exige guardar el token
  como secreto del repositorio).
