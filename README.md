# EasyLPAC

[lpac](https://github.com/estkme-group/lpac) GUI Frontend

Download: [GitHub Release](https://github.com/creamlike1024/EasyLPAC/releases/latest)

Linux release was built on Ubuntu 22.04, you may need to build by yourself if you can't run it

System Requirement
- Windows 7+
- MacOS 12+
- Linux: ? glibc and opengl support.
- FreeBSD: unknown

For Linux you need install `libpcsclite1`, `pcscd` and `libcurl` which are dependencies of lpac

# Usage

The Windows release and macOS release have lpac binary included.

For linux, currently there is no special package for various popular distributions (Sorry), you need to place the lpac binaries in the lpac folder.

For FreeBSD, you need compile lpac by yourself and place the lpac binaries in the lpac folder.

Currently, only APDUINTERFACE for pcsc and HTTPINTERFACE for curl are supported.

# FAQ

## macOS `SCardTransmit() failed: 80100016`

If you are using macOS Sonoma, you may encounter this error: `SCardTransmit() failed: 80100016`

This is because there is a bug in Apple's USB CCID Card Reader Driver, you can solve it by reading the following article:

- [Apple's own CCID driver in Sonoma](https://blog.apdu.fr/posts/2023/11/apple-own-ccid-driver-in-sonoma/)
- [macOS Sonoma bug: SCardControl() returns SCARD_E_NOT_TRANSACTED](https://blog.apdu.fr/posts/2023/09/macos-sonoma-bug-scardcontrol-returns-scard_e_not_transacted/)

## lpac error: `APDU library init error`

If you see `SCardListReaders() failed` and `APDU library init error`, that means the card reader is not connected properly. Try connecting the card reader correctly and try again

## Will it send notification automatically?

EasyLPAC will ask you to send notification after install or delete operation.

Other notification(enable, disable) will not be automatically processed. You should process them manually in the Notification Tab

The notification will be kept in your eUICC, including those have been successfully processed. You should manually remove them or click confirm when the program asks you to remove them.
