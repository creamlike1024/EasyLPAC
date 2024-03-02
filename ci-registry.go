//go:generate curl -o ci-registry.json https://euicc-manual.septs.app/docs/pki/ci/manifest.json
package main

import (
	_ "embed"
	"encoding/json"
	"strings"
)

//go:embed ci-registry.json
var ciRegistryBundle []byte

type CertificateIssuer struct {
	KeyID   string `json:"key-id"`
	Country string `json:"country"`
	Name    string `json:"name"`
}

var issuerRegistry []*CertificateIssuer

func init() {
	if err := json.Unmarshal(ciRegistryBundle, &issuerRegistry); err != nil {
		panic(err)
	}
}

func GetIssuer(keyId string) *CertificateIssuer {
	for _, issuer := range issuerRegistry {
		if strings.HasPrefix(keyId, issuer.KeyID) {
			return issuer
		}
	}
	return nil
}
