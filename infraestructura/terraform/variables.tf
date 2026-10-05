variable "region" {
  description = "Region de AWS."
  type        = string
  default     = "us-east-1"
}

variable "project" {
  description = "Nombre del proyecto, prefijo de los recursos."
  type        = string
  default     = "identity-hub"
}

variable "environment" {
  description = "Entorno de despliegue."
  type        = string
  default     = "prod"
}

variable "localstack" {
  description = "true al aplicar contra LocalStack: usa Mailpit en ECS en lugar de SES y omite la validacion DNS de ACM, que LocalStack solo simula."
  type        = bool
  default     = false
}

# ── Dominios ────────────────────────────────────────────────────────────────
variable "zone_domain" {
  description = "Dominio de la zona de Route 53 (por ejemplo identityhub.example.com)."
  type        = string
}

variable "hub_host" {
  description = "Host publico del Hub (por ejemplo identityhub.example.com)."
  type        = string
}

variable "contabilidad_host" {
  description = "Host publico de Contabilidad (por ejemplo contabilidad.identityhub.example.com)."
  type        = string
}

variable "internal_domain" {
  description = "Namespace DNS privado de Cloud Map; hace que api, broker y db resuelvan como en el compose."
  type        = string
  default     = "identityhub.internal"
}

# ── Imagenes (Docker Hub por digest, ADR 0011) ─────────────────────────────
variable "image_api" {
  description = "Imagen de la API por digest, por ejemplo docker.io/<ns>/identity-api@sha256:<digest>."
  type        = string
}

variable "image_worker" {
  description = "Imagen del worker por digest."
  type        = string
}

variable "image_web" {
  description = "Imagen de Nginx con las dos SPA, por digest."
  type        = string
}

variable "image_migrate" {
  description = "Imagen de migrate con las migraciones de db/migrations incluidas (el compose las monta como volumen; en ECS deben ir en la imagen)."
  type        = string
}

variable "image_broker" {
  description = "Imagen de RabbitMQ, el mismo digest que usa el compose."
  type        = string
  default     = "docker.io/library/rabbitmq@sha256:ddc75301edf58a8332934cf2d801be7cbf8d65c6458d747364a8046238ff1c89"
}

variable "image_mailpit" {
  description = "Imagen de Mailpit; solo se usa con localstack = true."
  type        = string
  default     = "docker.io/axllent/mailpit:v1.20"
}

# ── Aplicacion ─────────────────────────────────────────────────────────────
variable "app_version" {
  description = "Valor de APP_VERSION para api y worker."
  type        = string
  default     = "1.0.0"
}

variable "bootstrap_admin_email" {
  description = "Correo del administrador inicial (BOOTSTRAP_ADMIN_EMAIL); vacio si no se usa."
  type        = string
  default     = ""
}

variable "ses_from_address" {
  description = "Remitente SMTP_FROM; en produccion debe pertenecer al dominio verificado en SES."
  type        = string
  default     = "no-reply@identityhub.example.com"
}

variable "app_desired_count" {
  description = "Numero de tareas de api y web."
  type        = number
  default     = 2
}

# ── Red y datos ────────────────────────────────────────────────────────────
variable "nat_gateway_per_az" {
  description = "true: una NAT y una tabla de rutas privada por zona (referencia de produccion). false: una sola NAT, mas barata pero sin tolerancia a la caida de su zona."
  type        = bool
  default     = true
}

variable "vpc_cidr" {
  description = "CIDR de la VPC."
  type        = string
  default     = "10.20.0.0/16"
}

variable "db_name" {
  description = "Base de datos de la aplicacion (POSTGRES_DB del compose)."
  type        = string
  default     = "identity"
}

variable "db_admin_username" {
  description = "Usuario administrador de RDS (POSTGRES_USER del compose)."
  type        = string
  default     = "identity_admin"
}

variable "db_instance_class" {
  description = "Clase de instancia de RDS."
  type        = string
  default     = "db.t4g.micro"
}

variable "db_multi_az" {
  description = "Despliegue Multi-AZ de RDS."
  type        = bool
  default     = true
}

variable "db_backup_retention_days" {
  description = "Dias de retencion de backups de RDS."
  type        = number
  default     = 7
}

variable "log_retention_days" {
  description = "Retencion de los grupos de CloudWatch Logs."
  type        = number
  default     = 365
}
