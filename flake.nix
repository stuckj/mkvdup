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
        lib = pkgs.lib;
        # Version is updated by the release workflow before tagging
        version = "1.8.2";

        # Both outputs build the same source tree — the flake reference decides
        # *which* tree. They differ only in the installed command name, so a
        # canary cut from a development branch can sit alongside a stable
        # install, the same way the deb/rpm and Homebrew packages do.
        #
        #   #mkvdup         point the ref at a release tag
        #   #mkvdup-canary  point the ref at a branch you want to test
        mkPackage =
          { command }:
          pkgs.buildGoModule {
            pname = command;
            inherit version;
            src = ./.;
            vendorHash = "sha256-PjYhS3wYnXOIYxpjX5YsIsSqoY+ATOjR61cWH5HwCIU=";
            subPackages = [ "cmd/mkvdup" ];
            ldflags = [
              "-s"
              "-w"
              "-X main.version=${version}"
            ];
            nativeBuildInputs = [ pkgs.installShellFiles ];
            # docs/mkvdup.1 is a template that the release tooling expands; do
            # the same here so the man page isn't full of @PACKAGE_NAME@.
            postPatch = ''
              substituteInPlace docs/mkvdup.1 \
                --replace-fail '@PACKAGE_NAME_UPPER@' '${lib.toUpper command}' \
                --replace-fail '@PACKAGE_NAME@' '${command}'
            ''
            + lib.optionalString (command != "mkvdup") ''
              mv docs/mkvdup.1 docs/${command}.1
            '';
            postInstall = ''
              ${lib.optionalString (command != "mkvdup")
                "mv $out/bin/mkvdup $out/bin/${command}"
              }
              installManPage docs/${command}.1
              installShellCompletion --cmd ${command} \
                --bash scripts/mkvdup-completion.bash \
                --zsh scripts/mkvdup-completion.zsh \
                --fish scripts/mkvdup.fish
              install -Dm755 scripts/mount.fuse.mkvdup $out/bin/mount.fuse.${command}
            '';
            meta = {
              description =
                "MKV deduplication tool using FUSE"
                + lib.optionalString (command != "mkvdup") " (canary build)";
              homepage = "https://github.com/stuckj/mkvdup";
              license = lib.licenses.mit;
              mainProgram = command;
            };
          };

        mkvdup = mkPackage { command = "mkvdup"; };
        mkvdup-canary = mkPackage { command = "mkvdup-canary"; };
      in
      {
        # default is mkvdup: that is what someone installing this project
        # expects to get. mkvdup-canary is for testing an unmerged branch, which
        # is a deliberate act, so it should have to be asked for by name.
        packages = {
          default = mkvdup;
          inherit mkvdup mkvdup-canary;
        };
      }
    );
}
