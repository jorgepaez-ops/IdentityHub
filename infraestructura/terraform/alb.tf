# Un ALB publico; reglas por Host que reproducen los dos server{} de Nginx.
# ELB y reglas ejecutan de verdad en LocalStack.

resource "aws_lb" "main" {
  name                       = "${var.project}-${var.environment}"
  load_balancer_type         = "application"
  internal                   = false
  subnets                    = aws_subnet.public[*].id
  security_groups            = [aws_security_group.svc["alb"].id]
  drop_invalid_header_fields = true
  enable_deletion_protection = true
  # Sin access_logs a proposito (exige un bucket S3 con su politica): trabajo futuro.
  #checkov:skip=CKV_AWS_91:Los access logs exigen un bucket S3 con politica propia; la referencia nunca se aplica y queda como trabajo futuro
}

resource "aws_lb_target_group" "web" {
  #checkov:skip=CKV_AWS_378:TLS termina en el ALB y el trafico hacia web viaja por subredes privadas; TLS extremo a extremo es trabajo futuro
  name        = "${var.project}-${var.environment}-web"
  port        = 8080
  protocol    = "HTTP"
  target_type = "ip"
  vpc_id      = aws_vpc.main.id

  health_check {
    path    = "/healthz"
    matcher = "200"
  }
}

# No se aplica en LocalStack: solo CRUD (ADR 0012)  (el certificado ACM)
resource "aws_lb_listener" "https" {
  load_balancer_arn = aws_lb.main.arn
  port              = 443
  protocol          = "HTTPS"
  ssl_policy        = "ELBSecurityPolicy-TLS13-1-2-2021-06"
  certificate_arn   = local.certificate_arn

  default_action {
    type = "fixed-response"
    fixed_response {
      content_type = "text/plain"
      message_body = "Host no reconocido"
      status_code  = "404"
    }
  }
}

# El puerto 80 solo redirige a HTTPS (301); no sirve contenido.
resource "aws_lb_listener" "http" {
  load_balancer_arn = aws_lb.main.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type = "redirect"
    redirect {
      port        = "443"
      protocol    = "HTTPS"
      status_code = "HTTP_301"
    }
  }
}

resource "aws_lb_listener_rule" "hosts" {
  listener_arn = aws_lb_listener.https.arn
  priority     = 10

  condition {
    host_header {
      values = [var.hub_host, var.contabilidad_host]
    }
  }

  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.web.arn
  }
}
