package euiccsvc

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
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
