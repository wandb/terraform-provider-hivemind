variable "secret_value" {
  type      = string
  sensitive = true
  ephemeral = true
}

variable "sandbox_provider" {
  type        = string
  description = "A runtime with secret_store = true from hivemind_forager_sandbox_providers."
}

resource "hivemind_forager_sandbox_secret" "example" {
  sandbox_provider = var.sandbox_provider
  name             = "api-token"
  value_wo         = var.secret_value
  value_wo_version = "1"
}
