{
  description = "A terminal annotation tool for research manual checks";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-26.05-darwin";
    nixpkgs-unstable.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  };

  outputs =
    {
      self,
      nixpkgs,
      nixpkgs-unstable,
    }:
    let
      systems = [
        "aarch64-darwin"
        "aarch64-linux"
        "x86_64-linux"
      ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
    in
    {
      packages = forAllSystems (pkgs: {
        default = pkgs.callPackage ./package.nix { buildGoModule = pkgs.buildGo127Module; };
      });

      apps = forAllSystems (pkgs: {
        default = {
          type = "app";
          program = pkgs.lib.getExe self.packages.${pkgs.stdenv.hostPlatform.system}.default;
          meta.description = "Run rater";
        };
      });

      checks = forAllSystems (pkgs: {
        package = self.packages.${pkgs.stdenv.hostPlatform.system}.default;
      });

      devShells = forAllSystems (
        pkgs:
        let
          unstable = nixpkgs-unstable.legacyPackages.${pkgs.stdenv.hostPlatform.system};
        in
        {
          default = pkgs.mkShell {
            env.GOTOOLCHAIN = "local";
            packages = with pkgs; [
              commitlint
              deadnix
              go_1_27
              gopls
              # lefthook before 2.1.6 runs hooks under a pty, which agent sandboxes deny.
              unstable.lefthook
              # golangci-lint refuses modules newer than the Go that built it.
              unstable.golangci-lint
              ls-lint
              nixfmt
              statix
            ];
          };
        }
      );
    };
}
