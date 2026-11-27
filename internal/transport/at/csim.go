package at

// CSIM mode, as lpac's at_csim driver (third_party/lpac/driver/apdu/
// at_etsi_csim.c): every APDU goes to the card as is with
// AT+CSIM=<len>,"<hex>" (TS 27.007 §8.17), and the channels are opened
// and closed with MANAGE CHANNEL and SELECT of our own. For modems that
// pass APDUs through but reject AT+CCHO (#69).
//
// The card, not the modem, chooses the channel number here, and the
// APDUs carry it in their CLA, so the handle is unused.

import (
	"fmt"
	"strconv"

	"EasyLPAC/internal/atclient"
)

type csimMode struct{ s *session }

func (m *csimMode) bind(s *session) { m.s = s }

func (m *csimMode) probes() []string { return []string{"AT+CSIM"} }

func (m *csimMode) open(aid []byte, _ uint8) (uint8, string, error) {
	resp, err := m.exchange([]byte{0x00, 0x70, 0x00, 0x00, 0x01}) // MANAGE CHANNEL open
	if err != nil {
		return 0, "", err
	}
	if len(resp) != 3 || resp[1] != 0x90 || resp[2] != 0x00 {
		return 0, "", fmt.Errorf("at: MANAGE CHANNEL open answered %X", resp)
	}
	channel := resp[0]
	if _, used := m.s.byChannel[channel]; used || channel == 0 || channel > maxChannels {
		// libeuicc can't address it in the CLA (or it is ours already):
		// give it back.
		if channel != 0 {
			_ = m.close(channel, "")
		}
		return 0, "", fmt.Errorf("at: card opened logical channel %d, need 1-%d", channel, maxChannels)
	}

	sel := append([]byte{channel, 0xA4, 0x04, 0x00, byte(len(aid))}, aid...)
	resp, err = m.exchange(sel)
	if err != nil {
		_ = m.close(channel, "")
		return 0, "", err
	}
	// 61xx: the FCI waits for a GET RESPONSE nobody needs; the
	// application is selected all the same.
	if sw1 := resp[len(resp)-2]; sw1 != 0x90 && sw1 != 0x61 {
		_ = m.close(channel, "")
		return 0, "", fmt.Errorf("at: SELECT %X on channel %d answered %X", aid, channel, resp[len(resp)-2:])
	}
	return channel, "", nil
}

func (m *csimMode) transmit(_ string, apdu []byte) ([]byte, error) {
	return m.exchange(apdu)
}

func (m *csimMode) close(channel uint8, _ string) error {
	resp, err := m.exchange([]byte{0x00, 0x70, 0x80, channel, 0x00}) // MANAGE CHANNEL close
	if err != nil {
		return err
	}
	if sw := resp[len(resp)-2:]; sw[0] != 0x90 || sw[1] != 0x00 {
		return fmt.Errorf("at: MANAGE CHANNEL close of channel %d answered %X", channel, sw)
	}
	return nil
}

// exchange sends one APDU with AT+CSIM and returns the card's response,
// SW1/SW2 included (ParseCSIM guarantees both).
func (m *csimMode) exchange(apdu []byte) ([]byte, error) {
	hexAPDU := atclient.EncodeHex(apdu)
	cmd := "AT+CSIM=" + strconv.Itoa(len(hexAPDU)) + ",\"" + hexAPDU + "\""
	res, err := m.s.client.CommandWithTimeout(cmd, "+CSIM: ", m.s.dataTimeout)
	if err != nil {
		return nil, err
	}
	parsed, err := atclient.ParseCSIM(res.Value)
	if err != nil {
		return nil, err
	}
	return parsed.Response, nil
}
