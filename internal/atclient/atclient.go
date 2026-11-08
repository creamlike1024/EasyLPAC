// Package atclient implements a single-command-in-flight AT protocol
// client on top of a byte stream port.
//
// Design notes (docs/refactor-agent-plan.md, AT section):
//   - one reader loop assembles lines from arbitrary read fragments;
//   - the command echo is stripped;
//   - final results are OK, ERROR, +CME ERROR and +CMS ERROR;
//   - unsolicited result codes (lines beginning with '+') are routed
//     to the URC handler and never treated as command results;
//   - after a timeout or protocol error the stream cannot be trusted:
//     the client is invalidated and every later command fails with
//     ErrDesynchronized until the port is reopened. A late response
//     must never be consumed by the next command.
package atclient

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Port is the byte stream (serial port).
type Port interface {
	Read(p []byte) (int, error)
	Write(p []byte) (int, error)
	Close() error
}

// TimeoutError is the sentinel the port returns when no data arrived
// within the read timeout (serial.ErrTimeout).
type TimeoutError interface{ Timeout() bool }

// ErrDesynchronized reports that a previous command timed out or
// violated the protocol; the port must be closed and reopened before
// further use.
var ErrDesynchronized = errors.New("atclient: stream desynchronized; reopen required")

// Defaults.
const (
	DefaultTimeout    = 10 * time.Second
	DefaultMaxLineLen = 4096
	readChunk         = 512
)

// Options configure a Client.
type Options struct {
	Timeout    time.Duration // final response timeout per command
	MaxLineLen int           // longer lines invalidate the connection
	URC        func(line string)
}

// Result is the outcome of a successful command.
type Result struct {
	// Value is the payload of the line carrying the expected prefix.
	Value string
	// URCs contains unsolicited lines seen while the command ran.
	URCs []string
}

// Client speaks AT over one port. Safe for concurrent use; commands are
// serialized.
type Client struct {
	port Port
	opts Options

	mu     sync.Mutex
	buf    []byte
	closed bool
	// invalid marks the stream unusable; reason keeps the cause.
	invalid error
}

func New(port Port, opts Options) *Client {
	if opts.Timeout <= 0 {
		opts.Timeout = DefaultTimeout
	}
	if opts.MaxLineLen <= 0 {
		opts.MaxLineLen = DefaultMaxLineLen
	}
	return &Client{port: port, opts: opts}
}

// Invalid reports whether the stream must be reopened.
func (c *Client) Invalid() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.invalid != nil
}

// Command sends cmd with the client's default timeout.
func (c *Client) Command(cmd string, expectPrefix string) (*Result, error) {
	return c.CommandWithTimeout(cmd, expectPrefix, c.opts.Timeout)
}

// CommandWithTimeout is Command with a per-call final-response timeout,
// used for data commands (APDU execution) that may legitimately take
// longer than control commands.
func (c *Client) CommandWithTimeout(cmd string, expectPrefix string, timeout time.Duration) (*Result, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil, errors.New("atclient: client closed")
	}
	if c.invalid != nil {
		return nil, fmt.Errorf("%w: %v", ErrDesynchronized, c.invalid)
	}

	if _, err := c.port.Write([]byte(cmd + "\r")); err != nil {
		return c.fail("write: %v", err)
	}

	if timeout <= 0 {
		timeout = c.opts.Timeout
	}
	deadline := time.Now().Add(timeout)
	res := &Result{}
	buf := make([]byte, readChunk)

	for {
		if time.Now().After(deadline) {
			return c.fail("timeout waiting for final response to %q", cmd)
		}
		n, err := c.port.Read(buf)
		if err != nil {
			if isTimeout(err) {
				continue
			}
			return c.fail("read: %v", err)
		}

		lines := c.appendAndSplit(buf[:n])
		if c.invalid != nil {
			return nil, c.invalid
		}
		for _, line := range lines {
			switch {
			case line == "" || line == cmd: // empty line or echo
			case line == "OK":
				return res, nil
			case line == "ERROR":
				return nil, errors.New("atclient: modem returned ERROR")
			case strings.HasPrefix(line, "+CME ERROR:"), strings.HasPrefix(line, "+CMS ERROR:"):
				return nil, fmt.Errorf("atclient: modem returned %s", line)
			case expectPrefix != "" && strings.HasPrefix(line, expectPrefix):
				res.Value = unquote(strings.TrimSpace(strings.TrimPrefix(line, expectPrefix)))
			case strings.HasPrefix(line, "+"):
				res.URCs = append(res.URCs, line)
				if c.opts.URC != nil {
					c.opts.URC(line)
				}
			default:
				// Non-standard informational line; keep as diagnostic.
				res.URCs = append(res.URCs, line)
			}
		}
	}
}

// fail marks the client invalid and returns the formatted error.
func (c *Client) fail(format string, args ...any) (*Result, error) {
	c.invalid = fmt.Errorf(format, args...)
	return nil, c.invalid
}

// appendAndSplit appends fresh bytes and returns complete lines. Line
// terminators are \r\n, \n or \r. An overlong line invalidates the
// client (the buffer cannot grow unbounded, and a desynchronized
// stream must not be parsed).
func (c *Client) appendAndSplit(fresh []byte) []string {
	c.buf = append(c.buf, fresh...)
	var lines []string
	for {
		idx := indexAny(c.buf, "\r\n")
		if idx < 0 {
			break
		}
		line := strings.TrimRight(string(c.buf[:idx]), "\r")
		// Skip the paired \n of a \r\n pair.
		rest := c.buf[idx+1:]
		if c.buf[idx] == '\r' && len(rest) > 0 && rest[0] == '\n' {
			rest = rest[1:]
		}
		c.buf = rest
		if len(line) > c.opts.MaxLineLen {
			c.invalid = fmt.Errorf("atclient: line exceeds %d bytes", c.opts.MaxLineLen)
			c.buf = nil
			return lines
		}
		lines = append(lines, line)
	}
	if len(c.buf) > c.opts.MaxLineLen {
		c.invalid = fmt.Errorf("atclient: line exceeds %d bytes", c.opts.MaxLineLen)
		c.buf = nil
	}
	return lines
}

func indexAny(b []byte, chars string) int {
	for i, c := range b {
		if strings.IndexByte(chars, c) >= 0 {
			return i
		}
	}
	return -1
}

func unquote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

func isTimeout(err error) bool {
	var t TimeoutError
	return errors.As(err, &t) && t.Timeout()
}

// Close closes the port. Idempotent.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	return c.port.Close()
}
