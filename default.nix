{
  pkgs ? import <nixpkgs> { },
}:

pkgs.buildGoModule {
  pname = "mkvdup-canary";
  # Version is updated by the release workflow before tagging
  version = "1.8.1";
  src = ./.;
  vendorHash = "sha256-aCPVeVKyXtVf/NICq3PUsfU1yw6HuQfHGtAFywY6c1U=";
  subPackages = [ "cmd/mkvdup" ];
  ldflags = [
    "-s"
    "-w"
    "-X main.version=canary"
  ];
  nativeBuildInputs = [ pkgs.installShellFiles ];
  postInstall = ''
    mv $out/bin/mkvdup $out/bin/mkvdup-canary
    installManPage docs/mkvdup.1
    installShellCompletion --bash scripts/mkvdup-completion.bash
    installShellCompletion --zsh scripts/mkvdup-completion.zsh
    installShellCompletion --fish scripts/mkvdup.fish
    install -Dm755 scripts/mount.fuse.mkvdup $out/bin/mount.fuse.mkvdup-canary
  '';
}
