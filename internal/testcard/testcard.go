// Package testcard provides a scripted eUICC APDU responder shared by
// tests in several packages (euicc service, AT transport, PC/SC where
// useful). It is referenced only from _test.go files.
package testcard

import "errors"

// Fixed identifiers used across tests.
var (
	AID    = []byte{0xA0, 0x00, 0x00, 0x05, 0x59, 0x10, 0x10, 0xFF, 0xFF, 0xFF, 0xFF, 0x89, 0x00, 0x00, 0x01, 0x00}
	EID    = []byte{0x89, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F}
	ICCID  = "89012345678901234567"
	ICCID2 = "89098765432109876543"
)

// Card answers ES10 APDUs with fixed DER payloads.
type Card struct {
	// FailEID makes the EID command return 6A88.
	FailEID bool
	// Download enables the ES10b install-flow responders (challenge,
	// info1, authenticateServer, prepareDownload, load BPP).
	Download bool
	// InstallNotificationSeq is the sequence number reported by the
	// simulated ProfileInstallationResult.
	InstallNotificationSeq int
	// FailBPP makes the install respond with an ErrorResult.
	FailBPP bool
	// WriteResult is the ES10 result code for write operations
	// (0 = success, nonzero = card-reported failure).
	WriteResult uint8
	// DefaultDp overrides the configured default SM-DP+ address
	// (defaults to smdp.example.com).
	DefaultDp string
	// Requests records every APDU received (body after the 5-byte
	// header), for assertions.
	Requests [][]byte
}

// Respond implements the wire-level response for one APDU, including
// the trailing SW1/SW2.
func (c *Card) Respond(apdu []byte) ([]byte, error) {
	if len(apdu) < 5 {
		return nil, errors.New("testcard: short APDU")
	}
	body := apdu[5:]
	c.Requests = append(c.Requests, append([]byte(nil), body...))

	switch {
	case HasTag(body, 0xBF3E): // GetEuiccData (EID)
		if c.FailEID {
			return []byte{0x6A, 0x88}, nil
		}
		return respond(DER(0xBF3E, DER(0x5A, EID)))
	case HasTag(body, 0xBF3C): // GetEuiccConfiguredAddresses
		dp := c.DefaultDp
		if dp == "" {
			dp = "smdp.example.com"
		}
		return respond(DER(0xBF3C, Cat(
			DER(0x80, []byte(dp)),
			DER(0x81, []byte("smds.example.com")),
		)))
	case HasTag(body, 0xBF22): // GetEuiccInfo2
		extCardResource := DER(0x84, Cat(
			DER(0x81, []byte{0x05}),
			DER(0x82, []byte{0x00, 0x10, 0x00}),
			DER(0x83, []byte{0x00, 0x08, 0x00}),
		))
		return respond(DER(0xBF22, Cat(
			DER(0x81, []byte{2, 2, 2}),
			DER(0x82, []byte{2, 2, 0}),
			DER(0x83, []byte{1, 0, 1}),
			extCardResource,
			DER(0x86, []byte{2, 2, 1}),
			DER(0x87, []byte{2, 3, 0}),
			DER(0xAB, []byte{1}),
			DER(0x0C, []byte("SAS-1234")),
			DER(0xAC, Cat(
				DER(0x80, []byte("label-x")),
				DER(0x81, []byte("https://ds.example")),
			)),
		)))
	case HasTag(body, 0xBF2D): // GetProfilesInfo
		profile1 := DER(0xE3, Cat(
			DER(0x5A, GSMBCD(ICCID)),
			DER(0x4F, AID),
			DER(0x9F70, []byte{0x01}),
			DER(0x90, []byte("MyProfile")),
			DER(0x91, []byte("Carrier Inc")),
			DER(0x92, []byte("Carrier Profile")),
			DER(0x93, []byte{0x01}),
			DER(0x94, []byte{0x89, 0x50, 0x4E, 0x47}),
			DER(0x95, []byte{0x02}),
		))
		profile2 := DER(0xE3, Cat(
			DER(0x5A, GSMBCD(ICCID2)),
			DER(0x4F, AID),
			DER(0x9F70, []byte{0x00}),
			DER(0x95, []byte{0x01}),
		))
		return respond(DER(0xBF2D, DER(0xA0, Cat(profile1, profile2))))
	case c.Download && HasTag(body, 0xBF2E): // GetEuiccChallenge
		return respond(DER(0xBF2E, DER(0x80, []byte("challenge-bytes"))))
	case c.Download && HasTag(body, 0xBF20): // GetEuiccInfo1
		return respond(DER(0xBF20, DER(0x04, []byte{0x01, 0x02})))
	case c.Download && HasTag(body, 0xBF38): // AuthenticateServer
		return respond(DER(0xBF38, DER(0x80, []byte{0x09, 0x09})))
	case c.Download && HasTag(body, 0xBF21): // PrepareDownload
		return respond(DER(0xBF21, DER(0x80, []byte{0x01})))
	case c.Download && isBPPSegment(body): // LoadBoundProfilePackage segments
		return respond(c.installationResult())
	case HasTag(body, 0xBF31) || HasTag(body, 0xBF32) || HasTag(body, 0xBF33): // enable/disable/delete
		tag := uint16(body[0])<<8 | uint16(body[1]) // echo the request tag
		return respond(DER(tag, DER(0x80, []byte{c.WriteResult})))
	case HasTag(body, 0xBF29): // SetNickname
		return respond(DER(0xBF29, DER(0x80, []byte{byte(c.WriteResult)})))
	case HasTag(body, 0xBF3F): // SetDefaultDpAddress
		return respond(DER(0xBF3F, DER(0x80, []byte{byte(c.WriteResult)})))
	case HasTag(body, 0xBF2B): // RetrieveNotificationsList
		notification := DER(0xBF37, DER(0xBF27, Cat(
			DER(0xBF2F, Cat(
				DER(0x80, []byte{0x00, 0x07}),
				DER(0x0C, []byte("smdp.example.com")),
				DER(0x5A, GSMBCD(ICCID)),
			)),
			DER(0xA2, DER(0xA0, nil)),
		)))
		return respond(DER(0xBF2B, DER(0xA0, notification)))
	case HasTag(body, 0xBF41): // CancelSession
		return respond(DER(0xBF41, DER(0x80, []byte{0x01})))
	case HasTag(body, 0xBF30): // NotificationSent
		return respond(DER(0xBF30, DER(0x80, []byte{0x00})))
	case HasTag(body, 0xBF28): // ListNotification
		notification := DER(0xBF2F, Cat(
			DER(0x80, []byte{0x00, 0x07}),
			DER(0x81, []byte{0x01, 0x80}),
			DER(0x0C, []byte("smdp.example.com")),
			DER(0x5A, GSMBCD(ICCID)),
		))
		return respond(DER(0xBF28, DER(0xA0, notification)))
	default:
		return nil, errors.New("testcard: unexpected request")
	}
}

// isBPPSegment reports whether the body is a LoadBoundProfilePackage
// segment as produced by libeuicc: the BF36 prefix, or one of the
// A0/A1/A2/A3 blocks and their BF2x children.
func isBPPSegment(body []byte) bool {
	for _, tag := range []uint16{0xBF36, 0xBF23, 0x0C, 0xA0, 0xA1, 0xBF24, 0xA2, 0xA3, 0xBF25} {
		if HasTag(body, tag) {
			return true
		}
	}
	return false
}

// installationResult builds the ES10b ProfileInstallationResult.
func (c *Card) installationResult() []byte {
	seq := c.InstallNotificationSeq
	if seq == 0 {
		seq = 7
	}
	metadata := DER(0xBF2F, Cat(
		DER(0x80, []byte{byte(seq)}),
		DER(0x0C, []byte("smdp.example.com")),
		DER(0x5A, GSMBCD(ICCID)),
	))
	var finalResult []byte
	if c.FailBPP {
		// ErrorResult: 80 = bppCommandId, 81 = errorReason
		finalResult = DER(0xA2, DER(0xA1, Cat(
			DER(0x80, []byte{0x05}), // loadProfileElements
			DER(0x81, []byte{0x0A}), // insufficient memory
		)))
	} else {
		finalResult = DER(0xA2, DER(0xA0, nil)) // SuccessResult
	}
	return DER(0xBF37, DER(0xBF27, Cat(metadata, finalResult)))
}

func respond(der []byte) ([]byte, error) {
	return append(der, 0x90, 0x00), nil
}

// HasTag reports whether a BER-TLV tag occurs at the start of buf.
func HasTag(buf []byte, tag uint16) bool {
	if tag > 0xFF {
		return len(buf) >= 2 && buf[0] == byte(tag>>8) && buf[1] == byte(tag)
	}
	return len(buf) >= 1 && buf[0] == byte(tag)
}

// Cat concatenates DER fragments.
func Cat(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// DER builds a BER-TLV element.
func DER(tag uint16, value []byte) []byte {
	var out []byte
	if tag > 0xFF {
		out = append(out, byte(tag>>8), byte(tag))
	} else {
		out = append(out, byte(tag))
	}
	out = append(out, DerLen(len(value))...)
	return append(out, value...)
}

// DerLen encodes a BER length.
func DerLen(n int) []byte {
	if n < 0x80 {
		return []byte{byte(n)}
	}
	if n <= 0xFF {
		return []byte{0x81, byte(n)}
	}
	return []byte{0x82, byte(n >> 8), byte(n)}
}

// GSMBCD swaps nibbles of a decimal digit string ("8901..." -> bytes).
func GSMBCD(s string) []byte {
	out := make([]byte, len(s)/2)
	for i := range out {
		hi := s[2*i] - '0'
		lo := s[2*i+1] - '0'
		out[i] = lo<<4 | hi
	}
	return out
}
