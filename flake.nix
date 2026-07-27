{
  description = "mkvdup - MKV deduplication tool using FUSE";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        # Version is updated by the release workflow before tagging
        version = "1.7.1-canary.0";
        mkvdup-canary = pkgs.buildGoModule {
          pname = "mkvdup-canary";
          inherit version;
          src = ./.;
          vendorHash = "sha256-aCPVeVKyXtVf/NICq3PUsfU1yw6HuQfHGtAFywY6c1U=";
          subPackages = [ "cmd/mkvdup" ];
          ldflags = [
            "-s"
            "-w"
            "-X main.version=${version}"
          ];
          nativeBuildInputs = [ pkgs.installShellFiles ];
          # docs/mkvdup.1 is a template expanded by the release tooling; do the
          # same here so the canary man page isn't full of @PACKAGE_NAME@.
          postPatch = ''
            substituteInPlace docs/mkvdup.1 \
              --replace-fail '@PACKAGE_NAME_UPPER@' 'MKVDUP-CANARY' \
              --replace-fail '@PACKAGE_NAME@' 'mkvdup-canary'
            mv docs/mkvdup.1 docs/mkvdup-canary.1
          '';
          postInstall = ''
            mv $out/bin/mkvdup $out/bin/mkvdup-canary
            installManPage docs/mkvdup-canary.1
            installShellCompletion --cmd mkvdup-canary \
              --bash scripts/mkvdup-completion.bash \
              --zsh scripts/mkvdup-completion.zsh \
              --fish scripts/mkvdup.fish
            install -Dm755 scripts/mount.fuse.mkvdup $out/bin/mount.fuse.mkvdup-canary
          '';
          meta = {
            description = "MKV deduplication tool using FUSE (canary build)";
            homepage = "https://github.com/stuckj/mkvdup";
            license = pkgs.lib.licenses.mit;
            mainProgram = "mkvdup-canary";
          };
        };
      in
      {
        packages = {
          default = mkvdup-canary;
          mkvdup-canary = mkvdup-canary;
        };
      }
    );
}
