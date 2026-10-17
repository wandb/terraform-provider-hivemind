# terraform-provider-hivemind

Terraform provider for HiveMind (AgentStream) Forager resources.

| Type | Name | Notes |
| --- | --- | --- |
| resource | `hivemind_forager_org_service_integration` | Org-wide credential-injecting proxy integration (e.g. Datadog). `headers_wo` is write-only. |
| resource | `hivemind_forager_sandbox_secret` | Secret in a sandbox provider's store. `value_wo` is write-only. |
| resource | `hivemind_forager_environment` | Sandbox environment. Destroy archives unless `hard_delete = true`. |
| data source | `hivemind_forager_integration_teams` | Team slugs valid for `allowed_teams`. |
| data source | `hivemind_forager_sandbox_providers` | Runtimes enabled for the org. |

See [`examples/datadog.tf`](examples/datadog.tf).

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

## Secrets and state

Write-only attributes require Terraform 1.11+. Their values are never stored in state or plan
files, so Terraform can't detect when they change: bump `headers_wo_version` /
`value_wo_version` to re-send them. Every update to an org service integration re-sends
`headers_wo` from config because the API replaces the whole definition; omitting `headers_wo`
removes stored credentials.

## Development

```sh
go test ./...          # drives the real terraform binary against an in-memory fake API
golangci-lint run ./...
```

To use a local build, add a `dev_overrides` entry for `registry.terraform.io/wandb/hivemind`
pointing at `$(go env GOPATH)/bin` in `~/.terraformrc` and run `go install .`.
