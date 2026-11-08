// Package testsmdp provides a scripted SM-DP+ HTTP server implementing
// the ES9+ endpoints used by the download/notification flow. Test-only:
// referenced exclusively from _test.go files.
package testsmdp

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// Server is a scripted SM-DP+.
type Server struct {
	HTTP *httptest.Server

	mu sync.Mutex
	// Requests records the JSON bodies received per endpoint.
	Requests map[string][]map[string]any

	// Fault injection.
	FailEndpoint   string // endpoint path suffix that returns HTTP 500
	ErrorStatus    string // if set, error statusCodeData with this subject code
	Calls          int
	OnNotification func(payload string)
}

// Fixed well-formed placeholders (the library only unpacks these; it
// does not verify signatures).
var (
	transactionID = []byte{0x01, 0x02, 0x03, 0x04}
	serverSigned1 = der(0x30, der(0x80, transactionID))
	serverSig1    = der(0x5F37, []byte{0xAA, 0xBB})
	ciPKID        = der(0x04, []byte{0x01, 0x02})
	serverCert    = der(0x30, []byte{0x01})
	smdpSigned2   = der(0x30, cat(der(0x80, transactionID), der(0x01, []byte{0x00})))
	smdpSig2      = der(0x5F37, []byte{0xCC})
	smdpCert      = der(0x30, []byte{0x03})
)

// ProfileMetadataDER is the ES8p metadata for the simulated profile.
func ProfileMetadataDER(iccid string, spn, name string) []byte {
	return der(0xBF25, cat(
		der(0x5A, gsmbcd(iccid)),
		der(0x91, []byte(spn)),
		der(0x92, []byte(name)),
		der(0x93, []byte{0x01}),
		der(0x95, []byte{0x02}),
	))
}

// BoundProfilePackageDER is a minimal BPP container following the
// structure libeuicc 2.3.0 expects: BF36 { BF23, A0, A1{...}, A3{...} }.
func BoundProfilePackageDER() []byte {
	initialiseSecureChannel := der(0xBF23, der(0x0C, []byte{0x01}))
	configureISDP := der(0xA0, der(0x80, []byte{0x02}))
	replaceSessionKeys := der(0xA1, der(0xBF24, []byte{0x03}))
	loadProfileElements := der(0xA3, der(0xBF25, []byte{0x04}))
	return der(0xBF36, cat(initialiseSecureChannel, configureISDP, replaceSessionKeys, loadProfileElements))
}

func New(t *testing.T) *Server {
	t.Helper()
	s := &Server{Requests: map[string][]map[string]any{}}
	mux := http.NewServeMux()
	for _, path := range []string{
		"/gsma/rsp2/es9plus/initiateAuthentication",
		"/gsma/rsp2/es9plus/authenticateClient",
		"/gsma/rsp2/es9plus/getBoundProfilePackage",
		"/gsma/rsp2/es9plus/handleNotification",
		"/gsma/rsp2/es9plus/cancelSession",
	} {
		p := path
		mux.HandleFunc(p, func(w http.ResponseWriter, r *http.Request) {
			s.handle(t, p, w, r)
		})
	}
	// libeuicc hardcodes the https:// scheme, so the simulated server
	// must speak TLS; tests use Client() which trusts its certificate.
	s.HTTP = httptest.NewTLSServer(mux)
	t.Cleanup(s.HTTP.Close)
	return s
}

// Address returns the host:port the client should address (libeuicc
// prefixes https:// itself).
func (s *Server) Address() string {
	return s.HTTP.URL[len("https://"):]
}

// Client returns an HTTP client trusting the test server certificate.
func (s *Server) Client() *http.Client { return s.HTTP.Client() }

func (s *Server) handle(t *testing.T, path string, w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)

	s.mu.Lock()
	s.Calls++
	trimmed := path[len("/gsma/rsp2/es9plus/"):]
	var parsed map[string]any
	_ = json.Unmarshal(body, &parsed)
	s.Requests[trimmed] = append(s.Requests[trimmed], parsed)
	fail := s.FailEndpoint == trimmed
	errorStatus := s.ErrorStatus
	onNotification := s.OnNotification
	s.mu.Unlock()

	// The library hardcodes https:// URLs; tests address the server
	// over a plain-HTTP proxy-free URL, so the test client rewrites
	// the scheme (see the euiccsvc test transport override).
	if fail {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"header":{"functionExecutionStatus":{"status":"Failed"}}}`))
		return
	}

	header := map[string]any{
		"functionExecutionStatus": map[string]any{"status": "Executed-Success"},
	}
	if errorStatus != "" {
		header["functionExecutionStatus"] = map[string]any{
			"status": "Failed",
			"statusCodeData": map[string]any{
				"subjectCode":       "8.8.1",
				"reasonCode":        errorStatus,
				"subjectIdentifier": "test",
				"message":           "simulated error",
			},
		}
	}

	resp := map[string]any{"header": header}
	switch trimmed {
	case "initiateAuthentication":
		resp["transactionId"] = base64.StdEncoding.EncodeToString(transactionID)
		resp["serverSigned1"] = base64.StdEncoding.EncodeToString(serverSigned1)
		resp["serverSignature1"] = base64.StdEncoding.EncodeToString(serverSig1)
		resp["euiccCiPKIdToBeUsed"] = base64.StdEncoding.EncodeToString(ciPKID)
		resp["serverCertificate"] = base64.StdEncoding.EncodeToString(serverCert)
	case "authenticateClient":
		resp["profileMetadata"] = base64.StdEncoding.EncodeToString(
			ProfileMetadataDER("89012345678901234567", "Carrier Inc", "Carrier Profile"))
		resp["smdpSigned2"] = base64.StdEncoding.EncodeToString(smdpSigned2)
		resp["smdpSignature2"] = base64.StdEncoding.EncodeToString(smdpSig2)
		resp["smdpCertificate"] = base64.StdEncoding.EncodeToString(smdpCert)
	case "getBoundProfilePackage":
		resp["boundProfilePackage"] = base64.StdEncoding.EncodeToString(BoundProfilePackageDER())
	case "handleNotification":
		if onNotification != nil {
			if v, ok := parsed["pendingNotification"].(string); ok {
				onNotification(v)
			}
		}
	case "cancelSession":
		// header-only success
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func der(tag uint16, value []byte) []byte {
	var out []byte
	if tag > 0xFF {
		out = append(out, byte(tag>>8), byte(tag))
	} else {
		out = append(out, byte(tag))
	}
	n := len(value)
	switch {
	case n < 0x80:
		out = append(out, byte(n))
	case n <= 0xFF:
		out = append(out, 0x81, byte(n))
	default:
		out = append(out, 0x82, byte(n>>8), byte(n))
	}
	return append(out, value...)
}

func cat(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func gsmbcd(s string) []byte {
	out := make([]byte, len(s)/2)
	for i := range out {
		hi := s[2*i] - '0'
		lo := s[2*i+1] - '0'
		out[i] = lo<<4 | hi
	}
	return out
}
