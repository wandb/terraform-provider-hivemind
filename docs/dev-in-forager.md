# Developing in Forager

Use **Terraform Provider HiveMind dev** (environment
`adbb6e0e-0701-4ec7-a8a9-3041cd897df9`, verified bake
`ce3d1fc1-97f9-49c7-a267-02f6e80c3a7f`). Its setup recipe is stored in
Forager; no repository setup script is needed.

The image includes Go **1.26.0** (from `go.mod`), Terraform **1.11.4**, and
golangci-lint **2.11.4**, plus cached Go modules and an installed local
provider. Select the baked Go before running checks: a fresh shell may put
the base image's Go ahead of it on `PATH`.

```sh
export PATH=/usr/local/bin:$PATH
export GOTOOLCHAIN=local GOPROXY=off
export TF_ACC_TERRAFORM_PATH=/usr/local/bin/terraform
export CHECKPOINT_DISABLE=1 TF_IN_AUTOMATION=1
go version  # go1.26.0
go mod verify
test -z "$(gofmt -l .)"
go vet ./...
golangci-lint run ./...
go test -race -count=1 -v ./...
```

These cover the CI checks; CI selects Go from `go.mod`, Terraform `1.11.*`,
and golangci-lint `2.11.4`. `GOPROXY=off` verifies the baked modules suffice.
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
