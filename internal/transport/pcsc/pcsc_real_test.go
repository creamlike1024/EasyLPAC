package pcsc

import (
	"context"
	"testing"

	"EasyLPAC/internal/transport"
)

// TestRealPCSCStack exercises the actual macOS/Linux/Windows PC/SC
// service: context establishment and reader enumeration. It asserts
// only that the calls succeed; the reader list may legitimately be
// empty on machines without readers.
func TestRealPCSCStack(t *testing.T) {
	b := New()
	if !b.Available() {
		t.Skip("PC/SC service not available on this machine")
	}
	devices, err := b.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	t.Logf("readers: %d", len(devices))

	if _, err := b.Open(context.Background(), transport.DeviceConfig{}); err == nil {
		t.Fatal("Open without reader must fail")
	}
}
