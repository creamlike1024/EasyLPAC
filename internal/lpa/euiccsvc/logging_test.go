package euiccsvc

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"EasyLPAC/internal/logmask"
	"EasyLPAC/internal/transport"
)

type echoHTTP struct{}

func (echoHTTP) Transmit(url string, headers []string, tx []byte) (uint32, []byte, error) {
	return 200, []byte(`{"header":{"functionExecutionStatus":{"status":"Executed-Success"}}}`), nil
}

func TestLoggedHTTPRedactsMatchingID(t *testing.T) {
	var buf bytes.Buffer
	l := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	h := &loggedHTTP{inner: echoHTTP{}, log: l, bodies: true}
	code, _, err := h.Transmit("https://smdp.example/gsma/rsp2/es9plus/initiateAuthentication?x=1", nil,
		[]byte(`{"smdpAddress":"smdp.example","matchingId" : "SECRET-123","euiccChallenge":"AA=="}`))
	if err != nil || code != 200 {
		t.Fatal(code, err)
	}
	out := buf.String()
	if strings.Contains(out, "SECRET-123") {
		t.Fatalf("matching ID leaked:\n%s", out)
	}
	if !strings.Contains(out, "smdp.example/gsma/rsp2/es9plus/initiateAuthentication") || strings.Contains(out, "x=1") {
		t.Fatalf("endpoint not logged as host+path:\n%s", out)
	}
}

// attrText renders apduAttrs as key=value pairs.
func attrText(attrs []any) string {
	var parts []string
	for i := 0; i+1 < len(attrs); i += 2 {
		parts = append(parts, fmt.Sprintf("%v=%v", attrs[i], attrs[i+1]))
	}
	return strings.Join(parts, " ")
}

func TestAPDUAttrs(t *testing.T) {
	id := func(s string) string { return s }
	for _, tc := range []struct {
		name      string
		cmd, resp string
		want      string
	}{
		{"ES10 request", "81E2910003BF2D00", "BF2D03A0019000",
			"cmd=STORE_DATA es10=GetProfilesInfo hdr=81E29100 tx=BF2D00 rx=BF2D03A001 sw=9000"},
		{"SELECT with Le", "00A4040410A0000005591010FFFFFFFF890000010000", "6A82",
			"cmd=SELECT hdr=00A40404 tx=A0000005591010FFFFFFFF8900000100 le=00 sw=6A82"},
		{"GET RESPONSE", "00C0000010", "0102039000",
			"cmd=GET_RESPONSE hdr=00C00000 le=10 rx=010203 sw=9000"},
		// A later block of a sequence starts mid-request: no ES10 name.
		{"later block", "81E2110102BF2E", "9000", "cmd=STORE_DATA hdr=81E21101 tx=BF2E sw=9000"},
		{"unknown, inconsistent Lc", "80100000FF01", "6D00", "hdr=80100000 tx=FF01 sw=6D00"},
		{"short", "00", "", "tx=00"},
	} {
		cmd, _ := hex.DecodeString(tc.cmd)
		resp, _ := hex.DecodeString(tc.resp)
		if got := attrText(apduAttrs(cmd, resp, nil, id)); got != tc.want {
			t.Errorf("%s:\n got %s\nwant %s", tc.name, got, tc.want)
		}
	}
	if got := attrText(apduAttrs([]byte{0, 0xA4, 4, 0}, nil, errors.New("gone"), id)); got != "cmd=SELECT hdr=00A40400 err=gone" {
		t.Errorf("error: %s", got)
	}
}

// A download's matching ID is starred in the APDUs and HTTP bodies
// logged, in the encodings it travels in.
func TestSecretsStarred(t *testing.T) {
	var buf bytes.Buffer
	l := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	secrets := new(logmask.Set)
	secrets.Add(logmask.Secret, "SECRET-123")
	ctx := hex.EncodeToString([]byte("SECRET-123"))
	a := &sessionAPDU{session: echoSession{}, log: l, secrets: secrets}
	if _, err := a.Transmit(append([]byte{0x81, 0xE2, 0x91, 0x00, 0x0E, 0xA0, 0x0C, 0x80, 0x0A}, []byte("SECRET-123")...)); err != nil {
		t.Fatal(err)
	}
	h := &loggedHTTP{inner: echoHTTP{}, log: l, bodies: true, secrets: secrets}
	body := `{"authenticateServerResponse":"` + base64.StdEncoding.EncodeToString([]byte("..ctxParams SECRET-123 ..")) + `"}`
	if _, _, err := h.Transmit("https://smdp.example/gsma/rsp2/es9plus/authenticateClient", nil, []byte(body)); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, leak := range []string{"SECRET-123", strings.ToUpper(ctx), base64.StdEncoding.EncodeToString([]byte("SECRET-123"))[:8]} {
		if strings.Contains(out, leak) {
			t.Fatalf("%s leaked:\n%s", leak, out)
		}
	}
}

type echoSession struct{ transport.CardSession }

func (echoSession) Transmit(context.Context, []byte) ([]byte, error) { return []byte{0x90, 0x00}, nil }
