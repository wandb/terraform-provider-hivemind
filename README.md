# terraform-provider-hivemind

Terraform provider for HiveMind Forager resources. Requires Terraform 1.11 or later.

| Type | Name | Notes |
| --- | --- | --- |
| resource | `hivemind_forager_org_service_integration` | Org-wide credential-injecting proxy integration (e.g. Datadog). `headers_wo` is write-only. |
| resource | `hivemind_forager_sandbox_secret` | Secret in a sandbox provider's store. `value_wo` is write-only. |
| resource | `hivemind_forager_environment` | Sandbox environment. Destroy archives unless `hard_delete = true`. |
| data source | `hivemind_forager_integration_teams` | Team slugs valid for `allowed_teams`. |
| data source | `hivemind_forager_sandbox_providers` | Runtimes enabled for the org. |
| data source | `hivemind_forager_base_images` | Catalog image IDs valid for `base_image`. |

See [`examples/datadog.tf`](examples/datadog.tf) and the [provider reference](docs/index.md).

## Installation

Public Registry publication is being prepared. The following configuration works
once the first signed release has been published and indexed in the Registry:

```hcl
terraform {
  required_version = ">= 1.11.0"
  required_providers {
    hivemind = {
      source  = "wandb/hivemind"
      version = "~> 0.1"
    }
  }
}

# Authenticate through HIVEMIND_TOKEN or OIDC; see below.
provider "hivemind" {}
```

Run `terraform init` to install the provider. Until publication, use the local
build instructions below. Maintainers should follow the [publication guide](docs/publishing.md),
including signing-key setup and a clean Registry installation check.

### Environment egress

On Sparkbox, `auto_detect_egress = true` records DNS lookups during the next bake and merges them into
`egress_allowlist`. The server then clears the flag. Terraform keeps your configured value and sends
it only on create or when you change it, so the cleared flag isn't drift. To get discovery again on a
later bake, set it to `false`, apply, then set it back to `true`. Leave `egress_allowlist` unset
when you use detection: Terraform then keeps the merged list instead of resetting it.

## Authentication

Create a HiveMind **service account** with scopes `["read", "manage"]` (only org admins can
create `manage` accounts; personal access tokens can't carry `manage`). Then either:

- set `HIVEMIND_TOKEN` (or `token`) to a token minted for the account, or
- run in GitHub Actions with `permissions: id-token: write` and configure the account's OIDC
  selector for the repository. The provider fetches the runner's OIDC token (audience defaults
  to the endpoint) and exchanges it at `/v1/auth/actions/exchange` for a short-lived token, or
- set `HIVEMIND_OIDC_TOKEN` (or `oidc_token`) to an id_token from another supported issuer.

`HIVEMIND_ENDPOINT` defaults to `https://hivemind.wandb.tools`. `HIVEMIND_ORG_ID` is sent as
`X-Org-Id` when set.

The `manage` scope also covers sandbox secret writes; a separate `write` scope is not required.
Secret names must use 1–64 letters, digits, or underscores and start with a letter or underscore.

## Secrets and state

Write-only attributes require Terraform 1.11+. Their values are never stored in state or plan
files, so Terraform can't detect when they change: bump `headers_wo_version` /
`value_wo_version` to re-send them. Every update to an org service integration re-sends
`headers_wo` from config because the API replaces the whole definition; omitting `headers_wo`
removes stored credentials.

## Development

Install Go matching `go.mod`, Terraform 1.11+, and golangci-lint 2.11.4.
Tests use a fake API and require no HiveMind credentials. See [CONTRIBUTING.md](CONTRIBUTING.md).

For the baked development environment, see [Developing in Forager](docs/dev-in-forager.md).

```sh
go test -race ./...    # drives the real terraform binary against an in-memory fake API
golangci-lint run ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
```

To use a local build, add a `dev_overrides` entry for `registry.terraform.io/wandb/hivemind`
pointing at `$(go env GOPATH)/bin` in `~/.terraformrc` and run `go install .`.

Generate and validate Registry docs after changing schemas, examples, or templates:

```sh
go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@v0.25.0 generate --provider-name hivemind --rendered-provider-name HiveMind
go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@v0.25.0 validate --provider-name hivemind
terraform fmt -check -recursive examples
```

CI also builds all release platforms with GoReleaser 2.18.3 in snapshot mode;
no release credentials are needed for that check.
