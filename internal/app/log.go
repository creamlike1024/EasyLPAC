package app

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"EasyLPAC/internal/logmask"
)

// LogEntry is one line of the in-memory application log.
type LogEntry struct {
	Time    time.Time
	Level   slog.Level
	Message string
	Attrs   []LogAttr
	// known are the identifiers obscured in raw data (APDUs, HTTP
	// bodies) when the entry renders obscured: those the app has read
	// by then, earlier entries included.
	known *logmask.Set
}

// LogAttr is one key=value of an entry. Secrets are already redacted;
// identifiers (EID, ICCID) are kept and obscured when rendered so.
type LogAttr struct {
	Key, Value string
	Identifier bool
}

// AttrText renders the attributes, obscuring identifiers when hide is
// set: the identifier attributes, and the known identifiers in the
// others' values.
func (e LogEntry) AttrText(hide bool) string {
	var sb strings.Builder
	for i, a := range e.Attrs {
		if i > 0 {
			sb.WriteByte(' ')
		}
		v := a.Value
		switch {
		case hide && a.Identifier:
			v = Mask(v)
		case hide:
			v = e.known.Apply(v)
		}
		if strings.ContainsAny(v, " \t\"") {
			v = fmt.Sprintf("%q", v)
		}
		sb.WriteString(a.Key + "=" + v)
	}
	return sb.String()
}

// logTimeLayout dates each exported line in full, with its UTC offset,
// as Console and other log viewers show plain-text logs.
const logTimeLayout = "2006-01-02 15:04:05.000-0700"

// Text renders the entry as one line, obscuring identifiers when hide is
// set.
func (e LogEntry) Text(hide bool) string {
	s := e.Time.Format(logTimeLayout) + " " + levelName(e.Level) + " " + e.Message
	if attrs := e.AttrText(hide); attrs != "" {
		s += " " + attrs
	}
	return s
}

func levelName(l slog.Level) string {
	switch {
	case l >= slog.LevelError:
		return "ERROR"
	case l >= slog.LevelWarn:
		return "WARN "
	case l >= slog.LevelInfo:
		return "INFO "
	default:
		return "DEBUG"
	}
}

// LogBuffer keeps the log of the current run in memory only (nothing is
// written to disk); the oldest entries are dropped beyond the limit.
// It is safe for concurrent use.
type LogBuffer struct {
	mu      sync.Mutex
	entries []LogEntry
	max     int
	dropped int
	version uint64
	started time.Time
	header  []string
	// known are the identifiers read so far (AddIdentifiers).
	known *logmask.Set
	// raw is set once raw protocol data may have been logged.
	raw bool
	// onAppend is called (without the lock held) after each entry.
	onAppend func()
}

// NewLogBuffer returns a buffer holding at most max entries.
func NewLogBuffer(max int) *LogBuffer {
	return &LogBuffer{max: max, started: time.Now(), known: new(logmask.Set)}
}

// AddIdentifiers records EIDs, ICCIDs and IMEIs the app has read or was
// given, which obscured entries hide in raw protocol data.
func (b *LogBuffer) AddIdentifiers(values ...string) {
	b.known.Add(logmask.Identifier, values...)
}

// SetHeader sets the environment lines written at the top of exports.
func (b *LogBuffer) SetHeader(lines ...string) {
	b.mu.Lock()
	b.header = append([]string(nil), lines...)
	b.mu.Unlock()
}

// SetRaw records that raw protocol data may be in the log.
func (b *LogBuffer) SetRaw() {
	b.mu.Lock()
	b.raw = true
	b.mu.Unlock()
}

// OnAppend registers a callback run after each new entry.
func (b *LogBuffer) OnAppend(fn func()) {
	b.mu.Lock()
	b.onAppend = fn
	b.mu.Unlock()
}

func (b *LogBuffer) add(e LogEntry) {
	b.mu.Lock()
	if len(b.entries) >= b.max {
		n := len(b.entries) - b.max + 1
		b.entries = append(b.entries[:0], b.entries[n:]...)
		b.dropped += n
	}
	e.known = b.known
	b.entries = append(b.entries, e)
	b.version++
	fn := b.onAppend
	b.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// Snapshot returns a copy of the entries and a version that changes
// whenever an entry is added.
func (b *LogBuffer) Snapshot() ([]LogEntry, uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]LogEntry(nil), b.entries...), b.version
}

// Export writes the header and every entry as text, obscuring
// identifiers when hide is set.
func (b *LogBuffer) Export(w io.Writer, hide bool) error {
	b.mu.Lock()
	header := append([]string(nil), b.header...)
	entries := append([]LogEntry(nil), b.entries...)
	dropped := b.dropped
	started := b.started
	b.mu.Unlock()

	var sb strings.Builder
	sb.WriteString("# EasyLPAC log, run started " + started.Format(time.RFC3339) + "\n")
	for _, h := range header {
		sb.WriteString("# " + h + "\n")
	}
	if hide {
		sb.WriteString("# EIDs and ICCIDs are obscured.\n")
	}
	switch {
	case b.raw && hide:
		sb.WriteString("# Debug logging was on: this log holds raw APDUs and HTTP bodies. The EIDs,\n")
		sb.WriteString("# ICCIDs and IMEIs read in this run are obscured in them; other data is not.\n")
	case b.raw:
		sb.WriteString("# WARNING: debug logging was on; this log holds raw APDUs and HTTP bodies,\n")
		sb.WriteString("# which include card identifiers. Do not publish it.\n")
	}
	if dropped > 0 {
		fmt.Fprintf(&sb, "# %d older entries were dropped\n", dropped)
	}
	sb.WriteString("\n")
	for _, e := range entries {
		sb.WriteString(e.Text(hide))
		sb.WriteByte('\n')
	}
	_, err := io.WriteString(w, sb.String())
	return err
}

// Handler returns a slog.Handler writing into the buffer at level and
// above.
func (b *LogBuffer) Handler(level slog.Leveler) slog.Handler {
	return &bufferHandler{buf: b, level: level}
}

type bufferHandler struct {
	buf    *LogBuffer
	level  slog.Leveler
	attrs  []slog.Attr
	groups string
}

func (h *bufferHandler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= h.level.Level()
}

func (h *bufferHandler) Handle(_ context.Context, r slog.Record) error {
	var attrs []LogAttr
	write := func(a slog.Attr) {
		a = redactSecret(a)
		if a.Equal(slog.Attr{}) {
			return
		}
		attrs = append(attrs, LogAttr{
			Key:        h.groups + a.Key,
			Value:      a.Value.Resolve().String(),
			Identifier: identifierKeys[strings.ToLower(a.Key)],
		})
	}
	for _, a := range h.attrs {
		write(a)
	}
	r.Attrs(func(a slog.Attr) bool { write(a); return true })
	h.buf.add(LogEntry{Time: r.Time, Level: r.Level, Message: r.Message, Attrs: attrs})
	return nil
}

func (h *bufferHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	n := *h
	n.attrs = append(append([]slog.Attr(nil), h.attrs...), attrs...)
	return &n
}

func (h *bufferHandler) WithGroup(name string) slog.Handler {
	n := *h
	n.groups = h.groups + name + "."
	return &n
}

// identifierKeys are obscured when identifiers are hidden; secretKeys
// are always redacted, wherever they are logged from.
var (
	identifierKeys = map[string]bool{"eid": true, "iccid": true}
	secretKeys     = map[string]bool{
		"matching_id": true, "confirmation_code": true, "activation_code": true,
		"confirm_code": true, "match_id": true,
	}
)

func redactSecret(a slog.Attr) slog.Attr {
	if secretKeys[strings.ToLower(a.Key)] {
		if a.Value.String() == "" {
			return slog.String(a.Key, "(none)")
		}
		return slog.String(a.Key, "(redacted)")
	}
	return a
}

// Mask keeps the first 7 characters of an identifier (EID, ICCID) and
// hides the rest, as the UI does.
func Mask(s string) string { return logmask.Mask(s) }
