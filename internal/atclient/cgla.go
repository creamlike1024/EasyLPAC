package atclient

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// EncodeHex renders APDU bytes for AT+CGLA and AT+CSIM (lowercase, like the
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

// APDUResponse is a parsed "+CGLA: <length>,<response>" or
// "+CSIM: <length>,<response>" line; both commands answer alike
// (TS 27.007 §8.17, §8.47).
type APDUResponse struct {
	// DeclaredLength is the <length> field: the number of characters
	// of <response>, i.e. twice the APDU byte count (each byte is two
	// hex digits).
	DeclaredLength int
	// Response is the decoded response APDU (data + SW1/SW2).
	Response []byte
}

// ParseCGLA parses the payload of a "+CGLA: " response line. The
// declared length must match the hex payload length; a mismatch means
// the stream cannot be trusted.
func ParseCGLA(value string) (*APDUResponse, error) { return parseAPDUResponse("CGLA", value) }

// ParseCSIM parses the payload of a "+CSIM: " response line, as
// ParseCGLA does.
func ParseCSIM(value string) (*APDUResponse, error) { return parseAPDUResponse("CSIM", value) }

func parseAPDUResponse(cmd, value string) (*APDUResponse, error) {
	declared, hexPart, ok := strings.Cut(value, ",")
	if !ok {
		return nil, fmt.Errorf("atclient: malformed %s response %q", cmd, value)
	}
	declaredLen, err := strconv.Atoi(strings.TrimSpace(declared))
	if err != nil {
		return nil, fmt.Errorf("atclient: bad %s length %q", cmd, declared)
	}
	hexPart = unquote(strings.TrimSpace(hexPart))
	if declaredLen != len(hexPart) {
		return nil, fmt.Errorf("atclient: %s declared length %d != payload length %d", cmd, declaredLen, len(hexPart))
	}
	resp, err := DecodeHex(hexPart)
	if err != nil {
		return nil, err
	}
	if len(resp) < 2 {
		return nil, fmt.Errorf("atclient: %s response shorter than SW1/SW2", cmd)
	}
	return &APDUResponse{DeclaredLength: declaredLen, Response: resp}, nil
}
