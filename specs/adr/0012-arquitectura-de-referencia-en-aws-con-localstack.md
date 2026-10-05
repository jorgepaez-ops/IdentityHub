# 0012 — Arquitectura de referencia en AWS, validada con LocalStack

Estado: aceptada · 2026-10-05

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
- ~~`terraform apply` contra LocalStack con `tflocal`~~ **Enmienda (2026-10-05, D8 del usuario):
  no se ejecuta.** Probarlo complicaba de más la entrega; en su lugar, la configuración y el
  despliegue se muestran con un diagrama (`docs/diagramas/uml/despliegue-aws.md`). El
  `README.md` de `infraestructura/terraform/` conserva cómo se haría con `tflocal`.
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
- **Brechas del código frente a un despliegue real** (encontradas al escribir el Terraform, T8a):
  - Los hosts `*.localhost` y el `redirect_uri` de OAuth están fijos en
    `backend/internal/config/config.go:66-68` y en `frontend/nginx/default.conf`; con dominios
    reales hay que volverlos configurables.
  - Detrás del ALB, Nginx reemplaza `X-Forwarded-For` con la IP del ALB, así que el límite de
    fallos por IP (RF-017) contaría a todos los clientes como uno solo: un atacante podría
    bloquear el login de todos. Antes de producción, Nginx debe confiar en el ALB y propagar la
    IP del cliente (`set_real_ip_from` con la subred del ALB) y `TRUSTED_PROXIES` debe incluirla.
  - El rol `identity_app` recibe su contraseña en `deploy/postgres-init/` en el primer arranque del
    contenedor; en RDS ese script no corre y el paso queda manual tras migrar (documentado en
    `infraestructura/terraform/README.md`).
- **Nginx resuelve `api` solo al arrancar** (`frontend/nginx/default.conf`): tras un despliegue de
  api, web conserva las IP viejas. El Terraform fuerza un despliegue de web cuando cambia la task
  definition de api; la solucion real es una directiva `resolver` en Nginx apuntando al DNS de
  Cloud Map, fuera de alcance de esta referencia.
- **La resolucion de nombres de Cloud Map desde las tareas no esta verificada:** por D8 no se
  aplica en LocalStack ni en AWS, asi que solo se comprobo la sintaxis (`terraform validate`).
- **Lo que no se valida en local:** TLS (ACM), DNS público (Route 53), filtrado de tráfico (WAFv2)
  y reglas de security groups. Terraform los aplica sin error en LocalStack, pero no prueban
  nada; el código los marca y el informe lo dice.
- **Checkov va a señalar simplificaciones** (por ejemplo, logs de acceso del ALB o réplicas de RDS).
  Cada excepción se documenta con `skip` y su justificación, nunca con `soft_fail`.
- **La arquitectura no se ejecuta de punta a punta** (D8): `validate` y Checkov prueban que el
  código es correcto y seguro de configurar, no que los servicios arranquen juntos en AWS.
