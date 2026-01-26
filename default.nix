with import <nixpkgs> {};

let
  go = go_1_24;
in
stdenv.mkDerivation {
  pname = "spark-personal-assistant-env";
  version = "0.1.0";

  buildInputs = [ go git make direnv golangci-lint ];

  nativeBuildInputs = [ pkgs.go-modules ];

  # No build - this is a development shell
  phases = [ "unpackPhase" "installPhase" ];

  installPhase = ''
    mkdir -p $out
  '';

  meta = {
    description = "Development environment for spark-personal-assistant";
    license = licenses.mit;
    maintainers = with maintainers; [ ];
  };
}
