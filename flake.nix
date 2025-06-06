{
  inputs = {
    flake-parts.url = "github:hercules-ci/flake-parts";
    nixpkgs.url = "github:nixos/nixpkgs";

    gomod2nix.url = "github:nix-community/gomod2nix";
    devshell.url = "github:numtide/devshell";
  };
  outputs = inputs @ {
    self,
    flake-parts,
    ...
  }: flake-parts.lib.mkFlake { inherit inputs; } {
    imports = [
      inputs.devshell.flakeModule
    ];
    systems = ["x86_64-linux" "x86_64-darwin" "aarch64-darwin"];
    perSystem = {
      pkgs,
      system,
      config,
      ...
    }: {
      _module.args.pkgs = import inputs.nixpkgs {
        inherit system;
        overlays = [
          inputs.gomod2nix.overlays.default
        ];
      };
      packages.default = config.packages.indexer;
      packages.indexer = pkgs.buildGoApplication {
        pname = "vault-indexer";
        version = "latest";
        src = ./.;
        modules = ./gomod2nix.toml;
        doCheck = false;
      };
      devshells.default = {
        commands = [
          {package = pkgs.gomod2nix;}
        ];
      };
    };
  };
}
