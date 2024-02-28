package main

import (
	"archive/zip"
	"bytes"
	"crypto/x509"
	_ "embed"
	"encoding/hex"
	"encoding/pem"
	"io"
)

//go:embed ci-registry.zip
var ciRegistryBundle []byte

func init() {
	reader, err := zip.NewReader(
		bytes.NewReader(ciRegistryBundle),
		int64(len(ciRegistryBundle)),
	)
	if err != nil {
		panic(err)
	}
	issuerRegistry = make(map[string]CertificateIssuer)
	for _, file := range reader.File {
		fp, err := file.Open()
		if err != nil {
			panic(err)
		}
		pemCertificate, err := io.ReadAll(fp)
		if err != nil {
			panic(err)
		}
		block, _ := pem.Decode(pemCertificate)
		certificate, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}
		issuer := certificate.Issuer
		keyId := certificate.AuthorityKeyId
		if certificate.IsCA {
			issuer = certificate.Subject
			keyId = certificate.SubjectKeyId
		}
		pemCertificateText := string(pemCertificate)
		var country string
		if len(issuer.Country) > 0 {
			country = issuer.Country[0]
		}
		issuerRegistry[hex.EncodeToString(keyId)] = CertificateIssuer{
			Country:    country,
			CommonName: issuer.CommonName,
			KeyID:      hex.EncodeToString(keyId),
			Text:       pemCertificateText,
		}
	}
}
