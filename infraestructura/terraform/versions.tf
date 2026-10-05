terraform {
  required_version = ">= 1.6"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
  }

  # Backend remoto descrito, no configurado (ADR 0012, ver README):
  # backend "s3" {
  #   bucket       = "<bucket-de-estado>"
  #   key          = "identity-hub/prod/terraform.tfstate"
  #   region       = "us-east-1"
  #   encrypt      = true
  #   kms_key_id   = "<arn-de-la-clave-kms-del-estado>"
  #   use_lockfile = true
  # }
}
