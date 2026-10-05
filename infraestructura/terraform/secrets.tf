# Los valores se generan aqui y viven en Secrets Manager (cifrado con KMS). Las
# task definitions los referencian por ARN (valueFrom); ninguna variable ni
# salida los expone. OJO: los valores generados SI quedan en el estado de
# Terraform, que por eso debe guardarse cifrado (ver README).

resource "random_password" "db_admin" {
  length  = 32
  special = false # sin caracteres que rompan la URL de conexion
}

resource "random_password" "identity_app" {
  length  = 32
  special = false
}

resource "random_password" "rabbitmq" {
  length  = 32
  special = false
}

# Semilla Ed25519 de 32 bytes en base64 (formato que exige JWT_SIGNING_KEY).
resource "random_bytes" "jwt_seed" {
  length = 32
}

locals {
  rabbitmq_user = "identityhub"
  # Contra LocalStack el RDS local no negocia TLS igual; en AWS real se exige.
  db_sslmode = var.localstack ? "disable" : "require"

  secret_values = {
    "jwt-signing-key"      = random_bytes.jwt_seed.base64
    "rabbitmq-password"    = random_password.rabbitmq.result
    "identity-app-pass"    = random_password.identity_app.result
    "rabbitmq-url"         = "amqp://${local.rabbitmq_user}:${random_password.rabbitmq.result}@broker:5672/"
    "database-url"         = "postgres://identity_app:${random_password.identity_app.result}@db:5432/${var.db_name}?sslmode=${local.db_sslmode}"
    "migrate-database-url" = "postgres://${var.db_admin_username}:${random_password.db_admin.result}@db:5432/${var.db_name}?sslmode=${local.db_sslmode}"
  }
}

resource "aws_secretsmanager_secret" "app" {
  for_each = toset(["jwt-signing-key", "rabbitmq-password", "identity-app-pass", "rabbitmq-url", "database-url", "migrate-database-url"])

  name                    = "${var.project}/${var.environment}/${each.key}"
  kms_key_id              = aws_kms_key.main.arn
  recovery_window_in_days = 7
  # Sin rotacion automatica: la semilla JWT rota con un despliegue coordinado.
}

resource "aws_secretsmanager_secret_version" "app" {
  for_each = aws_secretsmanager_secret.app

  secret_id     = each.value.id
  secret_string = local.secret_values[each.key]
}

locals {
  secret_arn = { for k, s in aws_secretsmanager_secret.app : k => s.arn }
}
