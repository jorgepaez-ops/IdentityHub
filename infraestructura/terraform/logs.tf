locals {
  services = ["api", "worker", "web", "broker", "mailpit", "migrate"]
}

resource "aws_cloudwatch_log_group" "svc" {
  for_each = toset(local.services)

  name              = "/${var.project}/${var.environment}/${each.key}"
  retention_in_days = var.log_retention_days
  kms_key_id        = aws_kms_key.main.arn
}

# ── Flow logs de la VPC (Checkov CKV2_AWS_11) ───────────────────────────────
# Dentro del prefijo /<project>/<environment>/ para que la politica de la clave KMS los cubra.
resource "aws_cloudwatch_log_group" "vpc_flow" {
  name              = "/${var.project}/${var.environment}/vpc-flow"
  retention_in_days = var.log_retention_days
  kms_key_id        = aws_kms_key.main.arn
}

data "aws_iam_policy_document" "flow_assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["vpc-flow-logs.amazonaws.com"]
    }
  }
}

data "aws_iam_policy_document" "flow_write" {
  statement {
    actions   = ["logs:CreateLogStream", "logs:PutLogEvents", "logs:DescribeLogStreams"]
    resources = ["${aws_cloudwatch_log_group.vpc_flow.arn}:*"]
  }
}

resource "aws_iam_role" "vpc_flow" {
  name               = "${var.project}-${var.environment}-vpc-flow"
  assume_role_policy = data.aws_iam_policy_document.flow_assume.json
}

resource "aws_iam_role_policy" "vpc_flow" {
  name   = "escribir-flow-logs"
  role   = aws_iam_role.vpc_flow.id
  policy = data.aws_iam_policy_document.flow_write.json
}

resource "aws_flow_log" "main" {
  vpc_id          = aws_vpc.main.id
  traffic_type    = "ALL"
  log_destination = aws_cloudwatch_log_group.vpc_flow.arn
  iam_role_arn    = aws_iam_role.vpc_flow.arn
}
