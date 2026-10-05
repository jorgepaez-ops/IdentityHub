# Arquitectura de referencia en AWS (Terraform)

Implementa la decision de `specs/adr/0012-arquitectura-de-referencia-en-aws-con-localstack.md`:
la topologia del `docker-compose.yml` en AWS, escrita como ejemplo de produccion (D3) y validada
en lo posible con LocalStack. **No es necesaria para ejecutar el proyecto**: eso sigue siendo
`make setup && docker compose up -d`.

## Que construye cada archivo

| Archivo | Contenido |
|---|---|
| `versions.tf`, `providers.tf` | Terraform >= 1.6, `hashicorp/aws ~> 6.0`, `random ~> 3.6`; backend S3 descrito en comentario |
| `variables.tf`, `terraform.tfvars.example` | Entradas; las imagenes (por digest) y los dominios son obligatorios |
| `network.tf` | VPC, 2 subredes publicas (ALB, NAT por zona o unica) y 2 privadas (ECS, RDS), DHCP con el dominio de Cloud Map |
| `security_groups.tf` | Un grupo por servicio y reglas de minimo privilegio (ALB, web 8080, api 8081, db 5432, broker 5672, correo) |
| `alb.tf` | ALB (con logs de acceso), listener HTTPS, HTTP que redirige, regla por `Host` hacia `web` |
| `alb_logs.tf` | Bucket S3 de los logs de acceso del ALB (cifrado SSE-S3, versionado, bloqueo publico, solo TLS) |
| `ecs.tf` | Cluster, roles IAM, task definitions (api, worker, web, broker, mailpit, migrate) y servicios |
| `rds.tf` | RDS PostgreSQL 16 cifrado con KMS, privado, con backups y proteccion contra borrado |
| `secrets.tf` | Contrasenas y semilla Ed25519 generadas con `random`, guardadas en Secrets Manager |
| `kms.tf`, `logs.tf` | Clave KMS con rotacion; grupos de CloudWatch Logs con retencion y KMS |
| `service_discovery.tf` | Cloud Map: `api`, `broker`, `db` (CNAME a RDS) y `mailpit` resuelven como en el compose |
| `dns_tls_waf.tf` | Route 53, ACM, WAFv2 sobre el ALB y la identidad de SES |
| `outputs.tf` | Identificadores y ARN; nunca valores de secretos |

## Del compose a AWS

| Compose | AWS | LocalStack |
|---|---|---|
| `web` | ECS Fargate tras el ALB (reglas por `Host`) | Real |
| `api`, `worker` | ECS Fargate en subredes privadas | Real |
| `db` | RDS PostgreSQL (nombre `db` por Cloud Map) | Real |
| `broker` | RabbitMQ autogestionado en ECS (D7), sin Amazon MQ | Real |
| `migrate` | Task definition de un solo uso | Real |
| `mailpit` | SES; Mailpit en ECS solo con `localstack = true` | Parcial |
| `.env` | Secrets Manager + `secrets.valueFrom` | Emulado |
| Logs | CloudWatch Logs (retencion + KMS) | Emulado |
| TLS, DNS, WAF, security groups | ACM, Route 53, WAFv2, SG | Solo CRUD |

Los recursos que LocalStack no aplica de verdad llevan el comentario
`# No se aplica en LocalStack: solo CRUD (ADR 0012)`.

El endurecimiento replica el compose: `readonlyRootFilesystem` y `capabilities drop ALL` en api,
worker, web, mailpit y migrate (con `/tmp` como volumen efimero), usuarios no-root (`65532` y `101`),
tareas sin IP publica, RDS privado y cifrado, ALB con `drop_invalid_header_fields`. El broker no se
endurece igual (como en el compose: su entrypoint arranca como root).

## Validar (sin cuenta de AWS ni LocalStack)

```sh
cd infraestructura/terraform
terraform init -backend=false
terraform fmt -check -recursive
terraform validate
```

`.terraform.lock.hcl` se versiona (versiones exactas de los providers).

## Aplicar contra LocalStack (lo hace el usuario, a mano)

```sh
pipx install terraform-local            # una vez
export LOCALSTACK_AUTH_TOKEN=...        # solo en tu entorno, nunca en el repo
localstack start -d                     # o docker run con la imagen oficial
cp terraform.tfvars.example terraform.tfvars   # y pon localstack = true
tflocal init && tflocal apply
```

`tflocal` genera `localstack_providers_override.tf` con los endpoints locales (ignorado por git),
por eso el codigo no lleva endpoints.

## Desplegar una version (procedimiento de dos fases)

Terraform no ordena la migracion respecto a `api`: un solo `apply` registra la task definition
nueva de `api` y actualiza el servicio a la vez. Por eso el despliegue es un procedimiento manual
en dos fases; el orden lo garantiza quien lo ejecuta, no el codigo.

**Fase 1: registrar las task definitions y la infraestructura que `migrate` necesita, sin tocar
los servicios.** `-target` limita el `apply` a esos recursos y sus dependencias; los servicios
siguen en la revision anterior. En un primer despliegue crea ademas la red, RDS y los secretos.

```sh
terraform apply \
  -target=aws_ecs_task_definition.migrate -target=aws_ecs_task_definition.api \
  -target=aws_ecs_task_definition.worker  -target=aws_ecs_task_definition.web \
  -target=aws_db_instance.main -target=aws_service_discovery_instance.db \
  -target=aws_security_group.svc -target=aws_route_table_association.private \
  -target=aws_vpc_dhcp_options_association.main -target=aws_ecs_cluster.main
```

**Migrar** (cluster, subredes y security group salen de las salidas de Terraform):

```sh
aws ecs run-task --cluster "$(terraform output -raw ecs_cluster_name)" --launch-type FARGATE \
  --task-definition identity-hub-migrate \
  --network-configuration "awsvpcConfiguration={subnets=[$(terraform output -raw private_subnet_ids_csv)],securityGroups=[$(terraform output -raw migrate_security_group_id)],assignPublicIp=DISABLED}"
```

Espera a que la tarea termine con codigo 0 (`aws ecs describe-tasks`). Solo la primera vez, da
login al rol `identity_app` (la migracion 000002 lo crea `NOLOGIN`; en el compose lo hace
`deploy/postgres-init/01-identity-app-role.sh`), usando el secreto `identity-app-pass`.

**Fase 2: `terraform apply` completo.** Crea o actualiza los servicios con las task definitions ya
registradas. `api` espera a quedar estable (`wait_for_steady_state`) y `web` se redespliega cuando
cambia la task definition de `api`.

Otras salidas utiles: `private_subnet_ids` (lista, para `-json`) y `migrate_task_definition`.

## Secretos y estado

Las contrasenas y la semilla JWT se generan con el provider `random` y **quedan en el estado de
Terraform en claro**. El estado local esta en `.gitignore` y no debe compartirse. Para uso real:
backend S3 con `encrypt = true`, clave KMS propia, versionado del bucket y bloqueo
(`use_lockfile`), acceso restringido por IAM. El bloque esta descrito en `versions.tf`, no activado.

## Lo que no se valida en local

TLS (ACM), DNS (Route 53), WAFv2 y las reglas de security groups: LocalStack los crea pero no los
hace cumplir. Tampoco se prueba la resolucion de nombres de Cloud Map desde las tareas locales ni
el comportamiento Multi-AZ, backups o proteccion contra borrado de RDS (`terraform destroy` exige
quitar `deletion_protection` antes en una cuenta real).

## Brechas conocidas

- **Correo real:** el worker envia SMTP sin autenticacion ni TLS (ADR 0012); la interfaz SMTP de
  SES exige ambos. Con `localstack = false` el worker apunta a SES pero no podra autenticarse hasta
  cambiar el codigo.
- **RabbitMQ autogestionado:** un solo nodo y sin disco persistente (decision D7); Amazon MQ no se
  usa porque LocalStack no lo soporta.
- **Imagen de migrate:** debe incluir `db/migrations` (el compose las monta como volumen).
- **Hosts fijos en la aplicacion:** Nginx (`server_name *.localhost`) y `config.go`
  (`OAuthRedirectURI`) tienen los dominios `.localhost` escritos; con dominios reales hay que
  parametrizarlos antes de que el flujo OAuth funcione de extremo a extremo.
- **IP de cliente:** Nginx sobrescribe `X-Forwarded-For` con la IP del ALB, asi que detras del ALB
  la API ve la IP del balanceador; ajustar `TRUSTED_PROXIES` y esa cabecera es trabajo futuro.
- **Nginx resuelve `api` solo al arrancar:** Terraform fuerza un despliegue de web cuando cambia la
  task definition de api, pero la solucion real es una directiva `resolver` en Nginx (DNS de Cloud
  Map), fuera de alcance. Una tarea de api reemplazada sin cambio de task definition sigue
  requiriendo reiniciar web.
- **Una sola NAT opcional:** `nat_gateway_per_az = false` deja una NAT (punto unico de fallo); el
  valor por defecto, `true`, crea una por zona.
- **Checkov** senalara simplificaciones; se documentan como excepciones en T10. Los logs de acceso
  del ALB van a un bucket S3 propio (`alb_logs.tf`).
