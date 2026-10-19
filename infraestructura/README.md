# infraestructura

La arquitectura de referencia en AWS, escrita con Terraform. Muestra cómo se desplegaría Identity Hub en la nube; **no hace falta para correr el proyecto**, que funciona completo en local con Docker Compose.

- `terraform/`: los módulos (red, balanceador, ECS, base de datos RDS, secretos, cifrado, DNS y WAF) y su propio [README](terraform/README.md), que explica cada archivo.

```bash
cd infraestructura/terraform
terraform init -backend=false && terraform validate   # valida sin cuenta de AWS
make scan-iac                                         # revisión de seguridad con Checkov
```

La decisión detrás de esta carpeta está en el [ADR 0012](../specs/adr/0012-arquitectura-de-referencia-en-aws-con-localstack.md) y el diagrama en [despliegue en AWS](../docs/diagramas/uml/despliegue-aws.md). Volver al [mapa del repositorio](../README.md#mapa-del-repositorio).
