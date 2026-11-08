package app

import (
	"encoding/json"
	"strings"
	"testing"

	"EasyLPAC/internal/lpa"
)

func TestValidEID(t *testing.T) {
	// Check digits per SGP.29 §10: set them to 00, take 98 - (n mod 97).
	base := "890490320000010000001234567890"
	n := 0
	for _, r := range base + "00" {
		n = (n*10 + int(r-'0')) % 97
	}
	cd := 98 - n
	eid := base + string(rune('0'+cd/10)) + string(rune('0'+cd%10))
	if !ValidEID(eid) {
		t.Fatalf("%s should be valid", eid)
	}
	bad := eid[:31] + string(rune('0'+(int(eid[31]-'0')+1)%10))
	for _, s := range []string{bad, eid[:31], eid + "0", strings.Replace(eid, "9", "A", 1)} {
		if ValidEID(s) {
			t.Errorf("%s should be invalid", s)
		}
	}
}

func TestEUICCInfo2JSONUsesLpacNames(t *testing.T) {
	var i lpa.EUICCInfo2
	i.Svn = "2.2.2"
	i.ExtCardResource.FreeNonVolatileMemory = 482655
	i.EuiccCiPKIDListForVerification = []string{"81370f5125d0b1d408d4c3b232e6d25e795bebfb"}
	var m map[string]any
	if err := json.Unmarshal(EUICCInfo2JSON(i), &m); err != nil {
		t.Fatal(err)
	}
	if m["svn"] != "2.2.2" || m["extCardResource"].(map[string]any)["freeNonVolatileMemory"] != float64(482655) {
		t.Fatalf("json %v", m)
	}
	if _, ok := m["euiccCiPKIdListForVerification"]; !ok {
		t.Fatalf("lpac key name missing: %v", m)
	}
}

func TestServerError(t *testing.T) {
	err := &lpa.OpError{Stage: lpa.StageDownload, Op: "download",
		Cause: &lpa.ES9Error{Stage: "es9p_authenticate_client", Subject: "8.2.7", Reason: "3.8", Message: "Refused"}}
	summary, codes, ok := ServerError(err)
	if !ok || summary == "Refused" || summary == "" || codes != "subject 8.2.7, reason 3.8" {
		t.Fatalf("got %q %q %v", summary, codes, ok)
	}
	if d := DescribeError(err); !strings.HasPrefix(d, "SM-DP+: ") {
		t.Fatalf("got %q", d)
	}
	// Unknown pairs fall back to the server's message.
	summary, _, _ = ServerError(&lpa.ES9Error{Subject: "8.99", Reason: "1.1", Message: "Custom text"})
	if summary != "Custom text" {
		t.Fatalf("got %q", summary)
	}
}
