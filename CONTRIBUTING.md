# Contributing

Open an issue describing a bug or proposed behavior, or submit a pull request
with a focused change. Never include real tokens, state, plans, or private
organization data in examples, issues, or test fixtures.

Use Go matching `go.mod`, Terraform 1.11 or later, and golangci-lint 2.11.4.
Run these checks from the repository root:

```sh
go mod verify
test -z "$(gofmt -l .)"
go vet ./...
golangci-lint run ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
go test -race ./...
terraform fmt -check -recursive examples
```

Tests run the real Terraform CLI against an in-memory fake API. They need no
HiveMind account and do not validate compatibility with a live deployment.
For behavior changes, extend those tests with the affected lifecycle or error
case. Keep resource secrets write-only and out of state and test output.

Regenerate and validate Registry documentation using the pinned commands in
[README Development](README.md#development). Edit schemas, examples, and
`templates/`; generated `docs/index.md`, `docs/resources/`, and
`docs/data-sources/` are checked into Git and verified by CI. Maintainer and
development guides elsewhere in `docs/` are handwritten.

For local provider builds and optional Forager tooling, see the README and
[Forager development guide](docs/dev-in-forager.md). For public releases, follow
the [publication guide](docs/publishing.md).
