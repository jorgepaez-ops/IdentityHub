# Sin endpoints ni credenciales aqui: contra LocalStack, tflocal genera
# localstack_providers_override.tf (ignorado por git). Las credenciales reales
# vienen del entorno o de un perfil, nunca del codigo.
provider "aws" {
  region = var.region

  default_tags {
    tags = {
      Project     = var.project
      Environment = var.environment
      ManagedBy   = "terraform"
    }
  }
}

provider "random" {}
