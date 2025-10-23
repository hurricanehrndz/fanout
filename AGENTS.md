# Agent Guidelines for Fanout

## Build/Test Commands
- **Build**: `go build -race ./...`
- **Test all**: `go test -race -short $(go list ./...)`
- **Test single package**: `go test -race -short ./path/to/package`
- **Test specific test**: `go test -race -short -run TestName ./path/to/package`
- **Lint**: `golangci-lint run` (requires golangci-lint v1.59.1+)
- **Format**: `gofmt -w .` and `goimports -w .`

## Code Style
- **License header**: All `.go` files require Apache 2.0 license header (see `.license/template.txt`)
- **Imports**: Use `goimports` with `local-prefixes: github.com/networkservicemesh/sdk`; standard library → third-party → local packages
- **Error handling**: MUST use `github.com/pkg/errors` (errors.Wrap, errors.Wrapf, errors.New, errors.Errorf); NEVER use `fmt.Errorf` or stdlib `errors`
- **Formatting**: Use `gofmt`; follow standard Go conventions
- **Naming**: Standard Go naming (camelCase for unexported, PascalCase for exported)
- **Comments**: Public symbols require doc comments; use godoc format
- **Cyclomatic complexity**: Keep functions under 15 complexity
- **Dependencies**: No dependencies on `github.com/networkservicemesh/*` except `github.com/networkservicemesh/api`
- **go.mod**: No `replace` directives allowed; run `go mod tidy` after dependency changes
- Align the happy path to the left edge

## Critical Rules
- Run `go mod tidy` after changing dependencies
- Never commit without license headers (`go-header` tool checks this)
- All tests must pass with race detector (`-race` flag)
