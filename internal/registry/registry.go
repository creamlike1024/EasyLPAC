// Package registry holds the eUICC reference data from the eUICC Manual
// (https://euicc-manual.osmocom.org), committed in data/ (updated by hand
// with scripts/update-euicc-data.sh), embedded, and never loaded online:
//
//   - eUICC manufacturers (docs/pki/eum, accredited.json): EUM
//     identification number prefixes of EIDs, SAS-accredited sites and
//     their certificates;
//   - certificate issuers (docs/pki/ci, manifest.json), by key ID, with
//     their certificates (files/<key id>.txt) and whether they are live,
//     test or mainland China CIs (bundle*.pem);
//   - SM-DP+ error codes (docs/rsp/sm-dp-plus, error-codes.json): the
//     meaning of ES9+ subject and reason code pairs.
package registry

import (
	"embed"
	"encoding/json"
	"encoding/pem"
	"sort"
	"strings"
	"sync"

	"EasyLPAC/internal/x509info"
)

var (
	//go:embed data/accredited.json
	accreditedJSON []byte
	//go:embed data/manifest.json
	manifestJSON []byte
	//go:embed data/error-codes.json
	errorCodesJSON []byte
	//go:embed data/ci
	ciFiles embed.FS
	//go:embed data/VERSION
	versionFile string
)

// DataVersion is the date (UTC, YYYY-MM-DD) the embedded data was
// downloaded.
var DataVersion = strings.TrimSpace(versionFile)

// Supplier is an eUICC manufacturer or a SAS-accredited supplier.
type Supplier struct {
	Name string `json:"name"`
	// Abbr is the short name, if any.
	Abbr    string `json:"abbr"`
	URL     string `json:"url"`
	Country string `json:"country"`
	// EUM lists the EUM identification number prefixes of its EIDs.
	EUM []string `json:"eum"`
	// Locations maps SAS site codes (as "GD-MM") to their countries.
	Locations map[string]string `json:"locations"`
	// Deprecated explains why the supplier no longer exists as such,
	// such as a merger.
	Deprecated string `json:"deprecated"`
}

// ShortName is the abbreviation, else the name.
func (s *Supplier) ShortName() string {
	if s.Abbr != "" {
		return s.Abbr
	}
	return s.Name
}

// Category is the kind of a certificate issuer, by the eUICC Manual's
// bundles.
type Category string

const (
	CategoryLive  Category = "live"
	CategoryTest  Category = "test"
	CategoryChina Category = "china"
)

// Issuer is a certificate issuer (CI).
type Issuer struct {
	// KeyID is a prefix of the subject key identifier, in lower case hex.
	KeyID   string   `json:"key-id"`
	Name    string   `json:"name"`
	Country string   `json:"country"`
	CRLs    []string `json:"crls"`
	// Category is empty when no bundle lists the CI.
	Category Category `json:"-"`
	// Certificate is the CI's certificate (DER), nil when the eUICC
	// Manual publishes none.
	Certificate []byte `json:"-"`
	// IssuedByCI is set when the published certificate is not the CI's
	// own but one it issued (its authority key ID, not its subject key
	// ID, is the CI's key ID), as for some mainland China CIs.
	IssuedByCI bool `json:"-"`
}

type accredited struct {
	Version   int         `json:"version"`
	Suppliers []*Supplier `json:"suppliers"`
	// Certificates maps SAS certificate numbers to their documents.
	Certificates map[string]string `json:"certificates"`
}

// ErrorCode is the meaning of an ES9+ status (SGP.22 subject and reason
// code).
type ErrorCode struct {
	Subject     string `json:"subject-code"`
	Reason      string `json:"reason-code"`
	Description string `json:"description"`
}

var (
	loadOnce   sync.Once
	data       accredited
	issuers    []*Issuer
	errorCodes []*ErrorCode
)

func load() {
	if err := json.Unmarshal(accreditedJSON, &data); err != nil {
		panic("registry: embedded accredited.json: " + err.Error())
	}
	if err := json.Unmarshal(manifestJSON, &issuers); err != nil {
		panic("registry: embedded manifest.json: " + err.Error())
	}
	if err := json.Unmarshal(errorCodesJSON, &errorCodes); err != nil {
		panic("registry: embedded error-codes.json: " + err.Error())
	}
	loadCertificates()
}

// loadCertificates attaches each CI's certificate (files/<key id>.txt:
// the openssl text, then the PEM) and its category (the bundle whose
// certificate has a subject key ID the CI's key ID begins).
func loadCertificates() {
	bundles := map[Category]string{CategoryLive: "bundle.pem", CategoryTest: "bundle-tests.pem", CategoryChina: "bundle-china.pem"}
	skis := map[string]Category{}
	for cat, name := range bundles {
		data, _ := ciFiles.ReadFile("data/ci/" + name)
		for {
			var b *pem.Block
			if b, data = pem.Decode(data); b == nil {
				break
			}
			if ski := x509info.SubjectKeyID(b.Bytes); ski != "" {
				skis[ski] = cat
			}
		}
	}
	for _, ci := range issuers {
		if txt, err := ciFiles.ReadFile("data/ci/" + ci.KeyID + ".txt"); err == nil {
			if b, _ := pem.Decode(txt); b != nil && b.Type == "CERTIFICATE" {
				ci.Certificate = b.Bytes
				ci.IssuedByCI = !strings.HasPrefix(x509info.SubjectKeyID(b.Bytes), ci.KeyID)
			}
		}
		for ski, cat := range skis {
			if strings.HasPrefix(ski, ci.KeyID) {
				ci.Category = cat
			}
		}
	}
}

// Issuers returns every certificate issuer, by name.
func Issuers() []*Issuer {
	loadOnce.Do(load)
	out := append([]*Issuer(nil), issuers...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// LookupErrorCode returns the meaning of an ES9+ subject and reason code
// pair, or "".
func LookupErrorCode(subject, reason string) string {
	loadOnce.Do(load)
	for _, e := range errorCodes {
		if e.Subject == subject && e.Reason == reason {
			return e.Description
		}
	}
	return ""
}

// LookupEUM returns the manufacturer whose EUM identification number
// begins the EID, the longest match, or nil. The EUM identification
// number is of variable length (SGP.29 §10).
func LookupEUM(eid string) *Supplier {
	s, _ := LookupEUMPrefix(eid)
	return s
}

// LookupEUMPrefix is LookupEUM that also returns the EUM identification
// number that matched.
func LookupEUMPrefix(eid string) (*Supplier, string) {
	loadOnce.Do(load)
	var best *Supplier
	prefix := ""
	for _, s := range data.Suppliers {
		for _, p := range s.EUM {
			if len(p) > len(prefix) && strings.HasPrefix(eid, p) {
				best, prefix = s, p
			}
		}
	}
	return best, prefix
}

// LookupSite returns the supplier of the SAS-accredited site a SAS
// accreditation number (as "HG-BG-UP-0926") belongs to, and the site's
// country.
func LookupSite(sasNumber string) (*Supplier, string) {
	s, _, country := LookupSiteCode(sasNumber)
	return s, country
}

// LookupSiteCode is LookupSite that also returns the site's code (as
// "HG-BG").
func LookupSiteCode(sasNumber string) (s *Supplier, code, country string) {
	loadOnce.Do(load)
	sasNumber = strings.ToUpper(strings.TrimSpace(sasNumber))
	for _, sup := range data.Suppliers {
		for c, cc := range sup.Locations {
			if strings.HasPrefix(sasNumber, strings.ToUpper(c)+"-") {
				return sup, strings.ToUpper(c), cc
			}
		}
	}
	return nil, "", ""
}

// CertificateURL returns the document of a SAS certificate, or "".
func CertificateURL(sasNumber string) string {
	loadOnce.Do(load)
	return data.Certificates[strings.ToUpper(strings.TrimSpace(sasNumber))]
}

// LookupIssuer returns the certificate issuer of a key ID (hex, any
// case), matched by the issuer's key ID prefix, or nil.
func LookupIssuer(keyID string) *Issuer {
	loadOnce.Do(load)
	keyID = strings.ToLower(keyID)
	for _, ci := range issuers {
		if ci.KeyID != "" && strings.HasPrefix(keyID, strings.ToLower(ci.KeyID)) {
			return ci
		}
	}
	return nil
}

// CountryFlag returns the regional indicator emoji for an ISO 3166-1
// alpha-2 code, or a globe for anything else.
func CountryFlag(countryCode string) string {
	if len(countryCode) != 2 {
		return "🌎"
	}
	cc := strings.ToUpper(countryCode)
	if cc[0] < 'A' || cc[0] > 'Z' || cc[1] < 'A' || cc[1] > 'Z' {
		return "🌎"
	}
	return string([]rune{rune(cc[0]-'A') + 0x1F1E6, rune(cc[1]-'A') + 0x1F1E6})
}
