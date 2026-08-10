{
  pkgs ? import <nixpkgs> { },
  # Mirrors the flake's two outputs. Plain `nix-build` gives mkvdup, the same as
  # `#mkvdup`; `nix-build --arg canary true` gives mkvdup-canary, which installs
  # under its own command name so it can sit alongside a stable install.
  canary ? false,
}:

let
  command = if canary then "mkvdup-canary" else "mkvdup";
in
pkgs.buildGoModule rec {
  pname = command;
  # Version is updated by the release workflow before tagging
  version = "1.9.2";
  src = ./.;
  vendorHash = "sha256-rp2M/Fe5P+ganzJ6/0c75PO9Kg38LL7+vwb6pwIOgSE=";
  subPackages = [ "cmd/mkvdup" ];
  ldflags = [
    "-s"
    "-w"
    "-X main.version=${version}"
  ];
  nativeBuildInputs = [ pkgs.installShellFiles ];
  # docs/mkvdup.1 is a template expanded by the release tooling; do the same
  # here so the man page isn't full of @PACKAGE_NAME@.
  postPatch = ''
    substituteInPlace docs/mkvdup.1 \
      --replace-fail '@PACKAGE_NAME_UPPER@' '${pkgs.lib.toUpper command}' \
      --replace-fail '@PACKAGE_NAME@' '${command}'
  ''
  + pkgs.lib.optionalString canary ''
    mv docs/mkvdup.1 docs/${command}.1
  '';
  postInstall = ''
    ${pkgs.lib.optionalString canary "mv $out/bin/mkvdup $out/bin/${command}"}
    installManPage docs/${command}.1
    installShellCompletion --cmd ${command} \
      --bash scripts/mkvdup-completion.bash \
      --zsh scripts/mkvdup-completion.zsh \
      --fish scripts/mkvdup.fish
    install -Dm755 scripts/mount.fuse.mkvdup $out/bin/mount.fuse.${command}
  '';
}
