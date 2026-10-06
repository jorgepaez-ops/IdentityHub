# Minimo privilegio: ALB -> web (8080) -> api (8081) -> db (5432) / broker (5672);
# worker -> broker / correo. Todo con reglas separadas y con descripcion.
# No se aplica en LocalStack: solo CRUD (ADR 0012)

locals {
  sg_names = ["alb", "web", "api", "worker", "broker", "db", "mail", "migrate"]
  # Grupos cuyas tareas necesitan salir por 443 (Docker Hub, Secrets Manager, Logs) via NAT.
  sg_https_egress = ["web", "api", "worker", "broker", "mail", "migrate"]
}

# No se aplica en LocalStack: solo CRUD (ADR 0012)
resource "aws_security_group" "svc" {
  #checkov:skip=CKV2_AWS_5:El grupo migrate lo usa la tarea de un solo uso lanzada con aws ecs run-task, no un servicio ECS
  for_each = toset(local.sg_names)

  name        = "${var.project}-${var.environment}-${each.key}"
  description = "Identity Hub: ${each.key}"
  vpc_id      = aws_vpc.main.id

  tags = { Name = "${var.project}-${each.key}" }
}

# ── ALB ─────────────────────────────────────────────────────────────────────
# No se aplica en LocalStack: solo CRUD (ADR 0012)
resource "aws_vpc_security_group_ingress_rule" "alb_https" {
  security_group_id = aws_security_group.svc["alb"].id
  description       = "HTTPS publico"
  ip_protocol       = "tcp"
  from_port         = 443
  to_port           = 443
  cidr_ipv4         = "0.0.0.0/0"
}

# No se aplica en LocalStack: solo CRUD (ADR 0012)
resource "aws_vpc_security_group_ingress_rule" "alb_http" {
  #checkov:skip=CKV_AWS_260:El puerto 80 del ALB solo redirige a HTTPS (301) y no sirve contenido
  security_group_id = aws_security_group.svc["alb"].id
  description       = "HTTP publico, el listener solo redirige a HTTPS"
  ip_protocol       = "tcp"
  from_port         = 80
  to_port           = 80
  cidr_ipv4         = "0.0.0.0/0"
}

# No se aplica en LocalStack: solo CRUD (ADR 0012)
resource "aws_vpc_security_group_egress_rule" "alb_to_web" {
  security_group_id            = aws_security_group.svc["alb"].id
  description                  = "ALB hacia web"
  ip_protocol                  = "tcp"
  from_port                    = 8080
  to_port                      = 8080
  referenced_security_group_id = aws_security_group.svc["web"].id
}

# ── Cadena web -> api ───────────────────────────────────────────────────────
# No se aplica en LocalStack: solo CRUD (ADR 0012)
resource "aws_vpc_security_group_ingress_rule" "web_from_alb" {
  security_group_id            = aws_security_group.svc["web"].id
  description                  = "ALB hacia Nginx"
  ip_protocol                  = "tcp"
  from_port                    = 8080
  to_port                      = 8080
  referenced_security_group_id = aws_security_group.svc["alb"].id
}

# No se aplica en LocalStack: solo CRUD (ADR 0012)
resource "aws_vpc_security_group_egress_rule" "web_to_api" {
  security_group_id            = aws_security_group.svc["web"].id
  description                  = "Nginx hacia la API"
  ip_protocol                  = "tcp"
  from_port                    = 8081
  to_port                      = 8081
  referenced_security_group_id = aws_security_group.svc["api"].id
}

# No se aplica en LocalStack: solo CRUD (ADR 0012)
resource "aws_vpc_security_group_ingress_rule" "api_from_web" {
  security_group_id            = aws_security_group.svc["api"].id
  description                  = "Nginx hacia la API"
  ip_protocol                  = "tcp"
  from_port                    = 8081
  to_port                      = 8081
  referenced_security_group_id = aws_security_group.svc["web"].id
}

# ── Datos: db (5432) y broker (5672) ────────────────────────────────────────
locals {
  db_clients     = ["api", "migrate"]
  broker_clients = ["api", "worker"]
}

# No se aplica en LocalStack: solo CRUD (ADR 0012)
resource "aws_vpc_security_group_ingress_rule" "db_from" {
  for_each = toset(local.db_clients)

  security_group_id            = aws_security_group.svc["db"].id
  description                  = "PostgreSQL desde ${each.key}"
  ip_protocol                  = "tcp"
  from_port                    = 5432
  to_port                      = 5432
  referenced_security_group_id = aws_security_group.svc[each.key].id
}

# No se aplica en LocalStack: solo CRUD (ADR 0012)
resource "aws_vpc_security_group_egress_rule" "to_db" {
  for_each = toset(local.db_clients)

  security_group_id            = aws_security_group.svc[each.key].id
  description                  = "PostgreSQL hacia db"
  ip_protocol                  = "tcp"
  from_port                    = 5432
  to_port                      = 5432
  referenced_security_group_id = aws_security_group.svc["db"].id
}

# No se aplica en LocalStack: solo CRUD (ADR 0012)
resource "aws_vpc_security_group_ingress_rule" "broker_from" {
  for_each = toset(local.broker_clients)

  security_group_id            = aws_security_group.svc["broker"].id
  description                  = "AMQP desde ${each.key}"
  ip_protocol                  = "tcp"
  from_port                    = 5672
  to_port                      = 5672
  referenced_security_group_id = aws_security_group.svc[each.key].id
}

# No se aplica en LocalStack: solo CRUD (ADR 0012)
resource "aws_vpc_security_group_egress_rule" "to_broker" {
  for_each = toset(local.broker_clients)

  security_group_id            = aws_security_group.svc[each.key].id
  description                  = "AMQP hacia broker"
  ip_protocol                  = "tcp"
  from_port                    = 5672
  to_port                      = 5672
  referenced_security_group_id = aws_security_group.svc["broker"].id
}

# ── Correo (solo worker) ────────────────────────────────────────────────────
# No se aplica en LocalStack: solo CRUD (ADR 0012)
resource "aws_vpc_security_group_ingress_rule" "mail_from_worker" {
  security_group_id            = aws_security_group.svc["mail"].id
  description                  = "SMTP desde worker (Mailpit, solo localstack)"
  ip_protocol                  = "tcp"
  from_port                    = 1025
  to_port                      = 1025
  referenced_security_group_id = aws_security_group.svc["worker"].id
}

# No se aplica en LocalStack: solo CRUD (ADR 0012)
resource "aws_vpc_security_group_egress_rule" "worker_to_mail" {
  security_group_id            = aws_security_group.svc["worker"].id
  description                  = "SMTP hacia Mailpit"
  ip_protocol                  = "tcp"
  from_port                    = 1025
  to_port                      = 1025
  referenced_security_group_id = aws_security_group.svc["mail"].id
}

# No se aplica en LocalStack: solo CRUD (ADR 0012)
# Excepcion aceptada de Trivy (AWS-0104): las tareas necesitan SMTP (587) hacia la interfaz de SES, que
# solo se alcanza por Internet via NAT; un VPC endpoint de SES es trabajo futuro (ADR 0012).
#trivy:ignore:AWS-0104
resource "aws_vpc_security_group_egress_rule" "worker_to_ses" {
  security_group_id = aws_security_group.svc["worker"].id
  description       = "SMTP con STARTTLS hacia la interfaz SMTP de SES"
  ip_protocol       = "tcp"
  from_port         = 587
  to_port           = 587
  cidr_ipv4         = "0.0.0.0/0"
}

# ── Salida HTTPS por la NAT ─────────────────────────────────────────────────
# No se aplica en LocalStack: solo CRUD (ADR 0012)
# Excepcion aceptada de Trivy (AWS-0104): las tareas necesitan HTTPS hacia Internet por la NAT
# (Docker Hub, Secrets Manager y CloudWatch Logs). Los VPC endpoints y un espejo de ECR son
# trabajo futuro (ADR 0012).
#trivy:ignore:AWS-0104
resource "aws_vpc_security_group_egress_rule" "https_out" {
  for_each = toset(local.sg_https_egress)

  security_group_id = aws_security_group.svc[each.key].id
  description       = "HTTPS hacia Docker Hub, Secrets Manager y CloudWatch (via NAT)"
  ip_protocol       = "tcp"
  from_port         = 443
  to_port           = 443
  cidr_ipv4         = "0.0.0.0/0"
}
