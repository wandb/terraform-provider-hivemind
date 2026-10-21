# Publishing HiveMind to the Terraform Registry

The provider address is `registry.terraform.io/wandb/hivemind`. Making the GitHub
repository public exposes the source and history; Terraform installation also
requires signed release assets and registration with the public Terraform
Registry. The existing Apache 2.0 license permits public distribution.

## What this repository supplies

- `.goreleaser.yml` builds static provider binaries for Linux, macOS, and Windows
  on amd64 and arm64, packages ZIPs, and signs a SHA-256 checksum file with GPG.
- `terraform-registry-manifest.json` declares plugin protocol 6.0, matching the
  Terraform Plugin Framework server in `main.go`. Protocol 6.0 is separate from
  the Terraform 1.11 minimum required by write-only attributes.
- `.github/workflows/release.yml` builds a **draft GitHub release** on a `v*` tag.
  The tagged commit must be on `main`; tests and vulnerability checks run before signing. The workflow
  uses GitHub's automatic token to upload assets and needs no HiveMind API token
  or Terraform Registry token.
- CI generates and validates Registry documentation and runs an unsigned
  snapshot build for every supported release platform. Snapshot builds are
  local checks, not publishable releases.

Go 1.26.10 is the minimum build toolchain because earlier 1.26 releases have
known standard-library vulnerabilities. Keep release toolchains patched; a
successful fake-API test suite alone does not detect these vulnerabilities.

## 1. Review before changing visibility

Merge the preparation PR after its checks pass. Review the complete Git history,
branches, tags, releases, issues, PRs, and attachments for credentials or private
material. A local Gitleaks scan of all fetched refs is useful, but cannot inspect
all GitHub-hosted discussions or guarantee the absence of private information.
Rotate any exposed credentials before publishing; deleting them from the latest
commit is insufficient. Enable GitHub secret scanning and push protection where
available, and review repository access and Actions permissions.

Before the first public release, validate the provider against a disposable
HiveMind organization with a service account scoped to `read` and `manage`.
Exercise the documented resource create/read/update/import/destroy paths, secret
rotation, and OIDC if offered to users. Use an environment variable style secret name
(such as `API_TOKEN`); `manage` covers secret writes without an extra `write` scope.
By default, destroying an environment
archives it; sandbox secrets and service integrations are deleted. The automated
suite uses a fake API and cannot establish compatibility with the live service.

An organization owner/admin should confirm that this source, API surface,
examples, and license are approved for publication. Then change
[`wandb/terraform-provider-hivemind` visibility](https://github.com/wandb/terraform-provider-hivemind/settings)
to public in GitHub Settings. This preparation does not change visibility.

## 2. Set up the Registry publisher and signing key

1. Sign in to [Terraform Registry](https://registry.terraform.io/) with a GitHub
   account permitted to publish for the `wandb` organization. Authorize the
   Registry's GitHub integration for the repository; organization OAuth approval
   may be required. Confirm access to the `wandb` namespace before tagging.
2. Create a dedicated, passphrase-protected GPG key with signing capability under
   the maintainer team's control. Store its backup and passphrase in the team's
   secret manager, and record the full fingerprint and expiration. Do not commit
   the private key, passphrase, or exported key files.
3. Export the **public** key as ASCII armor and add it to the Registry publisher's
   signing keys for the namespace. Register it before publishing the release.
   The public key must match the private key used by the release workflow.
4. In GitHub Settings → Environments, create `release`. Restrict it to release
   tags and configure required maintainer reviewers where supported. Anyone able
   to run modified release workflow code could otherwise access the signing key;
   protect `main`, release tags, and workflow changes accordingly.
5. Add environment secrets named `GPG_PRIVATE_KEY` (ASCII-armored private key) and
   `PASSPHRASE` (its passphrase). The workflow imports the key in the runner and
   passes its fingerprint to GoReleaser. `GITHUB_TOKEN` is supplied automatically.

For example, on a trusted maintainer workstation with GPG installed:

```sh
gpg --full-generate-key
gpg --list-secret-keys --keyid-format LONG
# Replace FINGERPRINT with the full signing-key fingerprint.
gpg --armor --export FINGERPRINT > hivemind-public.asc
# Store this file securely and upload its content only as GPG_PRIVATE_KEY.
gpg --armor --export-secret-keys FINGERPRINT > hivemind-private.asc
```

Follow the current [Registry publishing requirements](https://developer.hashicorp.com/terraform/registry/providers/publishing)
and [signing requirements](https://developer.hashicorp.com/terraform/registry/providers/publishing#signing-releases).
The Registry's UI and organization authorization can change; the namespace owner
must complete these account-level steps.

## 3. Build and inspect the first release

After the changes are merged, choose the first version (examples use `0.1.0`),
ensure `main` is current, and tag the reviewed commit:

```sh
git switch main
git pull --ff-only
git tag -a v0.1.0 -m 'HiveMind provider v0.1.0'
git push origin v0.1.0
```

Watch the Release workflow. Approve the `release` environment if configured.
A missing signing secret or failed test stops publication. It should produce a
draft GitHub release containing:

```text
terraform-provider-hivemind_0.1.0_linux_amd64.zip
terraform-provider-hivemind_0.1.0_linux_arm64.zip
terraform-provider-hivemind_0.1.0_darwin_amd64.zip
terraform-provider-hivemind_0.1.0_darwin_arm64.zip
terraform-provider-hivemind_0.1.0_windows_amd64.zip
terraform-provider-hivemind_0.1.0_windows_arm64.zip
terraform-provider-hivemind_0.1.0_manifest.json
terraform-provider-hivemind_0.1.0_SHA256SUMS
terraform-provider-hivemind_0.1.0_SHA256SUMS.sig
```

ZIPs contain `terraform-provider-hivemind_v0.1.0` (`.exe` on Windows) and LICENSE.
Download the draft assets to a fresh directory and verify the detached signature
and every checksum using the registered public key:

```sh
gpg --import hivemind-public.asc
gpg --verify terraform-provider-hivemind_0.1.0_SHA256SUMS.sig terraform-provider-hivemind_0.1.0_SHA256SUMS
sha256sum --check terraform-provider-hivemind_0.1.0_SHA256SUMS
```

Compare the reported signing fingerprint with the registered key. Add user-facing
release notes, inspect the ZIP names, manifest, and binary names, then publish the
draft as a normal release. Draft releases cannot be installed from the Registry.
Do not replace published binaries or move published tags; issue a new version.

## 4. Register the provider

In the Registry, choose **Publish → Provider**, select the GitHub organization
and `terraform-provider-hivemind` repository, and complete publication. The repo
must be public and have a qualifying signed release. Confirm that version
`0.1.0`, all six platforms, and the provider/resource/data-source docs appear at
[the provider page](https://registry.terraform.io/providers/wandb/hivemind/latest).
If ingestion fails, check Registry errors, GitHub authorization/webhook access,
release asset names, checksum coverage, and the registered signing key.

The Registry normally imports later versions through its GitHub integration
when their releases become public. Future tags still create draft releases in
this workflow; a maintainer must inspect and publish each draft. Monitor key
expiration and register replacement keys before signing releases with them.

## 5. Verify a real Registry installation

Use a fresh directory and a CLI config without development overrides or mirrors.
In particular, the Forager development image has a `dev_overrides` entry, so a
successful local schema check there does not prove Registry installation works.

Create `main.tf` in that directory:

```hcl
terraform {
  required_version = ">= 1.11.0"
  required_providers {
    hivemind = {
      source  = "wandb/hivemind"
      version = "= 0.1.0"
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

`terraform init` should identify `wandb/hivemind v0.1.0` and report the expected
signing-key ID. It should create a dependency lock file with Registry checksums.
No HiveMind credentials are needed for installation, schema inspection, or
validation. Next authenticate with a service account and perform the live smoke
check in a disposable organization before announcing general availability.

After successful publication, remove the README's pre-publication notice and
adjust its example version constraint if the first version differs from 0.1.0.
