// Package x509info decodes X.509 certificates for display. It reads the
// ASN.1 structure itself (encoding/asn1) instead of crypto/x509, which
// rejects keys on curves it does not implement, such as the
// brainpoolP256r1 of GSMA test CIs: every certificate shows, whatever
// its algorithms. It does not verify anything.
package x509info

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
)

type certificate struct {
	TBS       tbsCertificate
	SigAlg    algorithmIdentifier
	Signature asn1.BitString
}

type tbsCertificate struct {
	Raw        asn1.RawContent
	Version    int `asn1:"optional,explicit,default:0,tag:0"`
	Serial     *big.Int
	SigAlg     algorithmIdentifier
	Issuer     asn1.RawValue
	Validity   validity
	Subject    asn1.RawValue
	PublicKey  publicKeyInfo
	IssuerUID  asn1.BitString `asn1:"optional,tag:1"`
	SubjectUID asn1.BitString `asn1:"optional,tag:2"`
	Extensions []extension    `asn1:"optional,explicit,tag:3"`
}

type algorithmIdentifier struct {
	Algorithm  asn1.ObjectIdentifier
	Parameters asn1.RawValue `asn1:"optional"`
}

type validity struct {
	NotBefore, NotAfter time.Time
}

type publicKeyInfo struct {
	Algorithm algorithmIdentifier
	PublicKey asn1.BitString
}

type extension struct {
	ID       asn1.ObjectIdentifier
	Critical bool `asn1:"optional"`
	Value    []byte
}

// attributeSET is a RelativeDistinguishedName, a SET OF attributes
// (encoding/asn1 reads a SET for a type whose name ends in SET).
type attributeSET []attribute

type attribute struct {
	Type  asn1.ObjectIdentifier
	Value asn1.RawValue
}

// Attr is one attribute of a distinguished name.
type Attr struct{ Name, Value string }

// Extension is a decoded certificate extension.
type Extension struct {
	Name     string
	Critical bool
	// Values are its decoded contents, one per line; the hex of the
	// value when it is not one this package knows.
	Values []string
}

// Info is what a certificate says.
type Info struct {
	Version            int
	Serial             string
	SignatureAlgorithm string
	Issuer, Subject    []Attr
	NotBefore          time.Time
	NotAfter           time.Time
	KeyAlgorithm       string
	KeyCurve           string
	PublicKey          string
	SubjectKeyID       string
	AuthorityKeyID     string
	Extensions         []Extension
	SHA256, SHA1       string
	SelfSigned         bool
}

// Parse decodes a DER certificate.
func Parse(der []byte) (*Info, error) {
	var c certificate
	rest, err := asn1.Unmarshal(der, &c)
	if err != nil {
		return nil, fmt.Errorf("x509info: %w", err)
	}
	if len(rest) > 0 {
		return nil, errors.New("x509info: trailing data after certificate")
	}
	t := c.TBS
	info := &Info{
		Version:            t.Version + 1,
		SignatureAlgorithm: oidName(c.SigAlg.Algorithm),
		NotBefore:          t.Validity.NotBefore.UTC(),
		NotAfter:           t.Validity.NotAfter.UTC(),
		KeyAlgorithm:       oidName(t.PublicKey.Algorithm.Algorithm),
		PublicKey:          colonHex(t.PublicKey.PublicKey.Bytes),
		SelfSigned:         bytes.Equal(t.Issuer.FullBytes, t.Subject.FullBytes),
	}
	if t.Serial != nil {
		info.Serial = colonHex(t.Serial.Bytes())
	}
	if info.Issuer, err = parseName(t.Issuer.FullBytes); err != nil {
		return nil, err
	}
	if info.Subject, err = parseName(t.Subject.FullBytes); err != nil {
		return nil, err
	}
	var curve asn1.ObjectIdentifier
	if _, err := asn1.Unmarshal(t.PublicKey.Algorithm.Parameters.FullBytes, &curve); err == nil {
		info.KeyCurve = oidName(curve)
	}
	for _, e := range t.Extensions {
		x := Extension{Name: oidName(e.ID), Critical: e.Critical, Values: decodeExtension(e.ID, e.Value)}
		switch e.ID.String() {
		case "2.5.29.14":
			info.SubjectKeyID = strings.Join(x.Values, "")
		case "2.5.29.35":
			info.AuthorityKeyID = strings.TrimPrefix(firstWithPrefix(x.Values, "keyid:"), "keyid:")
		}
		info.Extensions = append(info.Extensions, x)
	}
	s256, s1 := sha256.Sum256(der), sha1.Sum(der)
	info.SHA256, info.SHA1 = colonHex(s256[:]), colonHex(s1[:])
	return info, nil
}

// SubjectKeyID returns the subject key identifier of a DER certificate,
// in lower case hex without separators, or "".
func SubjectKeyID(der []byte) string {
	info, err := Parse(der)
	if err != nil {
		return ""
	}
	return strings.ReplaceAll(info.SubjectKeyID, ":", "")
}

func firstWithPrefix(vs []string, p string) string {
	for _, v := range vs {
		if strings.HasPrefix(v, p) {
			return v
		}
	}
	return ""
}

func parseName(der []byte) ([]Attr, error) {
	var rdns []attributeSET
	if _, err := asn1.Unmarshal(der, &rdns); err != nil {
		return nil, fmt.Errorf("x509info: name: %w", err)
	}
	var out []Attr
	for _, rdn := range rdns {
		for _, a := range rdn {
			out = append(out, Attr{Name: oidName(a.Type), Value: stringValue(a.Value)})
		}
	}
	return out, nil
}

// stringValue reads a directory string, else shows the value's hex.
func stringValue(v asn1.RawValue) string {
	var s string
	if _, err := asn1.Unmarshal(v.FullBytes, &s); err == nil {
		return s
	}
	if v.Class == asn1.ClassUniversal && v.Tag == 30 && len(v.Bytes)%2 == 0 { // BMPString
		r := make([]rune, 0, len(v.Bytes)/2)
		for i := 0; i < len(v.Bytes); i += 2 {
			r = append(r, rune(v.Bytes[i])<<8|rune(v.Bytes[i+1]))
		}
		return string(r)
	}
	return hex.EncodeToString(v.FullBytes)
}

func decodeExtension(id asn1.ObjectIdentifier, v []byte) []string {
	switch id.String() {
	case "2.5.29.14": // subject key identifier
		var ski []byte
		if _, err := asn1.Unmarshal(v, &ski); err == nil {
			return []string{colonHex(ski)}
		}
	case "2.5.29.35": // authority key identifier
		var aki struct {
			KeyID []byte `asn1:"optional,tag:0"`
		}
		if _, err := asn1.Unmarshal(v, &aki); err == nil && len(aki.KeyID) > 0 {
			return []string{"keyid:" + colonHex(aki.KeyID)}
		}
	case "2.5.29.15": // key usage
		var bits asn1.BitString
		if _, err := asn1.Unmarshal(v, &bits); err == nil {
			names := []string{"Digital Signature", "Non Repudiation", "Key Encipherment", "Data Encipherment",
				"Key Agreement", "Certificate Sign", "CRL Sign", "Encipher Only", "Decipher Only"}
			var out []string
			for i, n := range names {
				if bits.At(i) == 1 {
					out = append(out, n)
				}
			}
			return out
		}
	case "2.5.29.19": // basic constraints
		var bc struct {
			CA      bool `asn1:"optional"`
			PathLen int  `asn1:"optional,default:-1"`
		}
		if _, err := asn1.Unmarshal(v, &bc); err == nil {
			s := fmt.Sprintf("CA:%v", strings.ToUpper(fmt.Sprint(bc.CA)))
			if bc.PathLen >= 0 {
				s += fmt.Sprintf(", pathlen:%d", bc.PathLen)
			}
			return []string{s}
		}
	case "2.5.29.32": // certificate policies
		var pols []struct {
			Policy     asn1.ObjectIdentifier
			Qualifiers asn1.RawValue `asn1:"optional"`
		}
		if _, err := asn1.Unmarshal(v, &pols); err == nil {
			var out []string
			for _, p := range pols {
				out = append(out, "Policy: "+oidName(p.Policy))
			}
			return out
		}
	case "2.5.29.37": // extended key usage
		var ekus []asn1.ObjectIdentifier
		if _, err := asn1.Unmarshal(v, &ekus); err == nil {
			var out []string
			for _, e := range ekus {
				out = append(out, oidName(e))
			}
			return out
		}
	case "2.5.29.17", "2.5.29.18": // subject / issuer alternative name
		if out, ok := generalNames(v); ok {
			return out
		}
	case "2.5.29.31": // CRL distribution points
		var dps []struct {
			Name asn1.RawValue `asn1:"optional,explicit,tag:0"`
		}
		if _, err := asn1.Unmarshal(v, &dps); err == nil {
			var out []string
			for _, dp := range dps {
				// DistributionPointName.fullName [0] GeneralNames
				var full asn1.RawValue
				if _, err := asn1.Unmarshal(dp.Name.FullBytes, &full); err == nil && full.Tag == 0 {
					if names, ok := generalNamesContent(full.Bytes); ok {
						out = append(out, names...)
					}
				}
			}
			if len(out) > 0 {
				return out
			}
		}
	}
	return []string{colonHex(v)}
}

func generalNames(v []byte) ([]string, bool) {
	var seq asn1.RawValue
	if _, err := asn1.Unmarshal(v, &seq); err != nil {
		return nil, false
	}
	return generalNamesContent(seq.Bytes)
}

func generalNamesContent(b []byte) ([]string, bool) {
	var out []string
	for len(b) > 0 {
		var n asn1.RawValue
		var err error
		if b, err = asn1.Unmarshal(b, &n); err != nil {
			return nil, false
		}
		switch n.Tag {
		case 1:
			out = append(out, "email:"+string(n.Bytes))
		case 2:
			out = append(out, "DNS:"+string(n.Bytes))
		case 6:
			out = append(out, "URI:"+string(n.Bytes))
		case 8:
			// registeredID [8] IMPLICIT OBJECT IDENTIFIER
			raw := append([]byte{0x06, byte(len(n.Bytes))}, n.Bytes...)
			var oid asn1.ObjectIdentifier
			if _, err := asn1.Unmarshal(raw, &oid); err == nil {
				out = append(out, "Registered ID:"+oidName(oid))
				continue
			}
			out = append(out, "Registered ID:"+hex.EncodeToString(n.Bytes))
		default:
			out = append(out, fmt.Sprintf("[%d]:%s", n.Tag, hex.EncodeToString(n.Bytes)))
		}
	}
	return out, true
}

func colonHex(b []byte) string {
	parts := make([]string, len(b))
	for i, x := range b {
		parts[i] = fmt.Sprintf("%02x", x)
	}
	return strings.Join(parts, ":")
}

// oidNames are the object identifiers shown by name.
var oidNames = map[string]string{
	// signature algorithms
	"1.2.840.10045.4.3.2":   "ecdsa-with-SHA256",
	"1.2.840.10045.4.3.3":   "ecdsa-with-SHA384",
	"1.2.840.10045.4.3.4":   "ecdsa-with-SHA512",
	"1.2.840.113549.1.1.11": "sha256WithRSAEncryption",
	"1.2.840.113549.1.1.12": "sha384WithRSAEncryption",
	"1.2.840.113549.1.1.13": "sha512WithRSAEncryption",
	"1.2.156.10197.1.501":   "SM2-with-SM3",
	// key algorithms and curves
	"1.2.840.10045.2.1":     "id-ecPublicKey",
	"1.2.840.113549.1.1.1":  "rsaEncryption",
	"1.2.840.10045.3.1.7":   "prime256v1 (NIST P-256)",
	"1.3.132.0.34":          "secp384r1 (NIST P-384)",
	"1.3.132.0.35":          "secp521r1 (NIST P-521)",
	"1.3.36.3.3.2.8.1.1.7":  "brainpoolP256r1",
	"1.3.36.3.3.2.8.1.1.11": "brainpoolP384r1",
	"1.2.156.10197.1.301":   "SM2",
	// name attributes
	"2.5.4.3":              "commonName",
	"2.5.4.5":              "serialNumber",
	"2.5.4.6":              "countryName",
	"2.5.4.7":              "localityName",
	"2.5.4.8":              "stateOrProvinceName",
	"2.5.4.10":             "organizationName",
	"2.5.4.11":             "organizationalUnitName",
	"2.5.4.97":             "organizationIdentifier",
	"1.2.840.113549.1.9.1": "emailAddress",
	// extensions
	"2.5.29.14": "Subject Key Identifier",
	"2.5.29.15": "Key Usage",
	"2.5.29.17": "Subject Alternative Name",
	"2.5.29.18": "Issuer Alternative Name",
	"2.5.29.19": "Basic Constraints",
	"2.5.29.31": "CRL Distribution Points",
	"2.5.29.32": "Certificate Policies",
	"2.5.29.35": "Authority Key Identifier",
	"2.5.29.37": "Extended Key Usage",
	// GSMA RSP (SGP.22 §4.5.2.1)
	"2.23.146.1.2.1.0": "id-rspRole-ci (2.23.146.1.2.1.0)",
	"2.23.146.1.2.1.1": "id-rspRole-euicc (2.23.146.1.2.1.1)",
	"2.23.146.1.2.1.2": "id-rspRole-eum (2.23.146.1.2.1.2)",
}

func oidName(oid asn1.ObjectIdentifier) string {
	if n, ok := oidNames[oid.String()]; ok {
		return n
	}
	return oid.String()
}

// JSON returns the certificate's fields as indented JSON, in the order
// a certificate lists them.
func (i *Info) JSON() []byte {
	name := func(attrs []Attr) ordered {
		var o ordered
		for _, a := range attrs {
			o = append(o, kv{a.Name, a.Value})
		}
		return o
	}
	key := ordered{{"algorithm", i.KeyAlgorithm}}
	if i.KeyCurve != "" {
		key = append(key, kv{"curve", i.KeyCurve})
	}
	key = append(key, kv{"publicKey", i.PublicKey})
	var exts ordered
	for _, e := range i.Extensions {
		v := ordered{{"critical", e.Critical}}
		if len(e.Values) == 1 {
			v = append(v, kv{"value", e.Values[0]})
		} else {
			v = append(v, kv{"values", e.Values})
		}
		exts = append(exts, kv{e.Name, v})
	}
	o := ordered{
		{"version", i.Version},
		{"serialNumber", i.Serial},
		{"signatureAlgorithm", i.SignatureAlgorithm},
		{"issuer", name(i.Issuer)},
		{"validity", ordered{
			{"notBefore", i.NotBefore.Format(time.RFC3339)},
			{"notAfter", i.NotAfter.Format(time.RFC3339)},
		}},
		{"subject", name(i.Subject)},
		{"subjectPublicKeyInfo", key},
		{"extensions", exts},
		{"fingerprints", ordered{{"sha256", i.SHA256}, {"sha1", i.SHA1}}},
	}
	out, _ := json.MarshalIndent(o, "", "  ")
	return out
}

type kv struct {
	k string
	v any
}

// ordered is a JSON object that keeps its keys' order.
type ordered []kv

func (o ordered) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for n, e := range o {
		if n > 0 {
			b.WriteByte(',')
		}
		k, _ := json.Marshal(e.k)
		v, err := json.Marshal(e.v)
		if err != nil {
			return nil, err
		}
		b.Write(k)
		b.WriteByte(':')
		b.Write(v)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}
