//go:build !windows

package serial

import (
	"os"
	"testing"
	"time"

	"github.com/creack/pty"
)

// TestReadTimeoutAndCloseOnPTY verifies the two behaviors that matter
// for the AT client, using a real tty (PTY pair) but no hardware:
//   - Read returns a timeout error instead of blocking forever;
//   - Close unblocks a concurrent Read.
func TestReadTimeoutAndCloseOnPTY(t *testing.T) {
	master, slave, err := pty.Open()
	if err != nil {
		t.Skipf("pty unavailable: %v", err)
	}
	defer master.Close()
	defer slave.Close()

	port, err := System{}.Open(slave.Name(), OpenOptions{Baud: 115200, ReadTimeout: 200 * time.Millisecond})
	if err != nil {
		t.Fatalf("open pty: %v", err)
	}
	defer port.Close()

	// 1) No data: expect a timeout error within a bounded time.
	start := time.Now()
	buf := make([]byte, 64)
	_, err = port.Read(buf)
	if err == nil {
		t.Fatal("expected read timeout error")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("read timeout took %v", elapsed)
	}

	// 2) Data from the peer arrives.
	if _, err := master.Write([]byte("OK\r\n")); err != nil {
		t.Fatal(err)
	}
	n, err := port.Read(buf)
	if err != nil {
		t.Fatalf("read after write: %v", err)
	}
	if string(buf[:n]) != "OK\r\n" {
		t.Fatalf("read %q", buf[:n])
	}

	// 3) Close while a Read is blocked unblocks it.
	done := make(chan error, 1)
	go func() {
		_, err := port.Read(make([]byte, 64))
		done <- err
	}()
	time.Sleep(50 * time.Millisecond)
	if err := port.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected read error after close")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Read did not unblock on Close")
	}
}

func TestEnumerateDoesNotFail(t *testing.T) {
	if _, err := (System{}).Enumerate(); err != nil {
		t.Fatalf("enumerate: %v", err)
	}
}

func TestOpenMissingPort(t *testing.T) {
	if _, err := (System{}).Open("/nonexistent/ttyZZZ0", OpenOptions{}); err == nil {
		t.Fatal("expected open error")
	}
	if _, err := (System{}).Open("", OpenOptions{}); err == nil {
		t.Fatal("expected empty-name error")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
