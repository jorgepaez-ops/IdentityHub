# Cloud Map: DNS privado para que los nombres del compose (api, broker, db,
# mailpit) resuelvan igual en AWS. La busqueda corta funciona por el dominio de
# busqueda del DHCP (network.tf).
# Sin verificar: la resolucion de estos nombres desde las tareas nunca se
# comprobo, porque por D8 no se hace apply (ni en LocalStack ni en AWS); solo
# pasan terraform validate y Checkov. Figura como no verificada en las
# consecuencias de la ADR 0012.

resource "aws_service_discovery_private_dns_namespace" "main" {
  name        = var.internal_domain
  description = "Nombres internos de ${var.project}"
  vpc         = aws_vpc.main.id
}

locals {
  discovered = toset(concat(["api", "broker"], var.localstack ? ["mailpit"] : []))
}

resource "aws_service_discovery_service" "svc" {
  for_each = local.discovered

  name = each.key

  dns_config {
    namespace_id   = aws_service_discovery_private_dns_namespace.main.id
    routing_policy = "MULTIVALUE"

    dns_records {
      type = "A"
      ttl  = 10
    }
  }

  health_check_custom_config {}
}

# db apunta al endpoint de RDS con un CNAME, no a una tarea.
resource "aws_service_discovery_service" "db" {
  name = "db"

  dns_config {
    namespace_id   = aws_service_discovery_private_dns_namespace.main.id
    routing_policy = "WEIGHTED"

    dns_records {
      type = "CNAME"
      ttl  = 30
    }
  }
}

resource "aws_service_discovery_instance" "db" {
  service_id  = aws_service_discovery_service.db.id
  instance_id = "rds"

  attributes = {
    AWS_INSTANCE_CNAME = aws_db_instance.main.address
  }
}
