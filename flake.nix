{
  description = "A caching replacement for aws eks get-token implemented in Go";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
      in
      {
        packages.default = pkgs.buildGoModule {
          pname = "go-aws-eks-get-token";
          version = "0.1.0";
          src = ./.;
          vendorHash = "sha256-PowXwMRH8JBVjxaJQDPPx0yZzmDZTuKNfwJ6tK6uS7o=";
          # vendorHash = nixpkgs.lib.fakeHash;
        };
      }
    );
}
