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

  # Gitleaks fija las lineas senaladas por numero en .gitleaksignore: si las mueves, actualiza las huellas.
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
  #checkov:skip=CKV2_AWS_57:La referencia no incluye una Lambda de rotacion; la rotacion es manual y esta documentada en el README
}

resource "aws_secretsmanager_secret_version" "app" {
  for_each = aws_secretsmanager_secret.app

  secret_id     = each.value.id
  secret_string = local.secret_values[each.key]
}

locals {
  secret_arn = { for k, s in aws_secretsmanager_secret.app : k => s.arn }
}

# ── Credenciales SMTP de SES (solo produccion) ──────────────────────────────
# El worker autentica contra el endpoint SMTP de SES (587, STARTTLS). Las
# credenciales SMTP no son la clave IAM: la contrasena es una derivada de ella
# (ses_smtp_password_v4). Un usuario IAM dedicado solo puede enviar desde la
# direccion configurada. Con localstack = true no existen: Mailpit no exige TLS.
resource "aws_iam_user" "ses_smtp" {
  #checkov:skip=CKV_AWS_273:Las credenciales SMTP de SES se derivan de una access key de usuario IAM; un rol o SSO no puede autenticarse por SMTP
  count = var.localstack ? 0 : 1

  name = "${var.project}-${var.environment}-ses-smtp"
}

data "aws_iam_policy_document" "ses_send" {
  count = var.localstack ? 0 : 1

  statement {
    sid       = "EnviarDesdeLaDireccionDelProyecto"
    actions   = ["ses:SendRawEmail"]
    resources = [aws_ses_domain_identity.main[0].arn]
    condition {
      test     = "StringEquals"
      variable = "ses:FromAddress"
      values   = [var.ses_from_address]
    }
  }
}

resource "aws_iam_user_policy" "ses_send" {
  #checkov:skip=CKV_AWS_40:Politica minima (solo ses:SendRawEmail desde una direccion) del unico usuario tecnico de SMTP; un grupo de un solo miembro no reduce el riesgo
  count = var.localstack ? 0 : 1

  name   = "enviar-correo"
  user   = aws_iam_user.ses_smtp[0].name
  policy = data.aws_iam_policy_document.ses_send[0].json
}

# La clave y su derivada quedan en el estado de Terraform (ver README).
resource "aws_iam_access_key" "ses_smtp" {
  count = var.localstack ? 0 : 1

  user = aws_iam_user.ses_smtp[0].name
}

resource "aws_secretsmanager_secret" "smtp_username" {
  count = var.localstack ? 0 : 1

  name                    = "${var.project}/${var.environment}/smtp-username"
  kms_key_id              = aws_kms_key.main.arn
  recovery_window_in_days = 7
  #checkov:skip=CKV2_AWS_57:La referencia no incluye una Lambda de rotacion; la rotacion es manual y esta documentada en el README
}

resource "aws_secretsmanager_secret_version" "smtp_username" {
  count = var.localstack ? 0 : 1

  secret_id     = aws_secretsmanager_secret.smtp_username[0].id
  secret_string = aws_iam_access_key.ses_smtp[0].id
}

resource "aws_secretsmanager_secret" "smtp_password" {
  count = var.localstack ? 0 : 1

  name                    = "${var.project}/${var.environment}/smtp-password"
  kms_key_id              = aws_kms_key.main.arn
  recovery_window_in_days = 7
  #checkov:skip=CKV2_AWS_57:La referencia no incluye una Lambda de rotacion; la rotacion es manual y esta documentada en el README
}

resource "aws_secretsmanager_secret_version" "smtp_password" {
  count = var.localstack ? 0 : 1

  secret_id     = aws_secretsmanager_secret.smtp_password[0].id
  secret_string = aws_iam_access_key.ses_smtp[0].ses_smtp_password_v4
}

locals {
  smtp_secret_arns = concat(aws_secretsmanager_secret.smtp_username[*].arn, aws_secretsmanager_secret.smtp_password[*].arn)

  # Solo el worker las recibe (la api no envia correo).
  smtp_secrets = var.localstack ? [] : [
    { name = "SMTP_USERNAME", valueFrom = aws_secretsmanager_secret.smtp_username[0].arn },
    { name = "SMTP_PASSWORD", valueFrom = aws_secretsmanager_secret.smtp_password[0].arn },
  ]
}
