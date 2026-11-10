// Package logmask obscures known identifiers (EID, ICCID, IMEI) and
// secrets (matching ID) in raw protocol logs: APDUs in hex and ES9+
// bodies, JSON with base64 ASN.1 inside. A value is looked for in each
// encoding it travels in, as the card and the SM-DP+ send it:
//
//   - its digits, as an EID's BCD in hex or a JSON field;
//   - nibble-swapped BCD in hex, padded with F (ICCID, IMEI on the card);
//   - its characters in hex (the EID in a certificate's serialNumber,
//     the matching ID in ctxParams1);
//   - those bytes in base64, at each of the three byte alignments.
//
// A value cut by the end or the start of a text, where an exchange is
// split over STORE DATA or GET RESPONSE blocks, is obscured too when at
// least edgeMin characters of it are there.
package logmask

import (
	"encoding/base64"
	"encoding/hex"
	"sort"
	"strings"
	"sync"
)

// Kind says how a value is obscured.
type Kind int

const (
	// Identifier is a digit (or hex) string: its digits keep their first
	// MaskKeep characters, as the UI shows identifiers; its other
	// encodings are starred.
	Identifier Kind = iota
	// Secret is starred in every encoding.
	Secret
)

// MaskKeep is how many leading characters Mask keeps.
const MaskKeep = 7

const (
	// minIdentifier and minSecret leave out values too short to look
	// for without obscuring unrelated data.
	minIdentifier = 8
	minSecret     = 4
	// minPattern is the shortest encoded form looked for, edgeMin the
	// shortest cut piece of one.
	minPattern = 6
	edgeMin    = 6
)

// Mask keeps the first MaskKeep characters of s and stars the rest.
func Mask(s string) string {
	if len(s) <= MaskKeep {
		return s
	}
	return s[:MaskKeep] + strings.Repeat("*", len(s)-MaskKeep)
}

// Set is a set of values to obscure. The zero value is empty and ready;
// a nil *Set obscures nothing. It is safe for concurrent use.
type Set struct {
	mu     sync.RWMutex
	seen   map[string]bool
	repl   map[string]string // encoded form -> its replacement
	r      *strings.Replacer
	edges  map[string][]edge // each edgeMin-long window of a form
	frozen bool              // r and edges match repl
}

// edge is a window of a form at pos.
type edge struct {
	form string
	pos  int
}

// Add adds values of kind k; empty, short and known values are skipped.
func (s *Set) Add(k Kind, values ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range values {
		key := string(rune('0'+k)) + v
		if s.seen[key] {
			continue
		}
		forms := encodings(k, v)
		if len(forms) == 0 {
			continue
		}
		if s.seen == nil {
			s.seen, s.repl = map[string]bool{}, map[string]string{}
		}
		s.seen[key] = true
		for f, r := range forms {
			if old, ok := s.repl[f]; !ok || strings.Count(r, "*") > strings.Count(old, "*") {
				s.repl[f] = r
			}
		}
		s.frozen = false
	}
}

// Empty reports whether there is nothing to obscure.
func (s *Set) Empty() bool {
	if s == nil {
		return true
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.repl) == 0
}

// Apply returns text with the values of the set obscured.
func (s *Set) Apply(text string) string {
	if s == nil || text == "" {
		return text
	}
	r, edges := s.compiled()
	if r == nil {
		return text
	}
	text = r.Replace(text)
	return cutEdges(text, edges)
}

func (s *Set) compiled() (*strings.Replacer, map[string][]edge) {
	s.mu.RLock()
	if s.frozen || len(s.repl) == 0 {
		defer s.mu.RUnlock()
		return s.r, s.edges
	}
	s.mu.RUnlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.frozen {
		forms := make([]string, 0, len(s.repl))
		for f := range s.repl {
			forms = append(forms, f)
		}
		// Longer forms first: a replacer tries its patterns in order.
		sort.Slice(forms, func(i, j int) bool {
			if len(forms[i]) != len(forms[j]) {
				return len(forms[i]) > len(forms[j])
			}
			return forms[i] < forms[j]
		})
		pairs := make([]string, 0, 2*len(forms))
		edges := map[string][]edge{}
		for _, f := range forms {
			pairs = append(pairs, f, s.repl[f])
			for p := 0; p+edgeMin <= len(f); p++ {
				w := f[p : p+edgeMin]
				edges[w] = append(edges[w], edge{f, p})
			}
		}
		s.r, s.edges, s.frozen = strings.NewReplacer(pairs...), edges, true
	}
	return s.r, s.edges
}

// cutEdges stars the longest piece of a form that ends text (its start
// cut off the form's tail) and the longest that starts it.
func cutEdges(text string, edges map[string][]edge) string {
	if len(text) < edgeMin {
		return text
	}
	tail, head := 0, 0
	for _, e := range edges[text[len(text)-edgeMin:]] {
		// The window ends text: the form's piece up to the window's end.
		if n := e.pos + edgeMin; n > tail && n < len(e.form) && strings.HasSuffix(text, e.form[:n]) {
			tail = n
		}
	}
	for _, e := range edges[text[:edgeMin]] {
		// The window starts text: the form's piece from the window on.
		if n := len(e.form) - e.pos; n > head && e.pos > 0 && strings.HasPrefix(text, e.form[e.pos:]) {
			head = n
		}
	}
	if tail+head > len(text) {
		return strings.Repeat("*", len(text))
	}
	if tail > 0 {
		text = text[:len(text)-tail] + strings.Repeat("*", tail)
	}
	if head > 0 {
		text = strings.Repeat("*", head) + text[head:]
	}
	return text
}

// encodings returns each form v travels in and its replacement.
func encodings(k Kind, v string) map[string]string {
	forms := map[string]string{}
	star := func(f string) {
		if len(f) >= minPattern {
			forms[f] = strings.Repeat("*", len(f))
		}
	}
	var raw [][]byte
	switch k {
	case Identifier:
		v = strings.ToUpper(v)
		if len(v) < minIdentifier || !isHex(v) {
			return nil
		}
		forms[v] = Mask(v)
		forms[strings.ToLower(v)] = Mask(strings.ToLower(v))
		if len(v)%2 == 0 {
			b, _ := hex.DecodeString(v)
			raw = append(raw, b)
		}
		sw := swapNibbles(v)
		star(sw)
		star(strings.ToLower(sw))
		b, _ := hex.DecodeString(sw)
		raw = append(raw, b)
	case Secret:
		if len(v) < minSecret {
			return nil
		}
		star(v)
	default:
		return nil
	}
	ascii := hex.EncodeToString([]byte(v))
	star(ascii)
	star(strings.ToUpper(ascii))
	raw = append(raw, []byte(v))
	for _, b := range raw {
		for shift := range 3 {
			star(base64Inner(b, shift))
		}
	}
	return forms
}

// isHex reports whether v, upper case, is hex digits: an identifier's
// digits, with F padding or as libeuicc renders an EID.
func isHex(v string) bool {
	for i := 0; i < len(v); i++ {
		if (v[i] < '0' || v[i] > '9') && (v[i] < 'A' || v[i] > 'F') {
			return false
		}
	}
	return true
}

// swapNibbles writes digits as the card stores an ICCID or IMEI: F
// padded to whole bytes, each byte's digits swapped.
func swapNibbles(v string) string {
	if len(v)%2 == 1 {
		v += "F"
	}
	b := []byte(v)
	for i := 0; i+1 < len(b); i += 2 {
		b[i], b[i+1] = b[i+1], b[i]
	}
	return string(b)
}

// base64Inner is the base64 of p, starting shift bytes into a 3-byte
// group, without the characters that bytes around p change.
func base64Inner(p []byte, shift int) string {
	buf := make([]byte, shift+len(p)+2)
	copy(buf[shift:], p)
	enc := base64.StdEncoding.EncodeToString(buf)
	start := (8*shift + 5) / 6
	end := 8 * (shift + len(p)) / 6
	if end <= start {
		return ""
	}
	return enc[start:end]
}
