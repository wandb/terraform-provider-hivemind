data "hivemind_forager_sandbox_providers" "available" {}

output "sandbox_providers" {
  value = data.hivemind_forager_sandbox_providers.available.providers
}
