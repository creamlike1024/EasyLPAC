package main

import (
	"encoding/pem"
	"strings"

	"github.com/egoist/mygo/ui"

	"EasyLPAC/internal/registry"
	"EasyLPAC/internal/x509info"
	"EasyLPAC/locales"
)

// openCertificate shows a CI's certificate: in a dialog on desktops, a
// page of its own on phones. keyID is the CI's key ID as the card lists
// it, in full.
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
	g.showCertificate()
}

// certificateDialog shows the certificate opened, decoded, on desktops.
func (g *gui) certificateDialog(c *ui.Context) {
	if g.certCI == nil {
		return
	}
	ui.Modal(c, &g.certOpen, func() {
		ui.Column(c).Width(640).Gap(12).Children(func() {
			g.certificateHead(c)
			g.certificateTree(ui.Scroll(c).MaxHeight(440), c)
			ui.Row(c).Gap(8).Children(func() {
				g.certificateTreeButtons(c)
				ui.Spacer(c)
				g.copyPEMButton(c)
				if ui.PrimaryButton(c, locales.CommonClose()).Clicked() {
					g.certOpen = false
				}
			})
		})
	})
}

// certificatePage is the certificate opened, on phones: the page
// scrolls, the tree in it.
func (g *gui) certificatePage(c *ui.Context) {
	if g.certCI == nil {
		return
	}
	g.certificateHead(c)
	ui.Row(c).Gap(8).Wrap().Children(func() {
		g.certificateTreeButtons(c)
		g.copyPEMButton(c)
	})
	// The page scrolls: the tree takes the height it needs.
	g.certificateTree(ui.Column(c), c)
}

// certificateHead names the certificate's CI, and its key ID.
func (g *gui) certificateHead(c *ui.Context) {
	t := c.Theme()
	ci := g.certCI
	ui.Column(c).Gap(12).Children(func() {
		ui.Row(c).Gap(8).Wrap().Children(func() {
			ui.Text(c, ci.Name).FontSize(16).Bold().MinWidth(0)
			if cat := categoryLabel(ci.Category); cat != "" {
				ui.Badge(c, cat)
			}
		})
		ui.Text(c, locales.CertKeyId(g.certKeyID)).FontSize(12).Font("monospace").TextColor(t.TextMuted).Selectable()
		if ci.IssuedByCI {
			ui.Text(c, locales.CertIssuedByCi()).FontSize(12).TextColor(t.Warning)
		}
	})
}

// certificateTree is the certificate decoded, as a tree of its fields,
// in box.
func (g *gui) certificateTree(box ui.Element, c *ui.Context) {
	t := c.Theme()
	box.Padding(6).Radius(12).Background(t.Surface).Border(1, t.Border).Children(func() {
		if g.certTree != nil {
			g.jsonTree(c, g.certTree, g.certTreeOpen, nil, nil)
		}
	})
}

// certificateTreeButtons open and close all the tree's nodes.
func (g *gui) certificateTreeButtons(c *ui.Context) {
	if ui.Button(c, locales.CardExpandAll()).Radius(14).Clicked() {
		setTreeOpen(g.certTree, g.certTreeOpen, true)
	}
	if ui.Button(c, locales.CardCollapseAll()).Radius(14).Clicked() {
		setTreeOpen(g.certTree, g.certTreeOpen, false)
	}
}

// copyPEMButton copies the certificate, in PEM.
func (g *gui) copyPEMButton(c *ui.Context) {
	if ui.Button(c, locales.CertCopyPem()).Radius(14).Clicked() {
		g.clip.WriteText(string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: g.certCI.Certificate})))
		c.Toast(locales.CertPemCopied())
	}
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
