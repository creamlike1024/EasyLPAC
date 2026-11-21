# EasyLPAC
[![Listed on Awesome MyGo](https://awesomemygo.dev/badge.svg)](https://awesomemygo.dev/apps/easylpac)

**Language:** [日本語](./README_ja-JP.md) | [正體中文](./README_zh-TW.md)

A GUI LPA for managing eSIMs on removable eUICCs.

> [!IMPORTANT]
> Starting with 0.9.0, EasyLPAC has been rewritten around **libeuicc**, the protocol library of [lpac](https://github.com/estkme-group/lpac), linked into the app. No external `lpac` executable is used or required.
>
> The earlier versions (lpac CLI + Fyne) live on the [`legacy`](https://github.com/creamlike1024/EasyLPAC/tree/legacy) branch, which is no longer maintained.

## Download

[GitHub Release](https://github.com/creamlike1024/EasyLPAC/releases/latest)

# Usage

Connect your card reader or modem, open **Settings**, choose the device, and the card is read.

## Notifications

By default EasyLPAC sends the notification of every operation (install, enable, disable, delete) to the operator's server and removes it once it is sent.

You can turn this off with **Send notifications** in Settings. The pending notifications are then listed on the Notifications page.

Handling notifications by hand does not follow the GSMA specifications, so leaving this on is recommended.

# Screenshots
<p>
<a href="./screenshots/profile.png"><img src="./screenshots/profile.png" height="240px"/></a>
<a href="./screenshots/chipinfo.png"><img src="./screenshots/chipinfo.png" height="240px"/></a>
</p>

# Building from source

Requirements: Go 1.27.3+, [zig](https://ziglang.org/) (used as the C compiler), CMake, and on Linux the pcsc-lite development files (see [Linux](#linux)).

```sh
git clone --recursive https://github.com/creamlike1024/EasyLPAC.git
cd EasyLPAC
./scripts/build-app.sh          # output in build/<os>-<arch>/
```

lpac is a git submodule at `third_party/lpac`. If you cloned without `--recursive`, run `git submodule update --init` first.

## Versions

`mygo.json` holds no real version (it says `0.0.0-dev`): the build scripts take it from the git tags, with `scripts/version.sh`. A build of a release tag has that version, as `0.9.3`; a build of a later commit is a snapshot, as `0.9.3.r12.ga4e79e0` (12 commits after 0.9.3, at commit a4e79e0), with `.dirty` at the end when there are uncommitted changes. A snapshot sorts after the release it follows and before the next one, for dpkg, rpm and pacman alike, so it is also the `pkgver` of an Arch Linux `-git` package as it is:

```sh
pkgver() {
  cd "$srcdir/EasyLPAC"
  ./scripts/version.sh
}
```

The history and the tags are needed: a shallow clone (`--depth 1`) has no tag to count from. `EASYLPAC_VERSION=0.9.3 ./scripts/build-app.sh` sets the version by hand.

## Releases

Development happens on `master`. CI tests and builds every push and pull request, and keeps the packages as the run's artifacts. To release, tag a commit of `master` whose CI passed as `x.y.z` (`x.y.z-rc.1` for a pre-release), and push the tag:

```sh
git tag 0.9.3
git push origin 0.9.3
```

The tag's run builds the packages with that version and publishes the GitHub Release. A fix for an older release is made on `master`, cherry-picked to a `release/x.y` branch made from that release's tag (`git switch -c release/0.9 0.9.3`), and tagged there.

## Linux

**Note: The build steps on Linux have not been tested.**

EasyLPAC links the system's pcsc-lite dynamically (`libpcsclite.so.1`); it is never bundled. The app then always talks to `pcscd` through the client library of the same pcsc-lite release, whatever version the build machine had. GTK 3 is loaded at run time and needs no development package.

Build dependencies:

| Distribution | Packages |
|---|---|
| Debian / Ubuntu | `libpcsclite-dev pkg-config cmake` |
| Fedora | `pcsc-lite-devel pkgconf-pkg-config cmake` |
| Arch Linux | `pcsclite pkgconf cmake` |
| openSUSE | `pcsc-lite-devel pkg-config cmake` |

zig does not search the system's library directories, so tell the linker where pcsc-lite is. The binary targets glibc 2.31, while the system's `libpcsclite.so` may have been built against a newer glibc; `--allow-shlib-undefined` accepts that, since the user's own library is the one loaded at run time:

```sh
export CGO_LDFLAGS="-L$(pkg-config --variable=libdir libpcsclite) -Wl,--allow-shlib-undefined"
./scripts/build-app.sh
```

Run-time dependencies: GTK 3, pcsc-lite, and the `pcscd` service with a reader driver (`ccid` for most USB readers):

| Distribution | Packages |
|---|---|
| Debian / Ubuntu | `libgtk-3-0 libpcsclite1 pcscd libccid` |
| Fedora | `gtk3 pcsc-lite pcsc-lite-ccid` |
| Arch Linux | `gtk3 pcsclite ccid` |
| openSUSE | `gtk3 pcsc-lite pcsc-ccid` |

Start the service with `sudo systemctl enable --now pcscd.socket`.

The release has a `.deb`, which depends on `libpcsclite1` and `pcscd`, an `.rpm`, which depends on `libpcsclite.so.1` and `pcsc-lite`, and a `PKGBUILD`, which depends on `pcsclite` and `ccid`: put it next to the release's `.tar.gz` and its two icons (`easylpac-256x256.png`, `easylpac-512x512.png`), then run `makepkg -si`. Packagers for other distributions should map these to their own packages, and keep pcsc-lite linked dynamically: a statically linked client may speak a protocol version the system's `pcscd` refuses.

# License

- EasyLPAC: [MIT](./LICENSE)
- libeuicc (from lpac), statically linked: [LGPL-2.1-only](https://github.com/estkme-group/lpac/blob/main/euicc/LICENSE). Each release ships `EasyLPAC-source.tar.gz`, the complete source including libeuicc and the scripts that build it, so the library can be modified and the app rebuilt.
- eUICC manufacturer and CI data from the [eUICC Manual](https://euicc-manual.osmocom.org/): CC BY-NC-SA 4.0
