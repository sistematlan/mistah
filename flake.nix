{
  description = "mistah — CLI open-source multiplataforma que recupera espacio en disco: cachés, papelera, backups viejos y más. Sin telemetría, código auditable.";

  # This flake is a self-hosted alternative to submitting mistah into
  # nixpkgs proper. nixpkgs is a centrally-reviewed monorepo (a PR into
  # NixOS/nixpkgs, reviewed by human maintainers) — the same category of
  # external-gatekeeping tradeoff winget and AUR both have, and mistah's
  # distribution roadmap (see BACKLOG.md) deliberately prioritized
  # self-hosted channels (Homebrew tap, Scoop bucket) over centrally-reviewed
  # ones. A flake in this repo gives the same "we control publishing
  # cadence" property Homebrew/Scoop have, without requiring anyone
  # else's review: `nix run github:sistematlan/mistah` always resolves
  # to whatever's on the `main` branch (or a specific tag/rev) directly.
  #
  # Submitting to nixpkgs itself remains a legitimate follow-up (tracked
  # in BACKLOG.md) for users who specifically want `nix-env -iA
  # nixpkgs.mistah` / `environment.systemPackages` without pinning a
  # flake input — just not implemented here.

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        packages.default = pkgs.buildGoModule {
          pname = "mistah";
          # Keep in sync with the latest git tag when cutting a new
          # release; unlike the Homebrew cask / Scoop manifest (which
          # GoReleaser regenerates automatically from the tag), this
          # flake is hand-maintained — there's no GoReleaser Nix
          # plugin in the OSS tier we use elsewhere in this project.
          version = "0.6.2";

          src = self;

          # proxyVendor fetches deps through the Go module proxy and
          # verifies them against go.sum, rather than having
          # buildGoModule run `go mod vendor` itself against the
          # source tree. The alternative (vendorHash-only, no
          # proxyVendor) hit a real "inconsistent vendoring" error
          # against this repo's go.mod/go.sum during testing — Nix's
          # internal `go mod vendor` run disagreed with what go.mod
          # declares as explicit requirements. proxyVendor sidesteps
          # that mismatch entirely; nixpkgs itself recommends it for
          # exactly this class of problem.
          proxyVendor = true;

          # vendorHash pins the exact go.sum-derived dependency tree.
          # Nix computes this deterministically; run `nix build` once
          # with vendorHash = null (or lib.fakeHash) and copy the hash
          # Nix reports in the error message here. Left as a
          # documented placeholder rather than guessed, since guessing
          # wrong silently breaks reproducibility guarantees that are
          # the entire point of using Nix.
          vendorHash = null;

          ldflags = [
            "-s"
            "-w"
            "-X main.version=0.6.2"
          ];

          meta = with pkgs.lib; {
            description = "CLI open-source multiplataforma que recupera espacio en disco: cachés, papelera, backups viejos y más";
            homepage = "https://mistah.sistematlan.com";
            license = licenses.mit;
            mainProgram = "mistah";
          };
        };

        apps.default = flake-utils.lib.mkApp {
          drv = self.packages.${system}.default;
        };
      });
}
