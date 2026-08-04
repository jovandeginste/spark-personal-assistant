with import <nixpkgs> {}; let
  go = go_1_26;
in
  stdenv.mkDerivation {
    pname = "spark-personal-assistant-env";
    version = "0.1.0";

    buildInputs = [go git gnumake direnv golangci-lint];

    nativeBuildInputs = [];

    # No build - this is a development shell
    phases = ["unpackPhase" "installPhase"];

    installPhase = ''
      mkdir -p $out
    '';

    meta = {
      description = "Development environment for spark-personal-assistant";
      license = lib.licenses.mit;
      maintainers = with lib.maintainers; [];
    };
  }
