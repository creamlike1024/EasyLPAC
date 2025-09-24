//go:generate curl -o eum-registry.json https://euicc-manual.osmocom.org/docs/pki/eum/manifest-v2.json
package main

import (
	_ "embed"
	"encoding/json"
	"strconv"
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

func (e *EUMIdentifier) ProductName(eid string) string {
	for _, product := range e.Products {
		if product.Test(eid) {
			return product.Name
		}
	}
	return ""
}

type EUMProduct struct {
	Prefix string      `json:"prefix"`
	Name   string      `json:"name"`
	Range  [][2]uint64 `json:"in-range"`
}

func (p *EUMProduct) Test(eid string) bool {
	if len(eid) != 32 || !strings.HasPrefix(eid, p.Prefix) {
		return false
	} else if len(p.Range) == 0 {
		return true
	}
	parsed, err := strconv.ParseUint(eid[len(p.Prefix):30], 10, 64)
	if err != nil {
		return false
	}
	var begin, end uint64
	for _, assignedRange := range p.Range {
		begin, end = assignedRange[0], assignedRange[1]
		if parsed >= begin && parsed <= end {
			return true
		}
	}
	return false
}

var EUMRegistry []*EUMIdentifier

func InitEumRegistry() {
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
