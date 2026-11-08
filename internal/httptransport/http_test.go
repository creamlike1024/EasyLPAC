package httptransport

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTransmit(t *testing.T) {
	var gotHeaders http.Header
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header.Clone()
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	tr := New(context.Background(), Options{})
	rcode, body, err := tr.Transmit(srv.URL, []string{
		"User-Agent: gsma-rsp-lpad",
		"X-Admin-Protocol: gsma/rsp/v2.2.2",
		"Content-Type: application/json",
	}, []byte(`{"pendingNotification":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	if rcode != 200 || string(body) != `{"ok":true}` {
		t.Fatalf("rcode=%d body=%q", rcode, body)
	}
	if gotHeaders.Get("X-Admin-Protocol") != "gsma/rsp/v2.2.2" {
		t.Fatalf("headers = %v", gotHeaders)
	}
	if gotBody != `{"pendingNotification":"x"}` {
		t.Fatalf("body = %q", gotBody)
	}
}

func TestTransmitHTTPErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"bad"}`))
	}))
	defer srv.Close()

	tr := New(context.Background(), Options{})
	rcode, body, err := tr.Transmit(srv.URL, nil, []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if rcode != 500 || string(body) != `{"error":"bad"}` {
		t.Fatalf("rcode=%d body=%q", rcode, body)
	}
}

func TestResponseSizeLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", 100)))
	}))
	defer srv.Close()

	tr := New(context.Background(), Options{MaxResponseBytes: 10})
	if _, _, err := tr.Transmit(srv.URL, nil, []byte(`{}`)); err == nil {
		t.Fatal("expected size limit error")
	}
}

func TestContextCancellation(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-release
	}))
	defer srv.Close()
	defer close(release)

	ctx, cancel := context.WithCancel(context.Background())
	tr := New(ctx, Options{})
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	if _, _, err := tr.Transmit(srv.URL, nil, []byte(`{}`)); err == nil {
		t.Fatal("expected cancellation error")
	}
}

func TestBadURL(t *testing.T) {
	tr := New(context.Background(), Options{})
	if _, _, err := tr.Transmit("://bad", nil, nil); err == nil {
		t.Fatal("expected URL error")
	}
}

// SM-DP+ servers use certificates outside the system trust store (GSMA
// CI or private self-signed); the default client must still connect.
func TestTransmitUntrustedTLS(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	tr := New(context.Background(), Options{})
	rcode, _, err := tr.Transmit(srv.URL, nil, []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if rcode != 200 {
		t.Fatalf("rcode=%d", rcode)
	}
}
