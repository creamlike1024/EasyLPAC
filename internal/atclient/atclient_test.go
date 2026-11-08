package atclient

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakePort replays scripted read chunks and records writes. Reads are
// served one chunk at a time; when the queue is empty, ErrTimeout is
// returned (like the normalized serial port).
type fakePort struct {
	mu       sync.Mutex
	chunks   []string
	writes   []string
	closed   bool
	blockCh  chan struct{} // if non-nil, Read waits until closed
	failWith error
}

type portTimeout struct{}

func (portTimeout) Error() string { return "serial: read timeout" }
func (portTimeout) Timeout() bool { return true }

var errTimeout = portTimeout{}

func (f *fakePort) push(s string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.chunks = append(f.chunks, s)
}

func (f *fakePort) Read(p []byte) (int, error) {
	for {
		f.mu.Lock()
		if len(f.chunks) > 0 {
			chunk := f.chunks[0]
			f.chunks = f.chunks[1:]
			f.mu.Unlock()
			n := copy(p, chunk)
			return n, nil
		}
		if f.failWith != nil {
			err := f.failWith
			f.mu.Unlock()
			return 0, err
		}
		block := f.blockCh
		closed := f.closed
		f.mu.Unlock()
		if closed {
			return 0, errors.New("port closed")
		}
		if block != nil {
			<-block
			continue
		}
		time.Sleep(2 * time.Millisecond)
		return 0, errTimeout
	}
}

func (f *fakePort) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return 0, errors.New("port closed")
	}
	f.writes = append(f.writes, string(p))
	return len(p), nil
}

func (f *fakePort) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	if f.blockCh != nil {
		close(f.blockCh)
		f.blockCh = nil
	}
	return nil
}

func (f *fakePort) written() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.writes...)
}

func newTestClient(t *testing.T, p *fakePort, u rcFunc) *Client {
	t.Helper()
	return New(p, Options{Timeout: 500 * time.Millisecond, URC: u})
}

type rcFunc = func(string)

func TestSimpleCommandWithEcho(t *testing.T) {
	p := &fakePort{}
	p.push("AT\r\r\nOK\r\n")
	c := newTestClient(t, p, nil)

	if _, err := c.Command("AT", ""); err != nil {
		t.Fatal(err)
	}
	if got := p.written(); len(got) != 1 || got[0] != "AT\r" {
		t.Fatalf("writes = %q", got)
	}
}

func TestExpectPrefixAndFragmentedReads(t *testing.T) {
	p := &fakePort{}
	// Response split across arbitrary chunk boundaries.
	p.push("AT+CCHO=\"A000\"\r\n")
	p.push("+CCHO: ")
	p.push("1\r\nOK\r\n")
	c := newTestClient(t, p, nil)

	res, err := c.Command("AT+CCHO=\"A000\"", "+CCHO: ")
	if err != nil {
		t.Fatal(err)
	}
	if res.Value != "1" {
		t.Fatalf("value = %q", res.Value)
	}
}

func TestErrorCases(t *testing.T) {
	for _, tc := range []struct{ name, resp string }{
		{"ERROR", "AT\r\r\nERROR\r\n"},
		{"CME", "AT\r\r\n+CME ERROR: 10\r\n"},
		{"CMS", "AT\r\r\n+CMS ERROR: 500\r\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &fakePort{}
			p.push(tc.resp)
			c := newTestClient(t, p, nil)
			if _, err := c.Command("AT", ""); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestURCRouting(t *testing.T) {
	var urcs []string
	p := &fakePort{}
	p.push("AT+CGLA=1,4,\"00A4\"\r\n+CREG: 2\r\n+CGLA: 4,\"9000\"\r\n+CREG: 1\r\nOK\r\n")
	c := newTestClient(t, p, func(line string) { urcs = append(urcs, line) })

	res, err := c.Command("AT+CGLA=1,4,\"00A4\"", "+CGLA: ")
	if err != nil {
		t.Fatal(err)
	}
	if res.Value != "4,\"9000\"" {
		t.Fatalf("value = %q", res.Value)
	}
	if len(urcs) != 2 || urcs[0] != "+CREG: 2" || urcs[1] != "+CREG: 1" {
		t.Fatalf("urcs = %q", urcs)
	}
	if len(res.URCs) != 2 {
		t.Fatalf("result urcs = %q", res.URCs)
	}
}

// A late response after a timeout must not be consumed by the next
// command: the client invalidates and refuses further use.
func TestTimeoutInvalidatesAndLateResponseIsNotConsumed(t *testing.T) {
	p := &fakePort{}
	c := newTestClient(t, p, nil)

	_, err := c.Command("AT", "")
	if err == nil {
		t.Fatal("expected timeout")
	}
	if !c.Invalid() {
		t.Fatal("client must be invalid after timeout")
	}

	p.push("OK\r\n") // the late response
	if _, err := c.Command("AT", ""); !errors.Is(err, ErrDesynchronized) {
		t.Fatalf("err = %v", err)
	}
	if got := p.written(); len(got) != 1 {
		t.Fatalf("second command must not be written, writes = %q", got)
	}
}

func TestLineTerminatorVariants(t *testing.T) {
	for _, term := range []string{"\r\n", "\n", "\r"} {
		t.Run(fmt.Sprintf("%q", term), func(t *testing.T) {
			p := &fakePort{}
			p.push("AT" + term + "OK" + term)
			c := newTestClient(t, p, nil)
			if _, err := c.Command("AT", ""); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestOverlongLineInvalidates(t *testing.T) {
	p := &fakePort{}
	long := strings.Repeat("x", 100)
	p.push("AT\r\n" + long + "\r\nOK\r\n")
	c := New(p, Options{Timeout: 200 * time.Millisecond, MaxLineLen: 32})
	if _, err := c.Command("AT", ""); err == nil {
		t.Fatal("expected overlong-line error")
	}
	if !c.Invalid() {
		t.Fatal("client must be invalid")
	}
}

func TestCGLAEndToEnd(t *testing.T) {
	p := &fakePort{}
	p.push("AT+CGLA=1,10,\"00A4000400\"\r\n+CGLA: 6,\"9000\"\r\nOK\r\n")
	c := newTestClient(t, p, nil)
	res, err := c.Command("AT+CGLA=1,10,\"00A4000400\"", "+CGLA: ")
	if err != nil {
		t.Fatal(err)
	}
	// Declared 6 vs payload "9000" (4 chars): must be rejected.
	if _, err := ParseCGLA(res.Value); err == nil {
		t.Fatal("expected CGLA length mismatch")
	}
}

func TestParseCGLA(t *testing.T) {
	got, err := ParseCGLA("10,\"9F1A049000\"")
	if err != nil {
		t.Fatal(err)
	}
	if got.DeclaredLength != 10 || len(got.Response) != 5 || got.Response[4] != 0x00 {
		t.Fatalf("cgla = %+v", got)
	}
	if _, err := ParseCGLA("10,9F1A"); err == nil {
		t.Fatal("expected length mismatch")
	}
	if _, err := ParseCGLA("nonsense"); err == nil {
		t.Fatal("expected malformed error")
	}
	if _, err := ParseCGLA("2,\"90\""); err == nil {
		t.Fatal("expected short-response error")
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	p := &fakePort{}
	c := newTestClient(t, p, nil)
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Command("AT", ""); err == nil {
		t.Fatal("command after close must fail")
	}
}
