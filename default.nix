{
  pkgs ? import <nixpkgs> { },
}:

pkgs.buildGoModule {
  pname = "mkvdup-canary";
  # Version is updated by the release workflow before tagging
  version = "1.7.1-canary.0";
  src = ./.;
  vendorHash = "sha256-5eT01KiQREYHZlMb+adavO2G2MbGAKOh8MdwV/dnOzg=";
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
