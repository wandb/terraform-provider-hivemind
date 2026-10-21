# Developing in Forager

For contributors with access to the **Terraform Provider HiveMind dev**
environment, its setup recipe is stored in Forager; no repository setup script
is needed. Other contributors can use the tools and checks in the README.

The existing image includes Go **1.26.0**, Terraform **1.11.4**, and
golangci-lint **2.11.4**, plus cached Go modules and an installed local
provider. Fresh default shells and Bash login shells select Go 1.26.0 without
changing `PATH`; `/usr/local/go` points to `/opt/forager-toolchains/go`.

The provider now requires Go **1.26.10** to address standard-library security
advisories. Until the managed image is updated, allow Go to download that patched
toolchain before running the checks; do not force `GOTOOLCHAIN=local` or
`GOPROXY=off` on the older image. The image's installed provider must also be
rebuilt with the patched toolchain.

```sh
export GOTOOLCHAIN=auto
export TF_ACC_TERRAFORM_PATH=/usr/local/bin/terraform
export CHECKPOINT_DISABLE=1 TF_IN_AUTOMATION=1
go version  # go1.26.10 or later
go mod verify
test -z "$(gofmt -l .)"
go vet ./...
golangci-lint run ./...
go test -race -count=1 -v ./...
```

CI selects Go from `go.mod`, Terraform `1.11.*`, and golangci-lint `2.11.4`.
It also runs `govulncheck` and distribution checks documented in the README.
Offline checks require the patched toolchain and all required modules to be
cached first.
The tests run the real Terraform CLI against an in-memory fake API, including
resource lifecycles and data sources, with no live credentials. This is a
CLI/provider repository: no persistent server or Docker is needed.

After changing provider code, rebuild the local binary with `go install .`.
The baked `~/.terraformrc` already points Terraform at `$(go env GOPATH)/bin`
(`/home/sparky/go/bin` in this image):

```hcl
provider_installation {
  dev_overrides {
    "registry.terraform.io/wandb/hivemind" = "/home/sparky/go/bin"
  }
  direct {}
}
```

In a temporary directory with a `required_providers` entry whose source is
`wandb/hivemind`, run `terraform providers schema -json` to check the installed
provider without credentials or `terraform init`. The schema exposes three
resources and two data sources listed in the README.

Managing real infrastructure requires user-supplied service-account
credentials as described in [README Authentication](../README.md#authentication).
Fake-API tests and schema inspection do not validate the live API.
