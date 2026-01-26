{
  description = "Flake for spark-personal-assistant development";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils, ... }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; config = {}; };
      in {
        devShell = pkgs.mkShell {
          name = "spark-dev";
          buildInputs = with pkgs; [
            go_1_24
            git
            gnumake
            direnv
            golangci-lint
          ];

          shellHook = ''
            export GOCACHE="$PWD/.cache/go"
            export GOPATH="$HOME/go"
            export PATH="$GOPATH/bin:$PATH"
          '';
        };
      });
}
