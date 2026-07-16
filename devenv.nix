{ pkgs, config, ... }:

{
  # Rolling nixpkgs keeps only the latest patch release; Go's native toolchain manager pins the requested one.
  env = {
    GOPATH = config.env.DEVENV_STATE + "/go";
    GOTOOLCHAIN = "go1.26.5";
  };

  packages = with pkgs; [
    go_1_26
    golangci-lint
    gotestsum
    shellcheck
    yamllint
  ];

  enterShell = ''
    export PATH="$GOPATH/bin:$PATH"

    if ! command -v go-header &> /dev/null; then
      echo "Installing go-header..."
      go install github.com/denis-tingajkin/go-header@v0.2.2 || return 1
    fi

    echo "Development environment loaded!"
    echo "Go version: $(go version)"
    echo "golangci-lint version: $(golangci-lint --version | head -1)"
    echo ""
    echo "Available commands:"
    echo "  go build -race ./..."
    echo "  go test -race -short \$(go list ./...)"
    echo "  golangci-lint fmt"
    echo "  golangci-lint run"
    echo "  go-header"
    echo "  yamllint -c .yamllint.yml --strict ."
  '';
}
