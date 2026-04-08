# nixpkgs package definition for mkvdup
# Submit this to NixOS/nixpkgs as: pkgs/by-name/mk/mkvdup/package.nix
{
  lib,
  buildGoModule,
  fetchFromGitHub,
  installShellFiles,
}:

buildGoModule rec {
  pname = "mkvdup";
  version = "1.8.0";

  src = fetchFromGitHub {
    owner = "stuckj";
    repo = "mkvdup";
    rev = "v${version}";
    hash = "sha256-B8fury9a4HIgAaaW+apSYXKvxS98X8I7cFytE4dJfsU=";
  };

  vendorHash = "sha256-5eT01KiQREYHZlMb+adavO2G2MbGAKOh8MdwV/dnOzg=";

  subPackages = [ "cmd/mkvdup" ];

  ldflags = [
    "-s"
    "-w"
    "-X main.version=${version}"
  ];

  nativeBuildInputs = [ installShellFiles ];

  postInstall = ''
    installManPage docs/mkvdup.1
    installShellCompletion --bash scripts/mkvdup-completion.bash
    installShellCompletion --zsh scripts/mkvdup-completion.zsh
    installShellCompletion --fish scripts/mkvdup.fish
    install -Dm755 scripts/mount.fuse.mkvdup $out/bin/mount.fuse.mkvdup
  '';

  meta = {
    description = "MKV deduplication tool using FUSE — stores MKV files as references to source media";
    homepage = "https://github.com/stuckj/mkvdup";
    license = lib.licenses.mit;
    maintainers = with lib.maintainers; [ stuckj ];
    mainProgram = "mkvdup";
  };
}
