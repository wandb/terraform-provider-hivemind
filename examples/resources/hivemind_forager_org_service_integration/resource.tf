variable "datadog_access_token" {
  type      = string
  sensitive = true
  ephemeral = true
}

variable "datadog_token_version" {
  type    = string
  default = "1"
}

# Sandboxes get DD_SITE=<proxy host> and a placeholder token; the Forager
# proxy injects the real Datadog token, so agents never see it.
resource "hivemind_forager_org_service_integration" "datadog" {
  name    = "datadog"
  target  = "https://api.us5.datadoghq.com"
  methods = ["GET", "HEAD", "POST"]
  env = {
    DD_SITE         = "{proxy_host}"
    DD_ACCESS_TOKEN = "forager-proxy"
    PUP_TRUST_SITE  = "1"
  }
  headers_wo         = { Authorization = "Bearer ${var.datadog_access_token}" }
  headers_wo_version = var.datadog_token_version
}
