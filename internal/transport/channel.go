package transport

import "fmt"

// The APDUs every card backend sends the same way.
const (
	InsManageChannel = 0x70
	InsSelect        = 0xA4
)

// TerminalCapabilities is the TERMINAL CAPABILITY announcement sent after
// connecting (see PCSCConfig.DisableTerminalCapability); its response is
// ignored by design.
var TerminalCapabilities = []byte{0x80, 0xAA, 0x00, 0x00, 0x0A, 0xA9, 0x08, 0x81, 0x00, 0x82, 0x01, 0x01, 0x83, 0x01, 0x07}

// TransmitFunc exchanges one raw APDU.
type TransmitFunc func(apdu []byte) ([]byte, error)

// OpenChannel performs MANAGE CHANNEL (open) followed by SELECT by AID on
// the new channel, through tx; name prefixes the errors ("pcsc").
//
// Channel logic follows ISO/IEC 7816-4 and ETSI TS 102 221. libeuicc
// addresses channels with the low nibble of CLA, so only standard
// channels 1-3 are accepted; an extended channel (4-19) is closed again
// and reported as unsupported instead of being silently truncated.
func OpenChannel(tx TransmitFunc, aid []byte, name string) (uint8, error) {
	if len(aid) == 0 || len(aid) > 32 {
		return 0, fmt.Errorf("%s: invalid AID length %d", name, len(aid))
	}
	resp, err := tx([]byte{0x00, InsManageChannel, 0x00, 0x00, 0x01})
	if err != nil {
		return 0, err
	}
	if len(resp) != 3 || SW(resp) != 0x9000 {
		return 0, fmt.Errorf("%s: MANAGE CHANNEL open failed: %x", name, resp)
	}
	channel := resp[0]
	if channel < 1 || channel > 3 {
		_, _ = tx(closeChannelAPDU(channel))
		return 0, fmt.Errorf("%s: card opened unsupported logical channel %d (only 1-3 supported)", name, channel)
	}

	sel := make([]byte, 0, 5+len(aid)+1)
	sel = append(sel, channel, InsSelect, 0x04, 0x00, byte(len(aid)))
	sel = append(sel, aid...)
	sel = append(sel, 0x00) // Le: request full FCI
	resp, err = tx(sel)
	if err != nil {
		_, _ = tx(closeChannelAPDU(channel))
		return 0, err
	}
	// T=0 cards answer 61xx (response bytes available) instead of
	// returning the FCI inline. The FCI is not needed, so 61xx counts
	// as success without a GET RESPONSE.
	if sw := SW(resp); sw != 0x9000 && sw>>8 != 0x61 {
		_, _ = tx(closeChannelAPDU(channel))
		return 0, fmt.Errorf("%s: SELECT AID failed: SW=%04X", name, sw)
	}
	return channel, nil
}

// CloseChannel performs MANAGE CHANNEL (close) through tx.
func CloseChannel(tx TransmitFunc, channel uint8, name string) error {
	resp, err := tx(closeChannelAPDU(channel))
	if err != nil {
		return err
	}
	if sw := SW(resp); sw != 0x9000 {
		return fmt.Errorf("%s: MANAGE CHANNEL close failed: SW=%04X", name, sw)
	}
	return nil
}

func closeChannelAPDU(channel uint8) []byte {
	return []byte{0x00, InsManageChannel, 0x80, channel, 0x00}
}
