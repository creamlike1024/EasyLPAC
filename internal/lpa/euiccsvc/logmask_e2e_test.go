package euiccsvc

// The protocol log of a whole download, through the real libeuicc, with
// the identifiers of the run obscured: none of them is left in any
// encoding the card or the SM-DP+ sent it in.

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"log/slog"
	"regexp"
	"strings"
	"testing"

	"EasyLPAC/internal/logmask"
	"EasyLPAC/internal/lpa"
	"EasyLPAC/internal/testcard"
	"EasyLPAC/internal/testsmdp"
)

func TestDownloadLogObscured(t *testing.T) {
	const matchID, imei = "MATCH-0123456789", "351234567890123"
	card := &testcard.Card{Download: true, InstallNotificationSeq: 42}
	smdp := testsmdp.New(t)
	svc := newDownloadService(t, card, smdp)
	var buf bytes.Buffer
	svc.cfg.Logger = slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	svc.cfg.DebugAPDU, svc.cfg.DebugHTTP = true, true

	if _, err := svc.ChipInfo(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ProfileList(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ProfileDownload(context.Background(), lpa.DownloadRequest{
		PullInfo: lpa.PullInfo{SMDP: smdp.Address(), MatchID: matchID, IMEI: imei},
		Confirm:  func(lpa.ProfileMetadata) (bool, error) { return true, nil },
	}); err != nil {
		t.Fatal(err)
	}
	raw := buf.String()
	if !strings.Contains(raw, "es10=AuthenticateServer") || !strings.Contains(raw, "sw=9000") {
		t.Fatalf("APDUs not logged as expected:\n%s", raw)
	}

	eid := hex.EncodeToString(testcard.EID)
	known := new(logmask.Set)
	known.Add(logmask.Identifier, eid, testcard.ICCID, testcard.ICCID2, imei)
	var out strings.Builder
	for _, line := range strings.Split(raw, "\n") {
		out.WriteString(known.Apply(line) + "\n")
	}
	masked := out.String()

	// The forms on the card and in hex.
	swapped := func(d string) string {
		if len(d)%2 == 1 {
			d += "F"
		}
		b := []byte(d)
		for i := 0; i+1 < len(b); i += 2 {
			b[i], b[i+1] = b[i+1], b[i]
		}
		return string(b)
	}
	forms := map[string]string{
		"EID":          strings.ToUpper(eid),
		"ICCID":        swapped(testcard.ICCID),
		"ICCID2":       swapped(testcard.ICCID2),
		"IMEI":         swapped(imei),
		"matching ID":  strings.ToUpper(hex.EncodeToString([]byte(matchID))),
		"matching ID ": matchID,
	}
	seen := map[string]bool{}
	for name, f := range forms {
		if strings.Contains(raw, f) {
			seen[name] = true
		}
		if strings.Contains(strings.ToUpper(masked), strings.ToUpper(f)) {
			t.Errorf("%s left as %s", name, f)
		}
	}
	// The forms in base64 bodies: decode what is left of each.
	byteForms := map[string][]byte{"EID": testcard.EID, "matching ID": []byte(matchID)}
	for _, name := range []string{"ICCID", "ICCID2", "IMEI"} {
		byteForms[name], _ = hex.DecodeString(forms[name])
	}
	b64 := regexp.MustCompile(`[A-Za-z0-9+/]{24,}={0,2}`)
	for _, tok := range b64.FindAllString(masked, -1) {
		dec, err := base64.StdEncoding.DecodeString(tok)
		if err != nil {
			continue
		}
		for name, f := range byteForms {
			if bytes.Contains(dec, f) {
				t.Errorf("%s left in base64 %s", name, tok)
			}
		}
	}
	for _, tok := range b64.FindAllString(raw, -1) {
		if dec, err := base64.StdEncoding.DecodeString(tok); err == nil {
			for name, f := range byteForms {
				if bytes.Contains(dec, f) {
					seen[name+" (base64)"] = true
				}
			}
		}
	}
	t.Logf("in the raw log: %v", seen)
	if testing.Verbose() {
		for _, line := range strings.Split(masked, "\n") {
			if strings.Contains(line, "msg=apdu") && strings.Contains(line, "es10=") {
				t.Log(line)
			}
		}
	}
}
