{ lib, buildGoModule }:

buildGoModule {
  pname = "rater";
  version = lib.fileContents ./cmd/rater/version.txt;

  src = lib.fileset.toSource {
    root = ./.;
    fileset = lib.fileset.unions [
      ./go.mod
      ./go.sum
      ./cmd
      ./internal
      # The store tests load the shipped examples.
      ./examples
    ];
  };

  vendorHash = "sha256-R33GsrU+B4ozhYYKp6YoO6MoUyAwrfuEzOsYCr9mjqE=";

  subPackages = [ "cmd/rater" ];

  env.CGO_ENABLED = 0;
  ldflags = [
    "-s"
    "-w"
  ];

  # Every package's tests run, not only those of subPackages.
  preCheck = ''
    unset subPackages
  '';

  # The serve tests listen on 127.0.0.1 and connect to it.
  __darwinAllowLocalNetworking = true;

  meta = {
    description = "Terminal annotation tool for research manual checks";
    homepage = "https://github.com/furedea/rater";
    license = lib.licenses.mit;
    mainProgram = "rater";
  };
}
