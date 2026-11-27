package app

import (
	"errors"
	"slices"
	"testing"
)

func TestParseInitCommands(t *testing.T) {
	got, err := ParseInitCommands("  AT+CMEE=2 \r\n\n at+qcfg=\"usbnet\"\n")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"AT+CMEE=2", "at+qcfg=\"usbnet\""}; !slices.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got, err := ParseInitCommands(" \n"); err != nil || got != nil {
		t.Fatalf("blank: %q, %v", got, err)
	}
	_, err = ParseInitCommands("AT+CMEE=2\n\nCFUN=1")
	var lineErr *InitCommandError
	if !errors.As(err, &lineErr) || lineErr.Line != 3 {
		t.Fatalf("err = %v", err)
	}
}
