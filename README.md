# EasyLPAC

[lpac](https://github.com/estkme-group/lpac) GUI Frontend

Download: [GitHub Release](https://github.com/creamlike1024/EasyLPAC/releases/latest)

System Requirement
- Windows 7+
- macOS 12+
- Linux: gtk3dialog? I'm not sure about dependencies.

Currently, only APDUINTERFACE for pcsc and HTTPINTERFACE for curl are supported.

# Usage

**[estk.me User](https://www.estk.me/)**: If you are using the ACR38U card reader included with estk card and are currently using **macOS 14 Sonoma**, please install the [card reader driver](https://www.acs.com.hk/en/driver/228/acr38u-nd-pocketmate-smart-card-reader-micro-usb/) first

Linux release `with-lpac` including the pre-built lpac from [lpac release](https://github.com/estkme-group/lpac/releases/latest), if it can't run on your system, you should [compile lpac](https://github.com/estkme-group/lpac?tab=readme-ov-file#compile) by yourself. The lpac binary file should be placed in the same directory as the EasyLPAC binary file

# Screenshots
<p>
<a href="https://github.com/creamlike1024/EasyLPAC/blob/master/screenshots/chipinfo.png"><img src="https://github.com/creamlike1024/EasyLPAC/blob/master/screenshots/chipinfo.png?raw=true"  height="180px"/></a>
<a href="https://github.com/creamlike1024/EasyLPAC/blob/master/screenshots/notification.png"><img src="https://github.com/creamlike1024/EasyLPAC/blob/master/screenshots/notification.png?raw=true" height="180px"/></a>
<a href="https://github.com/creamlike1024/EasyLPAC/blob/master/screenshots/profile.png"><img src="https://github.com/creamlike1024/EasyLPAC/blob/master/screenshots/profile.png?raw=true" height="180px"/></a>
</p>

# FAQ

## macOS `SCardTransmit() failed: 80100016`

If you are using macOS Sonoma, you may encounter this error: `SCardTransmit() failed: 80100016`

This is because there is a bug in Apple's USB CCID Card Reader Driver, you can try installing the macOS driver provided by your card reader manufacturer, Or you can solve it by reading the following article:

- [Apple's own CCID driver in Sonoma](https://blog.apdu.fr/posts/2023/11/apple-own-ccid-driver-in-sonoma/)
- [macOS Sonoma bug: SCardControl() returns SCARD_E_NOT_TRANSACTED](https://blog.apdu.fr/posts/2023/09/macos-sonoma-bug-scardcontrol-returns-scard_e_not_transacted/)

## lpac error: `APDU library init error`

If you see `SCardListReaders() failed` and `APDU library init error`, that means the card reader is not connected properly. Try connecting the card reader correctly and try again

## Will it send notification automatically?

EasyLPAC will ask you to send notification after install or delete operation.

Other notification(enable, disable) will not be automatically processed. You should process them manually in the Notification Tab

The notification will be kept in your eUICC, including those have been successfully processed. You should manually remove them or click confirm when the program asks you to remove them.
