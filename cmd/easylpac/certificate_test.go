package main

import (
	"strings"
	"testing"

	"EasyLPAC/internal/registry"
)

// A CI on brainpoolP256r1, which crypto/x509 cannot read, opens in the
// certificate dialog all the same.
func TestCertificateDialogBrainpool(t *testing.T) {
	f := newFixture(t, newFakeCard())
	f.tt.Frame()
	const keyID = "c0bc70ba36929d43b467ff57570530e57ab8fcd8"
	ci := registry.LookupIssuer(keyID)
	if ci == nil || ci.Certificate == nil {
		t.Fatal("GSMA test CI c0bc70 has no certificate")
	}
	f.g.openCertificate(ci, keyID)
	f.g.page = pageAbout
	f.tt.Frame()
	if !f.g.certOpen || !f.tt.HasText(ci.Name) || !f.tt.HasText("Copy PEM") || !f.tt.HasText("Key ID "+keyID) {
		t.Fatalf("dialog: %q", f.tt.Texts())
	}
	setTreeOpen(f.g.certTree, f.g.certTreeOpen, true)
	f.tt.Frame()
	if !f.tt.HasText("brainpoolP256r1") {
		t.Fatalf("curve not shown: %q", f.tt.Texts())
	}
	f.click(t, "Copy PEM")
	if clip := f.g.clip.(*memClipboard).text; !strings.HasPrefix(clip, "-----BEGIN CERTIFICATE-----") {
		t.Fatalf("clipboard %q", clip)
	}
}
