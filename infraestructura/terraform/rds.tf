resource "aws_db_subnet_group" "main" {
  name       = "${var.project}-${var.environment}"
  subnet_ids = aws_subnet.private[*].id
}

# Equivale al servicio db del compose (postgres:16). En LocalStack ejecuta un
# PostgreSQL real; las protecciones (Multi-AZ, backups, borrado) solo son CRUD.
resource "aws_db_instance" "main" {
  identifier     = "${var.project}-${var.environment}"
  engine         = "postgres"
  engine_version = "16"
  instance_class = var.db_instance_class

  allocated_storage = 20
  storage_type      = "gp3"
  storage_encrypted = true
  kms_key_id        = aws_kms_key.main.arn

  db_name  = var.db_name
  username = var.db_admin_username
  # Gitleaks fija esta linea por numero en .gitleaksignore: si la mueves, actualiza las huellas.
  password = random_password.db_admin.result
  port     = 5432

  db_subnet_group_name   = aws_db_subnet_group.main.name
  vpc_security_group_ids = [aws_security_group.svc["db"].id]
  publicly_accessible    = false

  # No se aplica en LocalStack: solo CRUD (ADR 0012)
  multi_az                = var.db_multi_az
  backup_retention_period = var.db_backup_retention_days
  deletion_protection     = true
  copy_tags_to_snapshot   = true

  skip_final_snapshot       = false
  final_snapshot_identifier = "${var.project}-${var.environment}-final"

  auto_minor_version_upgrade          = true
  iam_database_authentication_enabled = true
  enabled_cloudwatch_logs_exports     = ["postgresql", "upgrade"]

  # Registro de consultas (parameter group), Performance Insights y monitoreo
  # extendido (Checkov CKV2_AWS_30, CKV_AWS_353, CKV_AWS_118).
  parameter_group_name            = aws_db_parameter_group.main.name
  performance_insights_enabled    = true
  performance_insights_kms_key_id = aws_kms_key.main.arn
  monitoring_interval             = 60
  monitoring_role_arn             = aws_iam_role.rds_monitoring.arn
}

# Registra las sentencias que modifican datos y las consultas lentas (> 1 s).
resource "aws_db_parameter_group" "main" {
  name   = "${var.project}-${var.environment}-postgres16"
  family = "postgres16"

  parameter {
    name  = "log_statement"
    value = "ddl"
  }

  parameter {
    name  = "log_min_duration_statement"
    value = "1000"
  }

  # TLS obligatorio en el servidor (CKV2_AWS_69); las URL de conexion ya usan sslmode=require.
  parameter {
    name  = "rds.force_ssl"
    value = "1"
  }
}

data "aws_iam_policy_document" "rds_monitoring_assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["monitoring.rds.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "rds_monitoring" {
  name               = "${var.project}-${var.environment}-rds-monitoring"
  assume_role_policy = data.aws_iam_policy_document.rds_monitoring_assume.json
}

resource "aws_iam_role_policy_attachment" "rds_monitoring" {
  role       = aws_iam_role.rds_monitoring.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonRDSEnhancedMonitoringRole"
}
