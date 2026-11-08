package main

import (
	"encoding/pem"
	"strings"

	"github.com/egoist/mygo/ui"

	"EasyLPAC/internal/registry"
	"EasyLPAC/internal/x509info"
	"EasyLPAC/locales"
)

// openCertificate shows a CI's certificate in a dialog; keyID is the CI's
// key ID as the card lists it, in full.
func (g *gui) openCertificate(ci *registry.Issuer, keyID string) {
	info, err := x509info.Parse(ci.Certificate)
	if err != nil {
		g.toast = err.Error()
		return
	}
	g.certCI = ci
	g.certKeyID = strings.ToLower(keyID)
	g.certTree, _ = parseJSONTree(info.JSON())
	g.certTreeOpen = map[string]bool{"$.issuer": true, "$.validity": true, "$.subject": true, "$.subjectPublicKeyInfo": true}
	g.certOpen = true
}

// certificateDialog shows the certificate opened, decoded.
func (g *gui) certificateDialog(c *ui.Context) {
	if g.certCI == nil {
		return
	}
	t := c.Theme()
	ci := g.certCI
	ui.Modal(c, &g.certOpen, func() {
		ui.Column(c).Width(640).Gap(12).Children(func() {
			ui.Row(c).Gap(8).Children(func() {
				ui.Text(c, ci.Name).FontSize(16).Bold().MinWidth(0)
				if cat := categoryLabel(ci.Category); cat != "" {
					ui.Badge(c, cat)
				}
			})
			ui.Text(c, locales.CertKeyId(g.certKeyID)).FontSize(12).Font("monospace").TextColor(t.TextMuted).Selectable()
			if ci.IssuedByCI {
				ui.Text(c, locales.CertIssuedByCi()).FontSize(12).TextColor(t.Warning)
			}
			ui.Scroll(c).MaxHeight(440).Padding(6).Radius(12).Background(t.Surface).Border(1, t.Border).Children(func() {
				if g.certTree != nil {
					g.jsonTree(c, g.certTree, g.certTreeOpen, nil, nil)
				}
			})
			ui.Row(c).Gap(8).Children(func() {
				if ui.Button(c, locales.CardExpandAll()).Radius(14).Clicked() {
					setTreeOpen(g.certTree, g.certTreeOpen, true)
				}
				if ui.Button(c, locales.CardCollapseAll()).Radius(14).Clicked() {
					setTreeOpen(g.certTree, g.certTreeOpen, false)
				}
				ui.Spacer(c)
				if ui.Button(c, locales.CertCopyPem()).Radius(14).Clicked() {
					g.clip.WriteText(string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ci.Certificate})))
					c.Toast(locales.CertPemCopied())
				}
				if ui.PrimaryButton(c, locales.CommonClose()).Clicked() {
					g.certOpen = false
				}
			})
		})
	})
}

// categoryLabel says what kind of CI it is.
func categoryLabel(cat registry.Category) string {
	switch cat {
	case registry.CategoryLive:
		return locales.CiCategoryLive()
	case registry.CategoryTest:
		return locales.CiCategoryTest()
	case registry.CategoryChina:
		return locales.CiCategoryChina()
	}
	return ""
}
