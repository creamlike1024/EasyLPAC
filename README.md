# EasyLPAC

[lpac](https://github.com/estkme-group/lpac) GUI Frontend

Download: [GitHub Release](https://github.com/creamlike1024/EasyLPAC/releases/latest)

# Usage

The Windows release and macOS release have lpac binary included.

For linux, currently there is no special package for various popular distributions (Sorry), you need put lpac binary to the lpac folder

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

No. Any notification generated from any operation(install, delete, enable, disable) on the profile will not be automatically processed.
You should process them manually in the Notification Tab

Unless you remove notification manually, the notification will be kept in your eUICC, including those have been successfully processed.
