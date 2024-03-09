//go:generate curl -o eum-registry.json https://euicc-manual.septs.app/docs/pki/eum/manifest.json
package main

import (
	_ "embed"
	"encoding/json"
	"path/filepath"
	"strings"
)

//go:embed eum-registry.json
var eumRegistryBundle []byte

type EUMIdentifier struct {
	EUM          string        `json:"eum"`
	Country      string        `json:"country"`
	Manufacturer string        `json:"manufacturer"`
	Products     []*EUMProduct `json:"products"`
}

func (e EUMIdentifier) ProductName(eid string) string {
	for _, product := range e.Products {
		if match, err := filepath.Match(product.Pattern, eid); err != nil || !match {
			continue
		}
		return product.Name
	}
	return ""
}

type EUMProduct struct {
	Pattern string `json:"pattern"`
	Name    string `json:"name"`
}

var EUMRegistry []*EUMIdentifier

func init() {
	if err := json.Unmarshal(eumRegistryBundle, &EUMRegistry); err != nil {
		panic(err)
	}
}

func GetEUM(eid string) *EUMIdentifier {
	for _, identifier := range EUMRegistry {
		if strings.HasPrefix(eid, identifier.EUM) {
			return identifier
		}
	}
	return nil
}
