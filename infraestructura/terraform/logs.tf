locals {
  services = ["api", "worker", "web", "broker", "mailpit", "migrate"]
}

resource "aws_cloudwatch_log_group" "svc" {
  for_each = toset(local.services)

  name              = "/${var.project}/${var.environment}/${each.key}"
  retention_in_days = var.log_retention_days
  kms_key_id        = aws_kms_key.main.arn
}
