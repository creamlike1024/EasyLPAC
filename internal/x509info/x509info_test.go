package x509info

import (
	"encoding/pem"
	"os"
	"strings"
	"testing"
)

// The GSMA test CIs on brainpoolP256r1, which crypto/x509 cannot read,
// decode as well as those on P-256.
func TestParseBundles(t *testing.T) {
	curves := map[string]int{}
	for _, f := range []string{"bundle.pem", "bundle-tests.pem", "bundle-china.pem"} {
		data, err := os.ReadFile("../registry/data/ci/" + f)
		if err != nil {
			t.Fatal(err)
		}
		for {
			var b *pem.Block
			if b, data = pem.Decode(data); b == nil {
				break
			}
			info, err := Parse(b.Bytes)
			if err != nil {
				t.Fatalf("%s: %v", f, err)
			}
			if info.Version != 3 || info.SubjectKeyID == "" || len(info.Subject) == 0 || info.NotAfter.Before(info.NotBefore) {
				t.Fatalf("%s: %+v", f, info)
			}
			curves[info.KeyCurve]++
			if !strings.HasPrefix(string(info.JSON()), "{\n  \"version\": 3") {
				t.Fatalf("json %s", info.JSON())
			}
		}
	}
	if curves["brainpoolP256r1"] == 0 || curves["prime256v1 (NIST P-256)"] == 0 {
		t.Fatalf("curves %v", curves)
	}
}
