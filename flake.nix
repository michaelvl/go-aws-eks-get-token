{
  description = "A caching replacement for aws eks get-token implemented in Go";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs { inherit system; };
      in
      {
        packages.default = pkgs.buildGoModule {
          pname = "go-aws-eks-get-token";
          version = "0.1.0";
          src = ./.;
          vendorHash = "sha256-IGUUb6RglMT/WkYWkcg0HBt0WwVzUyX4lBmygHdpYUQ=";
          # vendorHash = nixpkgs.lib.fakeHash;
        };
      }
    );
}
