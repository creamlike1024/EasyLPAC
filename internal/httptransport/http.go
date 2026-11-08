// Package httptransport implements the libeuicc HTTP interface with
// Go net/http. libeuicc owns the ES9+/ES11 protocol JSON; this adapter
// only performs the HTTPS exchange and returns the raw response.
package httptransport

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Defaults bound memory and time; libeuicc parses the body afterwards.
const (
	DefaultTimeout          = 60 * time.Second
	DefaultMaxResponseBytes = 4 << 20 // 4 MiB; ES9+ payloads are far smaller
)

// Options configure a Transport. Zero values select defaults.
type Options struct {
	// Client is the HTTP client to use. Nil uses a client with the
	// DefaultTimeout, the standard environment proxy settings and TLS
	// certificate verification disabled: SM-DP+ servers present
	// GSMA-CI or private self-signed certificates that no system trust
	// store holds, and the eUICC authenticates the SM-DP+ itself
	// (ES10b AuthenticateServer) regardless of the TLS layer.
	Client *http.Client
	// MaxResponseBytes caps the response body size.
	MaxResponseBytes int64
}

// Transport performs HTTPS requests for one libeuicc context.
// It is bound to a context so that a cancelled operation aborts the
// in-flight request (HTTP cancellation is not an ES9+ CancelSession;
// the caller decides whether a protocol-level cancel is appropriate).
type Transport struct {
	ctx    context.Context
	client *http.Client
	max    int64
}

// New creates a Transport bound to ctx.
func New(ctx context.Context, opts Options) *Transport {
	if ctx == nil {
		ctx = context.Background()
	}
	client := opts.Client
	if client == nil {
		client = &http.Client{Timeout: DefaultTimeout, Transport: insecureTransport()}
	}
	max := opts.MaxResponseBytes
	if max <= 0 {
		max = DefaultMaxResponseBytes
	}
	return &Transport{ctx: ctx, client: client, max: max}
}

// insecureTransport is http.DefaultTransport without certificate
// verification.
func insecureTransport() *http.Transport {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // see Options.Client
	return tr
}

// Transmit implements euicc.HTTPInterface. headers is the flat
// "Name: value" list provided by libeuicc.
func (t *Transport) Transmit(url string, headers []string, tx []byte) (uint32, []byte, error) {
	req, err := http.NewRequestWithContext(t.ctx, http.MethodPost, url, strings.NewReader(string(tx)))
	if err != nil {
		return 0, nil, fmt.Errorf("httptransport: build request: %w", err)
	}
	for _, h := range headers {
		name, value, ok := strings.Cut(h, ":")
		if !ok {
			continue // libeuicc always sends well-formed headers
		}
		req.Header.Add(strings.TrimSpace(name), strings.TrimSpace(value))
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("httptransport: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, t.max+1))
	if err != nil {
		return 0, nil, fmt.Errorf("httptransport: read response: %w", err)
	}
	if int64(len(body)) > t.max {
		return 0, nil, errors.New("httptransport: response exceeds size limit")
	}
	return uint32(resp.StatusCode), body, nil
}
