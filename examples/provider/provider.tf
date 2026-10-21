terraform {
  required_version = ">= 1.11.0"
  required_providers {
    hivemind = {
      source = "wandb/hivemind"
    }
  }
}

# Set HIVEMIND_TOKEN to a service-account token with read and manage scopes.
# Set HIVEMIND_ENDPOINT when using a different HiveMind deployment.
provider "hivemind" {}
