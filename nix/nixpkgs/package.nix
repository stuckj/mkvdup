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

  # docs/mkvdup.1 is a template; upstream's release tooling expands these before
  # packaging. --replace-fail so a rename upstream breaks the build loudly
  # instead of silently shipping a man page full of placeholders.
  postPatch = ''
    substituteInPlace docs/mkvdup.1 \
      --replace-fail '@PACKAGE_NAME_UPPER@' 'MKVDUP' \
      --replace-fail '@PACKAGE_NAME@' 'mkvdup'
  '';

  postInstall = ''
    installManPage docs/mkvdup.1
    installShellCompletion --cmd mkvdup \
      --bash scripts/mkvdup-completion.bash \
      --zsh scripts/mkvdup-completion.zsh \
      --fish scripts/mkvdup.fish
    install -Dm755 scripts/mount.fuse.mkvdup $out/bin/mount.fuse.mkvdup
  '';

  meta = {
    description = "MKV deduplication tool using FUSE — stores MKV files as references to source media";
    homepage = "https://github.com/stuckj/mkvdup";
    license = lib.licenses.mit;
    maintainers = with lib.maintainers; [ stuckj ];
    mainProgram = "mkvdup";
    # Left unset this defaults to every platform the Go compiler supports, which
    # includes FreeBSD and WASI. mkvdup is a FUSE filesystem shipping a bash
    # mount helper, and upstream builds and tests only Linux and macOS.
    platforms = lib.platforms.linux ++ lib.platforms.darwin;
  };
}
