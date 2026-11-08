package app

import (
	"errors"
	"testing"

	"EasyLPAC/internal/settings"
)

func TestParseAID(t *testing.T) {
	aid, err := ParseAID(settings.AIDDefault)
	if err != nil || len(aid) != 16 || aid[0] != 0xA0 || aid[15] != 0x00 {
		t.Fatalf("got %x %v", aid, err)
	}
	for _, s := range []string{"", "A000", settings.AIDDefault + "00", "ZZ000005591010FFFFFFFF8900000100"} {
		if _, err := ParseAID(s); err == nil {
			t.Errorf("ParseAID(%q): expected error", s)
		}
	}
}

func TestNewServiceRejectsUnusableConfig(t *testing.T) {
	cfg := &settings.Config{
		Backend:  "qmi",
		AID:      settings.AIDDefault,
		Backends: map[string]settings.Backend{},
	}
	if _, err := NewService(cfg, nil); !errors.Is(err, ErrBackendUnsupported) {
		t.Fatalf("unsupported backend: err = %v", err)
	}

	cfg.Backend = settings.BackendPCSC
	if _, err := NewService(cfg, nil); !errors.Is(err, ErrDeviceNotConfigured) {
		t.Fatalf("no reader: err = %v", err)
	}

	cfg.Backends[settings.BackendPCSC] = settings.Backend{Reader: "Reader A"}
	if _, err := NewService(cfg, nil); err != nil {
		t.Fatalf("configured pcsc: %v", err)
	}
}
