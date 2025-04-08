# EasyLPAC
[lpac](https://github.com/estkme-group/lpac) GUI フロントエンド

ダウンロード: [GitHub リリース](https://github.com/creamlike1024/EasyLPAC/releases/latest)

Arch Linux: ![AUR パッケージ](https://img.shields.io/aur/version/easylpac) [AUR - easylpac](https://aur.archlinux.org/packages/easylpac)
 [@1ridic](https://github.com/1ridic) に感謝します。

NixOS: [NUR](https://github.com/nix-community/NUR#readme) パッケージ https://github.com/nix-community/nur-combined/blob/master/repos/linyinfeng/pkgs/easylpac/default.nix

openSUSE: https://software.opensuse.org/package/easylpac ([OBS](https://build.opensuse.org/package/show/home:Psheng/EasyLPAC))

システム要件:
- Windows7 以降
- 最新の macOS
- Linux: `pcscd`, `pcsclite`、 `libcurl`(lpac 用) と `gtk3dialog` (EasyLPAC 用)。 依存関係についてはよく理解してません。

現在、pcsc の APDUINTERFACE と curl の HTTPINTERFACE のみがサポートされています。

# 使い方

実行する前にカードリーダーを接続してください。

[eSTK.me のユーザー](https://www.estk.me/): eSTK カードに付属の ACR38U カードリーダーを使用しており、現在の OS が **macOS 14 Sonoma** を使用している場合は[カードリーダードライバー](https://www.acs.com.hk/en/driver/228/acr38u-nd-pocketmate-smart-card-reader-micro-usb/)をインストールしてください。

## Linux

lpac バイナリの検索順序: 始めに EasyLPAC を同じディレクトリを検索します。<br>
見つからない場合は、`/usr/bin/lpac` を使用します。

`EasyLPAC-linux-x86_64-with-lpac.tar.gz` には、ビルド済みの lpac バイナリが含まれています。実行できない場合は、パッケージマネージャーで `lpac` をインストールするか自分で [lpac をコンパイル](https://github.com/estkme-group/lpac?tab=readme-ov-file#compile)する必要があります。

注意: Wayland では、クリップボードからの LPA アクティベーションコードと QR コードの読み取りは機能しません。

## 通知を自動で処理
EasyLPAC は既定ですべての操作の通知を処理し、正常に処理後に通知を削除します。

この動作を無効化するには、「設定」タブから「通知を自動で処理する」のチェックを外します。

ただし、通知を意図的に操作することは GSMA 仕様に準拠していないため、手動での操作は推奨しません。

# スクリーンショット
<p>
<a href="https://github.com/creamlike1024/EasyLPAC/blob/master/screenshots/chipinfo.png"><img src="https://github.com/creamlike1024/EasyLPAC/blob/master/screenshots/chipinfo.png?raw=true"  height="180px"/></a>
<a href="https://github.com/creamlike1024/EasyLPAC/blob/master/screenshots/notification.png"><img src="https://github.com/creamlike1024/EasyLPAC/blob/master/screenshots/notification.png?raw=true" height="180px"/></a>
<a href="https://github.com/creamlike1024/EasyLPAC/blob/master/screenshots/profile.png"><img src="https://github.com/creamlike1024/EasyLPAC/blob/master/screenshots/profile.png?raw=true" height="180px"/></a>
</p>

# FAQ

## 5ber 使用時に発生する `euicc_init` の lpac エラー

設定 -> lpac ISD-R AID に移動し、5ber をクリックして 5ber のカスタム AID を設定後に再試行してください。

## macOS で `SCardTransmit() が失敗しました: 80100016` が発生する

macOS Sonoma を使用している場合にこのエラーが発生する可能性があります: `SCardTransmit() が失敗しました: 80100016`

これは Apple の USB CCID カードリーダードライバーにバグがあるためです。カードリーダーメーカーが提供する macOS ドライバーをインストールしてください。または以下の記事を参照して解決することもできます:

- [Apple's own CCID driver in Sonoma](https://blog.apdu.fr/posts/2023/11/apple-own-ccid-driver-in-sonoma/)
- [macOS Sonoma bug: SCardControl() returns SCARD_E_NOT_TRANSACTED](https://blog.apdu.fr/posts/2023/09/macos-sonoma-bug-scardcontrol-returns-scard_e_not_transacted/)

## `SCardEstablishContext() が失敗しました: 8010001D` が発生する

これは PCSC サービスが実行されていないことが原因です。Linux の場合は、`pcscd` サービスです。

systemd ベースのディストリビューションで `pcscd` を起動します: `sudo systemctl start pcscd`

## `SCardListReaders() が失敗しました: 8010002E` が発生する

カードリーダーが未接続です。

## その他の `SCard` エラーコード

PCSC エラーコードの完全な説明の一覧は [pcsc-lite: ErrorCodes](https://pcsclite.apdu.fr/api/group__ErrorCodes.html) を参照してください。
