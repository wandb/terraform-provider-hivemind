# Publishing HiveMind to the Terraform Registry

The provider is published as
[`wandb/hivemind`](https://registry.terraform.io/providers/wandb/hivemind/latest).
Versions `0.1.0` and `0.1.1` are indexed in the Registry. The repository, signing
key, and Registry integration are already configured for future releases.

## Release configuration

- `.goreleaser.yml` builds static provider binaries for Linux, macOS, and Windows
  on amd64 and arm64, packages ZIPs, and signs a SHA-256 checksum file with GPG.
- `terraform-registry-manifest.json` declares plugin protocol 6.0, matching the
  Terraform Plugin Framework server in `main.go`. Protocol 6.0 is separate from
  the Terraform 1.11 minimum required by write-only attributes.
- `.github/workflows/release.yml` publishes a **signed GitHub release** on a `v*` tag,
  following the same tag-to-release flow as `wandb/terraform-provider-orca`.
  The tagged commit must be on `main`; tests and vulnerability checks run before
  signing. The workflow uses GitHub's automatic token to upload assets and needs
  no HiveMind API token or Terraform Registry token.
- CI generates and validates Registry documentation and runs an unsigned
  snapshot build for every supported release platform. Snapshot builds are
  local checks, not publishable releases.

GoReleaser's `release.draft: false` publishes the GitHub release after uploading
its assets, allowing the Registry to ingest it. Setting it to `true` leaves an
unpublished draft that a maintainer must publish manually. This setting controls
GitHub releases and is separate from a pull request's draft status.

Go 1.26.10 is the minimum build toolchain because earlier 1.26 releases have
known standard-library vulnerabilities. Keep release toolchains patched; a
successful fake-API test suite alone does not detect these vulnerabilities.

## Publish a new release

Before releasing behavior changes, validate the provider against a disposable
HiveMind organization with a service account scoped to `read` and `manage`.
Exercise the documented resource create/read/update/import/destroy paths, secret
rotation, and OIDC if offered to users. Use an environment variable style secret
name (such as `API_TOKEN`); `manage` covers secret writes without an extra `write` scope.
By default, destroying an environment archives it; sandbox secrets and service
integrations are deleted. The automated suite uses a fake API and cannot
establish compatibility with the live service.

After merging reviewed changes, choose an unused version (examples use `0.1.2`),
ensure `main` is current, and tag the reviewed commit:

```sh
git switch main
git pull --ff-only
git tag -a v0.1.2 -m 'HiveMind provider v0.1.2'
git push origin v0.1.2
```

Watch the Release workflow. Approve the `release` environment if configured.
A missing signing secret or failed test stops publication. It should produce a
published GitHub release containing:

```text
terraform-provider-hivemind_0.1.2_linux_amd64.zip
terraform-provider-hivemind_0.1.2_linux_arm64.zip
terraform-provider-hivemind_0.1.2_darwin_amd64.zip
terraform-provider-hivemind_0.1.2_darwin_arm64.zip
terraform-provider-hivemind_0.1.2_windows_amd64.zip
terraform-provider-hivemind_0.1.2_windows_arm64.zip
terraform-provider-hivemind_0.1.2_manifest.json
terraform-provider-hivemind_0.1.2_SHA256SUMS
terraform-provider-hivemind_0.1.2_SHA256SUMS.sig
```

ZIPs contain `terraform-provider-hivemind_v0.1.2` (`.exe` on Windows) and LICENSE.
Download the published assets to a fresh directory and save the namespace's
public signing key as `hivemind-public.asc`. Verify the detached signature and
every checksum:

```sh
gpg --import hivemind-public.asc
gpg --verify terraform-provider-hivemind_0.1.2_SHA256SUMS.sig terraform-provider-hivemind_0.1.2_SHA256SUMS
sha256sum --check terraform-provider-hivemind_0.1.2_SHA256SUMS
```

Compare the reported signing fingerprint with the registered key. Inspect the
ZIP names, manifest, and binary names, then confirm the new version appears in
the Registry with all six platforms and the provider/resource/data-source docs.
Do not replace published binaries or move published tags; issue a new version.

## Verify a Registry installation

Use a fresh directory and a CLI config without development overrides or mirrors.
In particular, the Forager development image has a `dev_overrides` entry, so a
successful local schema check there does not prove Registry installation works.

Create `main.tf` in that directory. This example checks the published `0.1.1`;
replace the version with the release you are verifying:

```hcl
terraform {
  required_version = ">= 1.11.0"
  required_providers {
    hivemind = {
      source  = "wandb/hivemind"
      version = "= 0.1.1"
    }
  }
}
provider "hivemind" {}
```

Create `registry.tfrc` containing only `provider_installation { direct {} }` and run:

```sh
export TF_CLI_CONFIG_FILE="$PWD/registry.tfrc"
unset TF_PLUGIN_CACHE_DIR TF_REATTACH_PROVIDERS
terraform init -backend=false
terraform providers schema -json > schema.json
terraform validate
```

`terraform init` should identify `wandb/hivemind v0.1.1` and report the expected
signing-key ID. It should create a dependency lock file with Registry checksums.
No HiveMind credentials are needed for installation, schema inspection, or
validation.

## Registry administration

The `wandb` public namespace is managed by the `weights-and-biases` HCP Terraform
organization. Open **Registry → Public namespaces → wandb** to manage the
provider, view ingestion errors, or resync releases. These operations require
organization-owner access or **Manage public providers** permission; GitHub
repository admin access alone is insufficient. See the
[HCP Terraform namespace guide](https://developer.hashicorp.com/terraform/cloud-docs/users-teams-organizations/organizations/public-namespace/artifacts).

The GitHub `release` environment contains `GPG_PRIVATE_KEY` and `PASSPHRASE`.
Reuse these for normal releases. Restrict the environment to release tags and
maintainer approvals where supported, and protect `main`, release tags, and
workflow changes.

For signing-key rotation, keep the private key and passphrase in the team's
secret manager. Register the replacement public key in the namespace before
signing releases with it, and update the GitHub environment secrets. Keep old
public keys registered so users can verify existing versions. Follow the current
[Registry signing requirements](https://developer.hashicorp.com/terraform/registry/providers/publishing#preparing-and-adding-a-signing-key)
and [HCP Terraform key-management instructions](https://developer.hashicorp.com/terraform/cloud-docs/users-teams-organizations/organizations/public-namespace/manage).

If a new release is missing from the Registry, check that it is published on
GitHub, inspect HCP Terraform's ingestion errors, and verify GitHub App access,
release asset names, checksum coverage, and the registered signing key. Use
**Resync** on the provider in **Public namespaces** after correcting the cause.
