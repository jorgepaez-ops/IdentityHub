resource "aws_ecs_cluster" "main" {
  name = "${var.project}-${var.environment}"

  setting {
    name  = "containerInsights"
    value = "enabled"
  }
}

# ── Roles de IAM ────────────────────────────────────────────────────────────
data "aws_iam_policy_document" "ecs_assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["ecs-tasks.amazonaws.com"]
    }
  }
}

# Rol de ejecucion: baja la imagen, escribe logs y lee SOLO los secretos del proyecto.
resource "aws_iam_role" "execution" {
  name               = "${var.project}-${var.environment}-ecs-exec"
  assume_role_policy = data.aws_iam_policy_document.ecs_assume.json
}

resource "aws_iam_role_policy_attachment" "execution_managed" {
  role       = aws_iam_role.execution.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

data "aws_iam_policy_document" "execution_secrets" {
  statement {
    sid       = "LeerSecretosDelProyecto"
    actions   = ["secretsmanager:GetSecretValue"]
    resources = values(local.secret_arn)
  }
  statement {
    sid       = "DescifrarConLaClaveDelProyecto"
    actions   = ["kms:Decrypt"]
    resources = [aws_kms_key.main.arn]
  }
}

resource "aws_iam_role_policy" "execution_secrets" {
  name   = "secretos-y-kms"
  role   = aws_iam_role.execution.id
  policy = data.aws_iam_policy_document.execution_secrets.json
}

# Rol de las tareas: sin permisos (la aplicacion no llama a la API de AWS).
resource "aws_iam_role" "task" {
  name               = "${var.project}-${var.environment}-ecs-task"
  assume_role_policy = data.aws_iam_policy_document.ecs_assume.json
}

# ── Definiciones de contenedor ──────────────────────────────────────────────
locals {
  smtp_host = var.localstack ? "mailpit" : "email-smtp.${var.region}.amazonaws.com"
  smtp_port = var.localstack ? "1025" : "587"

  hub_url = "https://${var.hub_host}"

  log_config = { for s in local.services : s => {
    logDriver = "awslogs"
    options = {
      "awslogs-group"         = aws_cloudwatch_log_group.svc[s].name
      "awslogs-region"        = var.region
      "awslogs-stream-prefix" = s
    }
  } }

  # Endurecimiento comun, equivalente a read_only + cap_drop ALL del compose.
  # /tmp es un volumen efimero de la tarea (Fargate no tiene tmpfs).
  hardened = {
    readonlyRootFilesystem = true
    linuxParameters        = { capabilities = { drop = ["ALL"] } }
    mountPoints            = [{ sourceVolume = "tmp", containerPath = "/tmp", readOnly = false }]
  }

  # Variables que comparten api y worker (mismos nombres que el compose).
  app_env = [
    { name = "LOG_LEVEL", value = "info" },
    { name = "APP_VERSION", value = var.app_version },
    { name = "SMTP_HOST", value = local.smtp_host },
    { name = "SMTP_PORT", value = local.smtp_port },
    { name = "SMTP_FROM", value = var.ses_from_address },
    { name = "TRUSTED_PROXIES", value = var.vpc_cidr },
  ]

  app_secrets = [
    { name = "DATABASE_URL", valueFrom = local.secret_arn["database-url"] },
    { name = "RABBITMQ_URL", valueFrom = local.secret_arn["rabbitmq-url"] },
    { name = "JWT_SIGNING_KEY", valueFrom = local.secret_arn["jwt-signing-key"] },
  ]
}

resource "aws_ecs_task_definition" "api" {
  family                   = "${var.project}-api"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = 512
  memory                   = 1024
  execution_role_arn       = aws_iam_role.execution.arn
  task_role_arn            = aws_iam_role.task.arn

  volume {
    name = "tmp"
  }

  container_definitions = jsonencode([merge(local.hardened, {
    name             = "api"
    image            = var.image_api
    essential        = true
    user             = "65532:65532"
    portMappings     = [{ containerPort = 8081, protocol = "tcp" }]
    logConfiguration = local.log_config["api"]
    environment = concat(local.app_env, [
      { name = "API_PORT", value = "8081" },
      { name = "PUBLIC_BASE_URL", value = local.hub_url },
      { name = "JWT_ISSUER", value = local.hub_url },
      { name = "BOOTSTRAP_ADMIN_EMAIL", value = var.bootstrap_admin_email },
    ])
    secrets = local.app_secrets
    # La imagen distroless no tiene shell: el binario se autosondea.
    healthCheck = {
      command     = ["CMD", "/usr/local/bin/api", "healthcheck"]
      interval    = 10
      timeout     = 3
      retries     = 5
      startPeriod = 15
    }
  })])
}

resource "aws_ecs_task_definition" "worker" {
  family                   = "${var.project}-worker"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = 256
  memory                   = 512
  execution_role_arn       = aws_iam_role.execution.arn
  task_role_arn            = aws_iam_role.task.arn

  volume {
    name = "tmp"
  }

  container_definitions = jsonencode([merge(local.hardened, {
    name             = "worker"
    image            = var.image_worker
    essential        = true
    user             = "65532:65532"
    portMappings     = [{ containerPort = 9091, protocol = "tcp" }]
    logConfiguration = local.log_config["worker"]
    environment      = local.app_env
    secrets          = local.app_secrets
  })])
}

resource "aws_ecs_task_definition" "web" {
  family                   = "${var.project}-web"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = 256
  memory                   = 512
  execution_role_arn       = aws_iam_role.execution.arn
  task_role_arn            = aws_iam_role.task.arn

  volume {
    name = "tmp"
  }

  container_definitions = jsonencode([merge(local.hardened, {
    name             = "web"
    image            = var.image_web
    essential        = true
    user             = "101"
    portMappings     = [{ containerPort = 8080, protocol = "tcp" }]
    logConfiguration = local.log_config["web"]
  })])
}

# RabbitMQ autogestionado (D7). Sin EFS a proposito: los mensajes pendientes se
# pierden si la tarea se reemplaza. Aceptable para la referencia (el worker
# reintenta y los eventos viven en la base); una nube real usaria Amazon MQ o
# EFS/EBS. No se endurece con read-only ni cap_drop porque el entrypoint de la
# imagen arranca como root y cambia de usuario (igual que el compose).
resource "aws_ecs_task_definition" "broker" {
  family                   = "${var.project}-broker"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = 512
  memory                   = 1024
  execution_role_arn       = aws_iam_role.execution.arn
  task_role_arn            = aws_iam_role.task.arn

  volume {
    name = "rabbitdata"
  }

  container_definitions = jsonencode([{
    name             = "broker"
    image            = var.image_broker
    essential        = true
    portMappings     = [{ containerPort = 5672, protocol = "tcp" }]
    logConfiguration = local.log_config["broker"]
    mountPoints      = [{ sourceVolume = "rabbitdata", containerPath = "/var/lib/rabbitmq", readOnly = false }]
    environment      = [{ name = "RABBITMQ_DEFAULT_USER", value = local.rabbitmq_user }]
    secrets          = [{ name = "RABBITMQ_DEFAULT_PASS", valueFrom = local.secret_arn["rabbitmq-password"] }]
    healthCheck = {
      command     = ["CMD", "rabbitmq-diagnostics", "-q", "check_running"]
      interval    = 10
      timeout     = 10
      retries     = 10
      startPeriod = 30
    }
  }])
}

# Solo con localstack = true: en AWS el correo sale por SES.
resource "aws_ecs_task_definition" "mailpit" {
  count = var.localstack ? 1 : 0

  family                   = "${var.project}-mailpit"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = 256
  memory                   = 512
  execution_role_arn       = aws_iam_role.execution.arn
  task_role_arn            = aws_iam_role.task.arn

  volume {
    name = "tmp"
  }

  container_definitions = jsonencode([merge(local.hardened, {
    name      = "mailpit"
    image     = var.image_mailpit
    essential = true
    portMappings = [
      { containerPort = 1025, protocol = "tcp" },
      { containerPort = 8025, protocol = "tcp" },
    ]
    logConfiguration = local.log_config["mailpit"]
    environment = [
      { name = "MP_MAX_MESSAGES", value = "500" },
      { name = "MP_SMTP_AUTH_ACCEPT_ANY", value = "1" },
      { name = "MP_SMTP_AUTH_ALLOW_INSECURE", value = "1" },
    ]
    healthCheck = {
      command     = ["CMD", "/mailpit", "readyz"]
      interval    = 10
      timeout     = 5
      retries     = 5
      startPeriod = 5
    }
  })])
}

# Tarea de un solo uso (equivale al servicio migrate): se lanza con
# "aws ecs run-task" antes de desplegar api (ver README). No es un servicio.
resource "aws_ecs_task_definition" "migrate" {
  family                   = "${var.project}-migrate"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = 256
  memory                   = 512
  execution_role_arn       = aws_iam_role.execution.arn
  task_role_arn            = aws_iam_role.task.arn

  container_definitions = jsonencode([{
    name                   = "migrate"
    image                  = var.image_migrate
    essential              = true
    readonlyRootFilesystem = true
    linuxParameters        = { capabilities = { drop = ["ALL"] } }
    logConfiguration       = local.log_config["migrate"]
    # migrate no lee la URL del entorno: se expande en un shell (la imagen
    # migrate/migrate es Alpine). ECS no interpola variables en "command".
    entryPoint = ["/bin/sh", "-c"]
    command    = ["exec migrate -path=/migrations -database \"$DATABASE_URL\" up"]
    secrets    = [{ name = "DATABASE_URL", valueFrom = local.secret_arn["migrate-database-url"] }]
  }])
}

# ── Servicios ───────────────────────────────────────────────────────────────
locals {
  private_subnet_ids = aws_subnet.private[*].id
}

resource "aws_ecs_service" "broker" {
  name            = "broker"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.broker.arn
  desired_count   = 1
  launch_type     = "FARGATE"

  # RabbitMQ no tiene almacenamiento persistente y ambas tareas se registrarian
  # con el mismo nombre de Cloud Map. Con el reemplazo gradual por defecto
  # (100 % sanas, 200 % maximo) convivirian dos brokers y los mensajes quedarian
  # repartidos entre ellos. Se acepta una breve caida del broker durante el
  # despliegue (se detiene la tarea vieja antes de iniciar la nueva) a cambio de
  # no partir los mensajes; el worker y la api reintentan la conexion.
  deployment_minimum_healthy_percent = 0
  deployment_maximum_percent         = 100

  network_configuration {
    subnets          = local.private_subnet_ids
    security_groups  = [aws_security_group.svc["broker"].id]
    assign_public_ip = false
  }

  service_registries {
    registry_arn = aws_service_discovery_service.svc["broker"].arn
  }

  deployment_circuit_breaker {
    enable   = true
    rollback = true
  }
}

resource "aws_ecs_service" "mailpit" {
  count = var.localstack ? 1 : 0

  name            = "mailpit"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.mailpit[0].arn
  desired_count   = 1
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = local.private_subnet_ids
    security_groups  = [aws_security_group.svc["mail"].id]
    assign_public_ip = false
  }

  service_registries {
    registry_arn = aws_service_discovery_service.svc["mailpit"].arn
  }
}

# Desplegar api solo despues de ejecutar la tarea migrate: el orden lo impone el
# procedimiento de dos fases del README (apply de task definitions, migrate,
# apply completo), no Terraform.
resource "aws_ecs_service" "api" {
  name            = "api"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.api.arn
  desired_count   = var.app_desired_count
  launch_type     = "FARGATE"

  # Espera a que el despliegue de api quede estable; asi web (depends_on) solo se
  # crea o actualiza cuando api ya tiene tareas sanas.
  wait_for_steady_state = true

  network_configuration {
    subnets          = local.private_subnet_ids
    security_groups  = [aws_security_group.svc["api"].id]
    assign_public_ip = false
  }

  service_registries {
    registry_arn = aws_service_discovery_service.svc["api"].arn
  }

  deployment_circuit_breaker {
    enable   = true
    rollback = true
  }

  depends_on = [aws_ecs_service.broker, aws_service_discovery_instance.db]
}

resource "aws_ecs_service" "worker" {
  name            = "worker"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.worker.arn
  desired_count   = 1
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = local.private_subnet_ids
    security_groups  = [aws_security_group.svc["worker"].id]
    assign_public_ip = false
  }

  deployment_circuit_breaker {
    enable   = true
    rollback = true
  }

  depends_on = [aws_ecs_service.broker, aws_ecs_service.mailpit, aws_service_discovery_instance.db]
}

# Nginx resuelve "api" al arrancar (upstream api:8081), asi que web espera a api.
resource "aws_ecs_service" "web" {
  name            = "web"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.web.arn
  desired_count   = var.app_desired_count
  launch_type     = "FARGATE"

  # Nginx resuelve "api" solo al arrancar, asi que tras cada despliegue de api
  # web conservaria IP viejas. Se fuerza un despliegue de web cuando cambia la
  # task definition de api. La solucion real es una directiva resolver en Nginx
  # (DNS de Cloud Map con TTL corto); queda fuera de alcance (ADR 0012).
  force_new_deployment = true
  triggers = {
    api_task_definition = aws_ecs_task_definition.api.arn
  }

  network_configuration {
    subnets          = local.private_subnet_ids
    security_groups  = [aws_security_group.svc["web"].id]
    assign_public_ip = false
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.web.arn
    container_name   = "web"
    container_port   = 8080
  }

  deployment_circuit_breaker {
    enable   = true
    rollback = true
  }

  depends_on = [aws_ecs_service.api, aws_lb_listener.https]
}
