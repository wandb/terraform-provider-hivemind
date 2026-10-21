variable "base_image" {
  type        = string
  description = "Base image ID from data.hivemind_forager_base_images."
}

resource "hivemind_forager_environment" "example" {
  name           = "example"
  base_image     = var.base_image
  setup_script   = "echo Ready"
  startup_mode   = "on_demand"
  docker_enabled = false
  # Destroy archives this environment by default.
}
