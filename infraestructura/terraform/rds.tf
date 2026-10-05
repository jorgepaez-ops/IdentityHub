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
}
