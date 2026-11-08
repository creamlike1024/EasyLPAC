package atclient

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// EncodeHex renders APDU bytes for AT+CGLA (lowercase, like the
// reference tooling).
func EncodeHex(b []byte) string {
	return hex.EncodeToString(b)
}

// DecodeHex parses a hex string; odd length is an error.
func DecodeHex(s string) ([]byte, error) {
	if len(s)%2 != 0 {
		return nil, fmt.Errorf("atclient: odd hex length %d", len(s))
	}
	return hex.DecodeString(s)
}

// CGLA is a parsed "+CGLA: <length>,<response>" line.
type CGLA struct {
	// DeclaredLength is the <length> field: the number of characters
	// of <response> per TS 27.007 §8.47, i.e. twice the APDU byte
	// count (each byte is two hex digits).
	DeclaredLength int
	// Response is the decoded response APDU (data + SW1/SW2).
	Response []byte
}

// ParseCGLA parses the payload of a "+CGLA: " response line. The
// declared length must match the hex payload length; a mismatch means
// the stream cannot be trusted.
func ParseCGLA(value string) (*CGLA, error) {
	declared, hexPart, ok := strings.Cut(value, ",")
	if !ok {
		return nil, fmt.Errorf("atclient: malformed CGLA response %q", value)
	}
	declaredLen, err := strconv.Atoi(strings.TrimSpace(declared))
	if err != nil {
		return nil, fmt.Errorf("atclient: bad CGLA length %q", declared)
	}
	hexPart = unquote(strings.TrimSpace(hexPart))
	if declaredLen != len(hexPart) {
		return nil, fmt.Errorf("atclient: CGLA declared length %d != payload length %d", declaredLen, len(hexPart))
	}
	resp, err := DecodeHex(hexPart)
	if err != nil {
		return nil, err
	}
	if len(resp) < 2 {
		return nil, errors.New("atclient: CGLA response shorter than SW1/SW2")
	}
	return &CGLA{DeclaredLength: declaredLen, Response: resp}, nil
}
