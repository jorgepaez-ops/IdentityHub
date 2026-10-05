# Ningun valor de secreto se expone: solo identificadores y ARN.

output "alb_dns_name" {
  description = "DNS del ALB."
  value       = aws_lb.main.dns_name
}

output "hub_url" {
  description = "URL publica del Hub."
  value       = "https://${var.hub_host}"
}

output "contabilidad_url" {
  description = "URL publica de Contabilidad."
  value       = "https://${var.contabilidad_host}"
}

output "ecs_cluster_name" {
  description = "Nombre del cluster ECS."
  value       = aws_ecs_cluster.main.name
}

output "migrate_task_definition" {
  description = "Task definition de la migracion de un solo uso."
  value       = aws_ecs_task_definition.migrate.arn
}

output "private_subnet_ids" {
  description = "Subredes privadas, para aws ecs run-task."
  value       = aws_subnet.private[*].id
}

output "migrate_security_group_id" {
  description = "Security group de la tarea migrate."
  value       = aws_security_group.svc["migrate"].id
}

output "rds_endpoint" {
  description = "Endpoint de RDS (no es un secreto)."
  value       = aws_db_instance.main.address
}

output "secret_arns" {
  description = "ARN de los secretos en Secrets Manager (no sus valores)."
  value       = local.secret_arn
}

output "kms_key_arn" {
  description = "ARN de la clave KMS."
  value       = aws_kms_key.main.arn
}

output "name_servers" {
  description = "Servidores de nombres de la zona; delegar el dominio a estos."
  value       = aws_route53_zone.main.name_servers
}
