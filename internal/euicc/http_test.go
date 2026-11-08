package euicc

// Pure-Go tests for the HTTP adapter logic. The C trampoline itself
// (pointer/NULL-terminated-array marshalling) is covered by the C-side
// harness in scripts/test-http-shim.c, run by scripts/test-euicc.sh;
// cgo is not allowed inside *_test.go files.

import (
	"testing"
)

// headerSlice mirrors the trampoline's NULL-terminated array walk but
// operates on a Go slice; keeps the ordering contract testable.
func headerSlice(cHeaders []string) []string {
	var out []string
	for _, h := range cHeaders {
		if h == "" {
			break // sentinel
		}
		out = append(out, h)
	}
	return out
}

func TestHeaderSlice(t *testing.T) {
	got := headerSlice([]string{"a", "b", ""})
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("got %v", got)
	}
}
