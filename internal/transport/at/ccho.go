package at

// CCHO mode, following 3GPP TS 27.007 §8.45-8.47 semantics:
//
//	AT+CCHO="<aid hex>"             open channel + select AID (modem side)
//	AT+CGLA=<session>,<len>,"<hex>" exchange one raw APDU
//	AT+CCHC=<session>               close channel
//
// The modem session identifier returned by +CCHO is opaque and is NOT
// the APDU logical channel number: it is the channel's handle.

import (
	"errors"
	"fmt"
	"strings"

	"EasyLPAC/internal/atclient"
)

type cchoMode struct{ s *session }

func (m *cchoMode) bind(s *session) { m.s = s }

func (m *cchoMode) probes() []string { return []string{"AT+CCHO", "AT+CCHC", "AT+CGLA"} }

func (m *cchoMode) open(aid []byte, free uint8) (uint8, string, error) {
	res, err := m.s.client.Command(fmt.Sprintf("AT+CCHO=\"%s\"", atclient.EncodeHex(aid)), "+CCHO: ")
	if err != nil {
		return 0, "", err
	}
	sessionID := strings.TrimSpace(res.Value)
	if sessionID == "" {
		return 0, "", errors.New("at: empty session identifier from +CCHO")
	}
	return free, sessionID, nil
}

func (m *cchoMode) transmit(sessionID string, apdu []byte) ([]byte, error) {
	hexAPDU := atclient.EncodeHex(apdu)
	cmd := fmt.Sprintf("AT+CGLA=%s,%d,\"%s\"", sessionID, len(hexAPDU), hexAPDU)
	res, err := m.s.client.CommandWithTimeout(cmd, "+CGLA: ", m.s.dataTimeout)
	if err != nil {
		return nil, err
	}
	parsed, err := atclient.ParseCGLA(res.Value)
	if err != nil {
		return nil, err
	}
	return parsed.Response, nil
}

func (m *cchoMode) close(_ uint8, sessionID string) error {
	_, err := m.s.client.Command("AT+CCHC="+sessionID, "")
	return err
}
