# ACM, Route 53 y WAFv2: en LocalStack son solo CRUD (se crean, no filtran ni
# resuelven nada). Se escriben para la nube real y para Checkov (ADR 0012).

# No se aplica en LocalStack: solo CRUD (ADR 0012)
resource "aws_route53_zone" "main" {
  name = var.zone_domain
}

# No se aplica en LocalStack: solo CRUD (ADR 0012)
resource "aws_acm_certificate" "main" {
  domain_name               = var.hub_host
  subject_alternative_names = [var.contabilidad_host]
  validation_method         = "DNS"

  lifecycle {
    create_before_destroy = true
  }
}

# No se aplica en LocalStack: solo CRUD (ADR 0012)
resource "aws_route53_record" "cert_validation" {
  for_each = {
    for dvo in aws_acm_certificate.main.domain_validation_options : dvo.domain_name => {
      name   = dvo.resource_record_name
      record = dvo.resource_record_value
      type   = dvo.resource_record_type
    }
  }

  zone_id         = aws_route53_zone.main.zone_id
  name            = each.value.name
  type            = each.value.type
  records         = [each.value.record]
  ttl             = 60
  allow_overwrite = true
}

# La espera de validacion se omite con localstack = true: LocalStack no
# valida DNS y el apply quedaria esperando.
resource "aws_acm_certificate_validation" "main" {
  count = var.localstack ? 0 : 1

  certificate_arn         = aws_acm_certificate.main.arn
  validation_record_fqdns = [for r in aws_route53_record.cert_validation : r.fqdn]
}

locals {
  certificate_arn = var.localstack ? aws_acm_certificate.main.arn : aws_acm_certificate_validation.main[0].certificate_arn
}

# No se aplica en LocalStack: solo CRUD (ADR 0012)
resource "aws_route53_record" "hosts" {
  for_each = toset([var.hub_host, var.contabilidad_host])

  zone_id = aws_route53_zone.main.zone_id
  name    = each.key
  type    = "A"

  alias {
    name                   = aws_lb.main.dns_name
    zone_id                = aws_lb.main.zone_id
    evaluate_target_health = true
  }
}

# ── WAFv2 sobre el ALB ──────────────────────────────────────────────────────
# No se aplica en LocalStack: solo CRUD (ADR 0012)
resource "aws_wafv2_web_acl" "alb" {
  name  = "${var.project}-${var.environment}"
  scope = "REGIONAL"

  default_action {
    allow {}
  }

  rule {
    name     = "reglas-comunes"
    priority = 10
    override_action {
      none {}
    }
    statement {
      managed_rule_group_statement {
        name        = "AWSManagedRulesCommonRuleSet"
        vendor_name = "AWS"
      }
    }
    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name                = "reglas-comunes"
      sampled_requests_enabled   = true
    }
  }

  rule {
    name     = "entradas-maliciosas-conocidas"
    priority = 20
    override_action {
      none {}
    }
    statement {
      managed_rule_group_statement {
        name        = "AWSManagedRulesKnownBadInputsRuleSet"
        vendor_name = "AWS"
      }
    }
    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name                = "entradas-maliciosas"
      sampled_requests_enabled   = true
    }
  }

  # Complementa el limit_req de Nginx sobre /api/v1/auth/ (T29).
  rule {
    name     = "limite-por-ip"
    priority = 30
    action {
      block {}
    }
    statement {
      rate_based_statement {
        limit              = 1000
        aggregate_key_type = "IP"
      }
    }
    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name                = "limite-por-ip"
      sampled_requests_enabled   = true
    }
  }

  visibility_config {
    cloudwatch_metrics_enabled = true
    metric_name                = "${var.project}-${var.environment}"
    sampled_requests_enabled   = true
  }
}

# No se aplica en LocalStack: solo CRUD (ADR 0012)
resource "aws_wafv2_web_acl_association" "alb" {
  resource_arn = aws_lb.main.arn
  web_acl_arn  = aws_wafv2_web_acl.alb.arn
}

# ── SES (solo produccion) ───────────────────────────────────────────────────
# El worker aun no autentica ni usa STARTTLS (ADR 0012): se deja la identidad
# lista para cuando lo soporte. No se aplica en LocalStack: solo CRUD (ADR 0012)
resource "aws_ses_domain_identity" "main" {
  count = var.localstack ? 0 : 1

  domain = var.zone_domain
}

resource "aws_ses_domain_dkim" "main" {
  count = var.localstack ? 0 : 1

  domain = aws_ses_domain_identity.main[0].domain
}
