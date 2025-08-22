{
  description = "A very basic flake";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-25.05";
    flake-parts.url = "github:hercules-ci/flake-parts";
  };

  outputs = inputs: inputs.flake-parts.lib.mkFlake { inherit inputs; } {
    systems = [ "x86_64-linux" ];
    imports = [ ./test.nix ];
    perSystem = { lib, self', pkgs, system, inputs', ... }: {
      packages.quota-exporter = pkgs.buildGoModule {
        name = "quota-exporter";
        src = lib.sources.sourceFilesBySuffices ./. [ ".go" ".mod" ".sum" ];
        vendorHash = "sha256-CbIdxlBAyiTqkTdX2bjP5kEgi8uk59aHTU/sJ6jA9M4=";
        meta.mainProgram = "quota-exporter";
      };

      packages.default = self'.packages.quota-exporter;
    };
  };
}
