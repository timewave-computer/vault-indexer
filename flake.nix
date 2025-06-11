{
  inputs = {
    flake-parts.url = "github:hercules-ci/flake-parts";
    nixpkgs.url = "github:nixos/nixpkgs";

    gomod2nix.url = "github:nix-community/gomod2nix";
    devshell.url = "github:numtide/devshell";
  };
  outputs = inputs@{ self, flake-parts, ... }:
    flake-parts.lib.mkFlake { inherit inputs; } ({ moduleWithSystem, ... }: {
      imports = [ inputs.devshell.flakeModule ];
      systems = [ "x86_64-linux" "x86_64-darwin" "aarch64-darwin" ];
      perSystem = { pkgs, system, config, ... }: {
        _module.args.pkgs = import inputs.nixpkgs {
          inherit system;
          overlays = [ inputs.gomod2nix.overlays.default ];
        };
        packages.default = config.packages.indexer;
        packages.vault-indexer = pkgs.buildGoApplication {
          pname = "vault-indexer";
          version = "latest";
          src = ./.;
          modules = ./gomod2nix.toml;
          doCheck = false;
        };
        devshells.default = {
          commands =
            [ { package = pkgs.gomod2nix; } { package = pkgs.nixfmt; } ];
        };
      };
      flake.nixosModules.vault-indexer = moduleWithSystem ({ self' }:
        { lib, config, ... }:
        let
          cfg = config.services.vault-indexer;
        in
        {
          options = {
            services.vault-indexer = {
              enable = lib.mkEnableOption "Vault Indexer";
            };

          };
          config = lib.mkIf cfg.enable {
            systemd.services.vault-indexer = {
              description = "vault indexer";
              wantedBy = [ "multi-user.target" ];
              after = [ "network.target" ];
              environment = {
                ENV = "prod";
              };
              preStart = ''
                mkdir -p github.com/timewave/
                if [[ -d github.com/timewave/vault-indexer ]]; then
                  unlink github.com/timewave/vault-indexer
                fi
                ln -s ${self'.packages.vault-indexer.src} github.com/timewave/vault-indexer
                cp -r ${self'.packages.vault-indexer.src}/abis .
              '';
              serviceConfig = {
                ExecStart = "${lib.getExe self'.packages.vault-indexer} prod";
                StateDirectory = "vault-indexer";
                WorkingDirectory = "/var/lib/vault-indexer";
              };
            };
          };
        });
    });
}
