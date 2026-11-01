{
  description = "";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-26.05-darwin";
    nixpkgs-unstable.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  };

  outputs =
    { nixpkgs, nixpkgs-unstable, ... }:
    let
      system = "aarch64-darwin";
      pkgs = import nixpkgs { inherit system; };
      unstable = nixpkgs-unstable.legacyPackages.${system};
    in
    {
      devShells.${system}.default = pkgs.mkShell {
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
    };
}
