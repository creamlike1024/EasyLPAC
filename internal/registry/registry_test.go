package registry

import (
	"regexp"
	"strings"
	"testing"

	"EasyLPAC/internal/x509info"
)

// The embedded data must have the shape the lookups rely on; the update
// script runs these tests to reject a download that does not.
func TestEmbeddedData(t *testing.T) {
	loadOnce.Do(load)
	if !regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`).MatchString(DataVersion) {
		t.Fatalf("DataVersion %q is not a date", DataVersion)
	}
	if data.Version != 1 {
		t.Fatalf("accredited.json version %d, want 1", data.Version)
	}
	eums, sites := 0, 0
	for _, s := range data.Suppliers {
		if s.Name == "" {
			t.Fatal("supplier without a name")
		}
		eums += len(s.EUM)
		sites += len(s.Locations)
	}
	if len(data.Suppliers) < 10 || eums < 10 || sites < 10 || len(data.Certificates) < 10 {
		t.Fatalf("too little data: suppliers=%d eum=%d sites=%d certs=%d", len(data.Suppliers), eums, sites, len(data.Certificates))
	}
	if len(issuers) < 10 {
		t.Fatalf("only %d certificate issuers", len(issuers))
	}
	for _, ci := range issuers {
		if !regexp.MustCompile(`^[0-9a-f]+$`).MatchString(ci.KeyID) || ci.Name == "" {
			t.Fatalf("bad issuer %+v", ci)
		}
	}
	if len(errorCodes) < 10 {
		t.Fatalf("only %d error codes", len(errorCodes))
	}
	code := regexp.MustCompile(`^\d+(\.\d+)+$`)
	for _, e := range errorCodes {
		if !code.MatchString(e.Subject) || !code.MatchString(e.Reason) || e.Description == "" {
			t.Fatalf("bad error code %+v", e)
		}
	}
}

func TestLookupErrorCode(t *testing.T) {
	if d := LookupErrorCode("8.2.7", "3.8"); d == "" {
		t.Fatal("8.2.7/3.8 (confirmation code refused) not found")
	}
	if d := LookupErrorCode("8.2.7", "9.9"); d != "" {
		t.Fatalf("unexpected %q", d)
	}
}

func TestLookups(t *testing.T) {
	loadOnce.Do(load)
	var withEUM *Supplier
	for _, s := range data.Suppliers {
		if len(s.EUM) > 0 {
			withEUM = s
			break
		}
	}
	if got := LookupEUM(withEUM.EUM[0] + "000000000000000000000000"); got != withEUM {
		t.Fatalf("LookupEUM = %+v", got)
	}
	if LookupEUM("00000000") != nil {
		t.Fatal("unexpected EUM match")
	}

	// GSMA RSP2 Root CI1, as cards list its full key ID.
	if ci := LookupIssuer("81370F5125D0B1D408D4C3B232E6D25E795BEBFB"); ci == nil || ci.Name == "" {
		t.Fatalf("LookupIssuer = %+v", ci)
	}
	if LookupIssuer("ffffffffffff") != nil {
		t.Fatal("unexpected issuer match")
	}
}

func TestLookupSite(t *testing.T) {
	loadOnce.Do(load)
	var sup *Supplier
	var code, country string
	for _, s := range data.Suppliers {
		for k, v := range s.Locations {
			sup, code, country = s, k, v
			break
		}
		if sup != nil {
			break
		}
	}
	got, gotCountry := LookupSite(code + "-UP-0001")
	if got != sup || gotCountry != country {
		t.Fatalf("LookupSite(%s) = %v %q", code, got, gotCountry)
	}
	if s, _ := LookupSite("ZZ-ZZ-UP-0001"); s != nil {
		t.Fatal("unexpected site match")
	}
	for num, url := range data.Certificates {
		if CertificateURL(num) != url {
			t.Fatalf("CertificateURL(%s)", num)
		}
		break
	}
}

func TestCountryFlag(t *testing.T) {
	if got := CountryFlag("jp"); got != "🇯🇵" {
		t.Fatalf("got %q", got)
	}
	for _, in := range []string{"", "J", "JPN", "1A"} {
		if got := CountryFlag(in); got != "🌎" {
			t.Errorf("CountryFlag(%q) = %q", in, got)
		}
	}
}

func TestCertificates(t *testing.T) {
	loadOnce.Do(load)
	withCert, cats := 0, map[Category]int{}
	for _, ci := range issuers {
		if ci.Certificate == nil {
			continue
		}
		withCert++
		cats[ci.Category]++
		info, err := x509info.Parse(ci.Certificate)
		if err != nil {
			t.Fatalf("CI %s: %v", ci.KeyID, err)
		}
		// The certificate is the CI's own (its key ID names it), or one
		// the CI issued (its authority key ID does).
		ski := strings.ReplaceAll(info.SubjectKeyID, ":", "")
		aki := strings.ReplaceAll(info.AuthorityKeyID, ":", "")
		switch {
		case strings.HasPrefix(ski, ci.KeyID) && !ci.IssuedByCI:
		case strings.HasPrefix(aki, ci.KeyID) && ci.IssuedByCI:
		default:
			t.Errorf("CI %s: certificate ski=%s aki=%s issuedByCI=%v", ci.KeyID, ski, aki, ci.IssuedByCI)
		}
	}
	if withCert < 10 || cats[CategoryLive] == 0 || cats[CategoryTest] == 0 || cats[CategoryChina] == 0 {
		t.Fatalf("certificates %d, categories %v", withCert, cats)
	}
}
