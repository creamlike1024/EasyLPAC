# EasyLPAC
**Language:** [英文](./README.md) [日文](./README_ja-JP.md)

[lpac](https://github.com/estkme-group/lpac) 圖形化前端

下載: [GitHub Release](https://github.com/creamlike1024/EasyLPAC/releases/latest)

Arch Linux: ![AUR package](https://img.shields.io/aur/version/easylpac) [AUR - easylpac](https://aur.archlinux.org/packages/easylpac)
 感謝 [@1ridic](https://github.com/1ridic)

NixOS: [NUR](https://github.com/nix-community/NUR#readme) 軟體套件 https://github.com/nix-community/nur-combined/blob/master/repos/linyinfeng/pkgs/easylpac/default.nix

openSUSE: https://software.opensuse.org/package/easylpac ([OBS](https://build.opensuse.org/package/show/home:Psheng/EasyLPAC))

系統需求:
- Windows 10以上 (最後一個支援 Windows 7 的版本是 [0.7.7.2](https://github.com/creamlike1024/EasyLPAC/releases/tag/0.7.7.2))
- 最新版的 macOS
- Linux: `pcscd`, `pcsclite`, `libcurl`(適用於 lpac) 和 `gtk3dialog` (適用於 EasyLPAC). 我不確定是否存在相依性。

目前僅支援 pcsc 的 APDUINTERFACE 和 curl 的 HTTPINTERFACE。

# 用法

執行前先連接您的讀卡機。

**[estk.me 使用者](https://www.estk.me/)**: 如果您使用的是 estk 卡隨附的 ACR38U 讀卡機，而系統是 **macOS 14 Sonoma**, 請先安裝 [讀卡機驅動程式](https://www.acs.com.hk/en/driver/228/acr38u-nd-pocketmate-smart-card-reader-micro-usb/)

## Linux

lpac 可執行檔搜尋順序: 首先，在 EasyLPAC 所在的目錄下搜尋。 如果找不到，則使用 `/usr/bin/lpac`

`EasyLPAC-linux-x86_64-with-lpac.tar.gz` 包含預先編譯的 lpac 可執行檔，如果無法執行，則需要透過套件管理器安裝 `lpac` 或自行 [編譯 lpac](https://github.com/estkme-group/lpac?tab=readme-ov-file#compile) 。

注意：在 Wayland 中無法從剪貼簿讀取 LPA 啟動碼和 QRcode

## 自動處理通知
EasyLPAC 預設會處理任何操作的通知，並在處理成功後自動刪除。

您可以前往「設定」分頁，取消勾選「自動處理通知」來停用此功能。

不過，手動操作通知不符合 GSMA 規範，因此不建議這麼做。

# 截圖
<p>
<a href="https://github.com/creamlike1024/EasyLPAC/blob/master/screenshots/chipinfo.png"><img src="https://github.com/creamlike1024/EasyLPAC/blob/master/screenshots/chipinfo.png?raw=true"  height="180px"/></a>
<a href="https://github.com/creamlike1024/EasyLPAC/blob/master/screenshots/notification.png"><img src="https://github.com/creamlike1024/EasyLPAC/blob/master/screenshots/notification.png?raw=true" height="180px"/></a>
<a href="https://github.com/creamlike1024/EasyLPAC/blob/master/screenshots/profile.png"><img src="https://github.com/creamlike1024/EasyLPAC/blob/master/screenshots/profile.png?raw=true" height="180px"/></a>
</p>

# 常見問題

## 使用 5ber 時出現 lpac 錯誤 euicc_init

前往「設定 -> lpac ISD-R AID」，點選5ber設定5ber的自訂AID，然後重試。

## macOS `SCardTransmit() failed: 80100016`

如果你使用 macOS Sonoma，可能會遇到這個錯誤: `SCardTransmit() failed: 80100016`

這是因為 Apple 的 USB CCID Card Reader Driver 有 bug，你可以試著安裝讀卡機廠商提供的 macOS 驅動程式，或者透過閱讀以下文章來解決：

- [Apple's own CCID driver in Sonoma](https://blog.apdu.fr/posts/2023/11/apple-own-ccid-driver-in-sonoma/)
- [macOS Sonoma bug: SCardControl() returns SCARD_E_NOT_TRANSACTED](https://blog.apdu.fr/posts/2023/09/macos-sonoma-bug-scardcontrol-returns-scard_e_not_transacted/)

## `SCardEstablishContext() failed: 8010001D`

這表示 PCSC 服務未運作。在 Linux 系統中，該服務名為 `pcscd` 。

在基於 systemd 的發行版上啟動 `pcscd` : `sudo systemctl start pcscd`

## `SCardListReaders() failed: 8010002E`

讀卡機未連接。

## Other `SCard` error codes

有關 PCSC 錯誤碼的完整解釋列表，請參閱 [pcsc-lite: 錯誤碼](https://pcsclite.apdu.fr/api/group__ErrorCodes.html)
