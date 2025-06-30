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
          meta.mainProgram = "vault-indexer";
        };
        devshells.default = {
          commands = [
            { package = pkgs.gomod2nix; }
            { package = pkgs.nixfmt-classic; }
            { package = pkgs.supabase-cli; }
          ];
        };
      };
      flake.nixosModules.vault-indexer = moduleWithSystem ({ self' }:
        { lib, config, pkgs, ... }:
        let
          inherit (lib) types;
          cfg = config.services.vault-indexer;
          package = self'.packages.vault-indexer;
        in {
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
              environment = { ENV = "prod"; };
              path = with pkgs; [ supabase-cli ];
              preStart = ''
                mkdir -p github.com/timewave/
                if [[ -d github.com/timewave/vault-indexer ]]; then
                  rm -rf github.com/timewave/vault-indexer
                fi
                ln -s ${package.src} github.com/timewave/vault-indexer

                if [[ -d abis ]]; then
                  rm -rf abis
                fi
                ln -s ${package.src}/abis .

                # Database Migration
                source .env.prod
                cd ${package.src}
                supabase db push --db-url "$INDEXER_POSTGRES_CONNECTION_STRING"
                cd -
              '';
              unitConfig = {
                StartLimitBurst = 5;
                StartLimitIntervalSec = "1m";
              };
              serviceConfig = {
                ExecStart = "${lib.getExe package} prod";
                Restart = "on-failure";
                RestartSec = "1s";
                StateDirectory = "vault-indexer";
                WorkingDirectory = "/var/lib/vault-indexer";
              };
            };
          };
        });
    });
}
