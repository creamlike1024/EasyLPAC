package euiccsvc

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// The APDU log writes an exchange as ISO 7816 and lpac do, in upper-case
// hex, split where it is read: the command's name, its header (CLA INS
// P1 P2), its data without Lc, Le, the response's data and its status
// word apart.
//
//	apdu cmd=STORE_DATA es10=GetProfilesInfo hdr=81E29100 tx=BF2D00 rx=BF2D… sw=9000

// insNames are the instructions libeuicc and the transports send.
var insNames = map[byte]string{
	0xA4: "SELECT",
	0x70: "MANAGE_CHANNEL",
	0xE2: "STORE_DATA",
	0xC0: "GET_RESPONSE",
	0xCA: "GET_DATA",
	0xAA: "TERMINAL_CAPABILITY",
}

// es10Names are the ES10 functions (SGP.22) by the tag of the request a
// STORE DATA sequence starts with.
var es10Names = map[uint16]string{
	0xBF20: "GetEuiccInfo1",
	0xBF22: "GetEuiccInfo2",
	0xBF2E: "GetEuiccChallenge",
	0xBF3E: "GetEuiccData",
	0xBF3C: "GetEuiccConfiguredAddresses",
	0xBF3F: "SetDefaultDpAddress",
	0xBF38: "AuthenticateServer",
	0xBF21: "PrepareDownload",
	0xBF36: "LoadBoundProfilePackage",
	0xBF41: "CancelSession",
	0xBF28: "ListNotification",
	0xBF2B: "RetrieveNotificationsList",
	0xBF30: "RemoveNotificationFromList",
	0xBF2D: "GetProfilesInfo",
	0xBF31: "EnableProfile",
	0xBF32: "DisableProfile",
	0xBF33: "DeleteProfile",
	0xBF29: "SetNickname",
	0xBF34: "EuiccMemoryReset",
	0xBF43: "GetRAT",
}

// apduMaxData caps each logged data field; a short APDU's data is at
// most 255 bytes, a response's 256.
const apduMaxData = 512

// apduAttrs returns the log attributes of one exchange, each hex field
// passed through scrub.
func apduAttrs(cmd, resp []byte, err error, scrub func(string) string) []any {
	var attrs []any
	field := func(key string, b []byte) {
		attrs = append(attrs, key, scrub(hexCap(b, apduMaxData)))
	}
	if len(cmd) < 4 {
		field("tx", cmd)
	} else {
		ins := cmd[1]
		if name, ok := insNames[ins]; ok {
			attrs = append(attrs, "cmd", name)
		}
		data, le, body := splitBody(cmd[4:])
		if ins == 0xE2 && cmd[3] == 0 && len(data) >= 2 && data[0] == 0xBF {
			if name, ok := es10Names[uint16(data[0])<<8|uint16(data[1])]; ok {
				attrs = append(attrs, "es10", name)
			}
		}
		attrs = append(attrs, "hdr", hexUpper(cmd[:4]))
		switch {
		case !body:
			field("tx", cmd[4:])
		case len(data) > 0:
			field("tx", data)
		}
		if le != nil {
			attrs = append(attrs, "le", hexUpper(le))
		}
	}
	if len(resp) >= 2 {
		if len(resp) > 2 {
			field("rx", resp[:len(resp)-2])
		}
		attrs = append(attrs, "sw", hexUpper(resp[len(resp)-2:]))
	} else if len(resp) > 0 {
		field("rx", resp)
	}
	if err != nil {
		attrs = append(attrs, "err", err)
	}
	return attrs
}

// splitBody splits what follows a short APDU's header into its data
// (Lc taken off) and Le. body is false when b is no short APDU's (an
// extended length), which is then logged whole.
func splitBody(b []byte) (data, le []byte, body bool) {
	switch {
	case len(b) == 0:
		return nil, nil, true
	case len(b) == 1:
		return nil, b, true
	}
	lc := int(b[0])
	switch {
	case lc == 0:
		return nil, nil, false
	case len(b) == 1+lc:
		return b[1:], nil, true
	case len(b) == 2+lc:
		return b[1 : 1+lc], b[1+lc:], true
	}
	return nil, nil, false
}

func hexUpper(b []byte) string { return strings.ToUpper(hex.EncodeToString(b)) }

func hexCap(b []byte, max int) string {
	if len(b) > max {
		return hexUpper(b[:max]) + fmt.Sprintf("…(+%d bytes)", len(b)-max)
	}
	return hexUpper(b)
}
