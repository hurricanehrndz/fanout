{
  description = "Fanout CoreDNS plugin development environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go_1_24
            golangci-lint
            gotools
            gotestsum
            yamllint
            shellcheck
            python3
          ];

          shellHook = ''
            if ! command -v go-header &> /dev/null; then
              echo "Installing go-header..."
              go install github.com/denis-tingajkin/go-header@v0.2.2
            fi

            echo "Development environment loaded!"
            echo "Go version: $(go version)"
            echo "golangci-lint version: $(golangci-lint --version | head -1)"
            echo ""
            echo "Available commands:"
            echo "  go build -race ./..."
            echo "  go test -race -short \$(go list ./...)"
            echo "  golangci-lint run"
            echo "  go-header"
            echo "  yamllint -c .yamllint.yml --strict ."
          '';
        };
      }
    );
}
