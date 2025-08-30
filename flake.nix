{
  description = "A Go replacement for aws eks get-token";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.05";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
      in
      {
        devShell = pkgs.mkShell {
          buildInputs = [
            pkgs.go_1_22
            pkgs.golangci-lint
          ];
        };
        packages.default = pkgs.buildGoModule {
          name = "go-aws-eks-get-token";
          src = ./.;
          vendorSha256 = null;
        };
      }
    );
}