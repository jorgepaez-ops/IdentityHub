# Logs de acceso del ALB. No se aplica en LocalStack: solo CRUD (ADR 0012)
# Los logs del ALB solo admiten cifrado SSE-S3 (AES256); SSE-KMS no esta soportado por ELB.

resource "aws_s3_bucket" "alb_logs" {
  #checkov:skip=CKV_AWS_144:La replicacion entre regiones de los logs queda fuera del alcance de una referencia que nunca se aplica
  #checkov:skip=CKV2_AWS_62:Las notificaciones de eventos sobre el bucket de logs no aportan a la referencia
  #checkov:skip=CKV_AWS_145:Los logs de acceso del ALB no admiten SSE-KMS; el bucket usa SSE-S3 (AES256), el unico cifrado soportado
  #checkov:skip=CKV_AWS_18:Este es el bucket de destino de los logs; activar logging sobre si mismo crearia un ciclo sin valor
  bucket        = "${var.project}-${var.environment}-alb-logs-${data.aws_caller_identity.current.account_id}"
  force_destroy = false
}

resource "aws_s3_bucket_public_access_block" "alb_logs" {
  bucket                  = aws_s3_bucket.alb_logs.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_ownership_controls" "alb_logs" {
  bucket = aws_s3_bucket.alb_logs.id
  rule {
    object_ownership = "BucketOwnerEnforced"
  }
}

# Excepcion aceptada de Trivy (AWS-0132): el ALB no puede entregar logs a un bucket cifrado con
# SSE-KMS (solo SSE-S3), asi que una clave propia haria fallar la entrega.
#trivy:ignore:AWS-0132
resource "aws_s3_bucket_server_side_encryption_configuration" "alb_logs" {
  bucket = aws_s3_bucket.alb_logs.id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_versioning" "alb_logs" {
  bucket = aws_s3_bucket.alb_logs.id
  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_lifecycle_configuration" "alb_logs" {
  bucket = aws_s3_bucket.alb_logs.id

  rule {
    id     = "expira-logs"
    status = "Enabled"
    filter {}

    expiration {
      days = var.log_retention_days
    }
    noncurrent_version_expiration {
      noncurrent_days = var.log_retention_days
    }
    abort_incomplete_multipart_upload {
      days_after_initiation = 7
    }
  }

  depends_on = [aws_s3_bucket_versioning.alb_logs]
}

data "aws_iam_policy_document" "alb_logs" {
  statement {
    sid       = "EntregaDeLogsDelALB"
    actions   = ["s3:PutObject"]
    resources = ["${aws_s3_bucket.alb_logs.arn}/${local.alb_log_prefix}/AWSLogs/${data.aws_caller_identity.current.account_id}/*"]
    # Regiones anteriores a agosto de 2022 (us-east-1 entre ellas) entregan con la cuenta de ELB
    # de la región; las más nuevas, con el principal de servicio. Se autorizan ambos.
    principals {
      type        = "AWS"
      identifiers = [data.aws_elb_service_account.main.arn]
    }
    principals {
      type        = "Service"
      identifiers = ["logdelivery.elasticloadbalancing.amazonaws.com"]
    }
  }

  statement {
    sid       = "SoloTLS"
    effect    = "Deny"
    actions   = ["s3:*"]
    resources = [aws_s3_bucket.alb_logs.arn, "${aws_s3_bucket.alb_logs.arn}/*"]
    principals {
      type        = "*"
      identifiers = ["*"]
    }
    condition {
      test     = "Bool"
      variable = "aws:SecureTransport"
      values   = ["false"]
    }
  }
}

resource "aws_s3_bucket_policy" "alb_logs" {
  bucket = aws_s3_bucket.alb_logs.id
  policy = data.aws_iam_policy_document.alb_logs.json

  depends_on = [aws_s3_bucket_public_access_block.alb_logs]
}

data "aws_elb_service_account" "main" {}

locals {
  # Un solo valor para el prefijo del ALB (alb.tf) y la ruta que autoriza la política.
  alb_log_prefix = "alb"
}
