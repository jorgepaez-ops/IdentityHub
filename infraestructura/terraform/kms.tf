data "aws_caller_identity" "current" {}

# Una sola clave cifra RDS, Secrets Manager y CloudWatch Logs.
data "aws_iam_policy_document" "kms" {
  # En una politica de clave KMS el recurso "*" significa "esta misma clave", no
  # todas las de la cuenta (excepciones de Checkov documentadas, ADR 0012).
  #checkov:skip=CKV_AWS_111:En una politica de clave KMS "*" se refiere a la propia clave; la politica ya esta acotada a ella
  #checkov:skip=CKV_AWS_356:En una politica de clave KMS "*" se refiere a la propia clave; no puede nombrarse otro recurso
  #checkov:skip=CKV_AWS_109:La administracion de permisos es de la cuenta raiz y vive solo dentro de la politica de esta clave
  statement {
    sid       = "AdministracionDeLaCuenta"
    effect    = "Allow"
    actions   = ["kms:*"]
    resources = ["*"]
    principals {
      type        = "AWS"
      identifiers = ["arn:aws:iam::${data.aws_caller_identity.current.account_id}:root"]
    }
  }

  statement {
    sid       = "CloudWatchLogs"
    effect    = "Allow"
    actions   = ["kms:Encrypt*", "kms:Decrypt*", "kms:ReEncrypt*", "kms:GenerateDataKey*", "kms:Describe*"]
    resources = ["*"]
    principals {
      type        = "Service"
      identifiers = ["logs.${var.region}.amazonaws.com"]
    }
    condition {
      test     = "ArnLike"
      variable = "kms:EncryptionContext:aws:logs:arn"
      values = [
        "arn:aws:logs:${var.region}:${data.aws_caller_identity.current.account_id}:log-group:/${var.project}/${var.environment}/*",
        # Los logs del WAF exigen el prefijo aws-waf-logs- en el nombre del grupo.
        "arn:aws:logs:${var.region}:${data.aws_caller_identity.current.account_id}:log-group:aws-waf-logs-${var.project}-${var.environment}",
      ]
    }
  }
}

resource "aws_kms_key" "main" {
  description             = "${var.project}-${var.environment}: RDS, secretos y logs"
  enable_key_rotation     = true
  deletion_window_in_days = 30
  policy                  = data.aws_iam_policy_document.kms.json
}

resource "aws_kms_alias" "main" {
  name          = "alias/${var.project}-${var.environment}"
  target_key_id = aws_kms_key.main.key_id
}
