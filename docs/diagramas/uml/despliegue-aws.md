# Despliegue de referencia en AWS

Cómo se desplegaría Identity Hub en producción según la [ADR 0012](../../../specs/adr/0012-arquitectura-de-referencia-en-aws-con-localstack.md)
y el Terraform de [`infraestructura/terraform/`](../../../infraestructura/terraform/). Es una
arquitectura de referencia: el código pasa `terraform validate` y Checkov, pero no se aplica en una
nube real ni en LocalStack (decisión D8). El despliegue local que sí funciona está en
[`despliegue.md`](despliegue.md).

## Infraestructura

Una VPC `10.20.0.0/16` en dos zonas de disponibilidad. Solo el ALB vive en subredes públicas; las
tareas ECS y RDS van en subredes privadas, sin IP pública, y salen a Internet (descarga de imágenes
de Docker Hub, SES) por un NAT gateway. Las líneas punteadas marcan dependencias de configuración
(secretos, claves, logs), no tráfico de usuarios.

```mermaid
flowchart TB
    User([Navegador del empleado o admin])
    DockerHub[(Docker Hub: imágenes por digest, ADR 0011)]

    subgraph AWS[AWS us-east-1]
        R53[Route 53: identityhub y contabilidad]
        ACM[ACM: certificado TLS]
        WAF[WAFv2: CommonRuleSet y KnownBadInputs]

        subgraph VPC[VPC 10.20.0.0/16 en dos zonas]
            subgraph Public[Subredes públicas]
                ALB[ALB: 443 con TLS 1.3, 80 redirige a 443, reglas por Host]
                NAT[NAT gateway]
            end
            subgraph Private[Subredes privadas]
                subgraph ECS[Cluster ECS Fargate]
                    Web[web: Nginx con las dos SPA :8080]
                    Api[api :8081]
                    Worker[worker :9091 métricas]
                    Broker[broker: RabbitMQ :5672 en ECS, decisión D7]
                    Migrate[migrate: tarea de un solo uso]
                end
                RDS[(RDS PostgreSQL 16 cifrado, sin acceso público)]
                Map[Cloud Map: DNS privado api, broker, db]
            end
        end

        SM[Secrets Manager: contraseñas, URLs y semilla JWT]
        KMS[KMS: clave del proyecto]
        CW[CloudWatch Logs con retención]
        SES[SES: correo saliente]
    end

    User -->|HTTPS| R53 --> ALB
    WAF -.-> ALB
    ACM -.-> ALB
    ALB -->|HTTP :8080| Web
    Web -->|/api| Api
    Api -->|5432| RDS
    Api -->|AMQP 5672| Broker
    Worker -->|AMQP 5672| Broker
    Worker -->|SMTP| NAT
    Migrate -->|5432| RDS
    NAT --> SES
    NAT --> DockerHub
    Map -.-> Api
    SM -.->|secrets valueFrom| ECS
    KMS -.-> SM
    KMS -.-> RDS
    KMS -.-> CW
    ECS -.->|awslogs| CW
```

## Despliegue de una versión

Cómo llegaría una versión nueva a producción. La publicación de imágenes llega con T17; el resto
es lo que describe el Terraform. El orden migrar antes de actualizar `api` lo garantiza el
procedimiento de dos fases del [README](../../../infraestructura/terraform/README.md), no Terraform.

```mermaid
sequenceDiagram
    participant Dev as Mantenedor
    participant GH as GitHub Actions
    participant DH as Docker Hub
    participant TF as Terraform
    participant ECS as ECS Fargate
    participant RDS as RDS PostgreSQL

    Dev->>GH: Crea el tag vX.Y.Z
    GH->>GH: Gates de CI, Trivy y Checkov
    GH->>DH: Publica api, worker y web con firma Cosign y SBOM (T17)
    DH-->>Dev: Digests sha256 de cada imagen
    Dev->>TF: Fase 1, terraform apply con los digests y -target a las task definitions y a RDS
    TF->>ECS: Registra las task definitions nuevas sin tocar los servicios
    Dev->>ECS: aws ecs run-task migrate
    ECS->>RDS: Aplica las migraciones
    Note over Dev,RDS: Paso manual tras la primera migración, dar login al rol identity_app
    Dev->>TF: Fase 2, terraform apply completo
    TF->>ECS: Actualiza los servicios, api espera a quedar estable y luego web
    ECS->>ECS: Reemplazo gradual de tareas, el ALB solo enruta a tareas sanas
```

## Lo que este diagrama no promete

- **TLS, DNS público y WAF** se definen pero no se prueban (solo se crean en LocalStack, y no se
  aplica en ninguna nube).
- **El worker no toca la base de datos.** Solo consume la cola (AMQP) y envía SMTP: no recibe `DATABASE_URL` ni tiene regla de red hacia RDS.
- **El correo por SES** exige que el worker soporte autenticación SMTP y STARTTLS, y hoy no lo hace.
- **Detrás del ALB**, Nginx tiene que propagar la IP real del cliente; si no, el límite de fallos
  por IP trataría a todos los usuarios como uno solo.
- **Los hosts `*.localhost`** están fijos en el código y habría que volverlos configurables.

Detalle de estas brechas: [ADR 0012, Consecuencias](../../../specs/adr/0012-arquitectura-de-referencia-en-aws-con-localstack.md).

Fuente: `infraestructura/terraform/` (`network.tf`, `alb.tf`, `ecs.tf`, `rds.tf`, `secrets.tf`,
`kms.tf`, `logs.tf`, `service_discovery.tf`, `dns_tls_waf.tf`) y la ADR 0012.
