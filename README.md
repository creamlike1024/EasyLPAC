# EasyLPAC
**Language:** [日本語](./README_ja-JP.md)

[lpac](https://github.com/estkme-group/lpac) GUI Frontend

Download: [GitHub Release](https://github.com/creamlike1024/EasyLPAC/releases/latest)

Arch Linux: ![AUR package](https://img.shields.io/aur/version/easylpac) [AUR - easylpac](https://aur.archlinux.org/packages/easylpac)
 thanks to [@1ridic](https://github.com/1ridic)

NixOS: [NUR](https://github.com/nix-community/NUR#readme) package https://github.com/nix-community/nur-combined/blob/master/repos/linyinfeng/pkgs/easylpac/default.nix

openSUSE: https://software.opensuse.org/package/easylpac ([OBS](https://build.opensuse.org/package/show/home:Psheng/EasyLPAC))

System requirements:
- Windows7+
- latest macOS
- Linux: `pcscd`, `pcsclite`, `libcurl`(for lpac) and `gtk3dialog` (for EasyLPAC). I'm not sure about dependencies.

Currently, only APDUINTERFACE for pcsc and HTTPINTERFACE for curl are supported.

# Usage

Connect your card reader before running.

**[estk.me User](https://www.estk.me/)**: If you are using the ACR38U card reader included with estk card and are currently using **macOS 14 Sonoma**, please install the [card reader driver](https://www.acs.com.hk/en/driver/228/acr38u-nd-pocketmate-smart-card-reader-micro-usb/) first

## Linux

lpac binary search order: First, search in the same directory as EasyLPAC. If not found, use `/usr/bin/lpac`

`EasyLPAC-linux-x86_64-with-lpac.tar.gz` contain prebuilt lpac binary, if you can't run it, you need to install `lpac` by package manager or [compile lpac](https://github.com/estkme-group/lpac?tab=readme-ov-file#compile) by yourself.

Note: Reading LPA activation code and QRCode from clipboard not working in Wayland

## Auto process notification
EasyLPAC will process notification for any operation and remove it after successfully processing by default.

You can go to Settings Tab and uncheck "Auto process notification" to disable this behavior.

However, arbitrary manipulation of notifications does not comply with GSMA specifications, so manual operation is not recommended.

# Screenshots
<p>
<a href="https://github.com/creamlike1024/EasyLPAC/blob/master/screenshots/chipinfo.png"><img src="https://github.com/creamlike1024/EasyLPAC/blob/master/screenshots/chipinfo.png?raw=true"  height="180px"/></a>
<a href="https://github.com/creamlike1024/EasyLPAC/blob/master/screenshots/notification.png"><img src="https://github.com/creamlike1024/EasyLPAC/blob/master/screenshots/notification.png?raw=true" height="180px"/></a>
<a href="https://github.com/creamlike1024/EasyLPAC/blob/master/screenshots/profile.png"><img src="https://github.com/creamlike1024/EasyLPAC/blob/master/screenshots/profile.png?raw=true" height="180px"/></a>
</p>

# FAQ

## lpac error `euicc_init` when using 5ber

Go to Settings -> lpac ISD-R AID and click 5ber to set 5ber's custom AID, then retry

## macOS `SCardTransmit() failed: 80100016`

If you are using macOS Sonoma, you may encounter this error: `SCardTransmit() failed: 80100016`

This is because there is a bug in Apple's USB CCID Card Reader Driver, you can try installing the macOS driver provided by your card reader manufacturer, Or you can solve it by reading the following article:

- [Apple's own CCID driver in Sonoma](https://blog.apdu.fr/posts/2023/11/apple-own-ccid-driver-in-sonoma/)
- [macOS Sonoma bug: SCardControl() returns SCARD_E_NOT_TRANSACTED](https://blog.apdu.fr/posts/2023/09/macos-sonoma-bug-scardcontrol-returns-scard_e_not_transacted/)

## `SCardEstablishContext() failed: 8010001D`

This indicates that PCSC service is not running. For linux, it's `pcscd` service.

Start `pcscd` on systemd based distribution: `sudo systemctl start pcscd`

## `SCardListReaders() failed: 8010002E`

Card reader is not connected.

## Other `SCard` error codes

For complete explanation list of PCSC error codes, see [pcsc-lite: ErrorCodes](https://pcsclite.apdu.fr/api/group__ErrorCodes.html)
