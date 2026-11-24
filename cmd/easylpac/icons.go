package main

import (
	"github.com/egoist/mygo/ui"

	"EasyLPAC/internal/settings"
)

// icon builds a 24×24 line icon drawn in the current text color.
func icon(shapes string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">` + shapes + `</svg>`))
}

var (
	iconSIM      = icon(`<path d="M7 3h7l5 5v11a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2z"/><rect x="8.5" y="11" width="7" height="6.5" rx="1"/><path d="M12 11v6.5M8.5 14.25h7"/>`)
	iconBell     = icon(`<path d="M6 16V11a6 6 0 0 1 12 0v5l1.5 2h-15z"/><path d="M10 20.5a2.2 2.2 0 0 0 4 0"/>`)
	iconChip     = icon(`<rect x="6" y="6" width="12" height="12" rx="2"/><rect x="9.5" y="9.5" width="5" height="5" rx=".8"/><path d="M9 3v3M15 3v3M9 18v3M15 18v3M3 9h3M3 15h3M18 9h3M18 15h3"/>`)
	iconSettings = icon(`<path d="M18.65 10.16L20.88 10.51L20.88 13.49L18.65 13.84A6.9 6.9 0 0 1 18.01 15.40L19.33 17.23L17.23 19.33L15.40 18.01A6.9 6.9 0 0 1 13.84 18.65L13.49 20.88L10.51 20.88L10.16 18.65A6.9 6.9 0 0 1 8.60 18.01L6.77 19.33L4.67 17.23L5.99 15.40A6.9 6.9 0 0 1 5.35 13.84L3.12 13.49L3.12 10.51L5.35 10.16A6.9 6.9 0 0 1 5.99 8.60L4.67 6.77L6.77 4.67L8.60 5.99A6.9 6.9 0 0 1 10.16 5.35L10.51 3.12L13.49 3.12L13.84 5.35A6.9 6.9 0 0 1 15.40 5.99L17.23 4.67L19.33 6.77L18.01 8.60A6.9 6.9 0 0 1 18.65 10.16Z"/><circle cx="12" cy="12" r="2.8"/>`)
	iconLog      = icon(`<rect x="4" y="3" width="16" height="18" rx="2"/><path d="M8 8h8M8 12h8M8 16h5"/>`)
	iconRefresh  = icon(`<path d="M20 11a8 8 0 0 0-14.3-4.6L4 8"/><path d="M4 4v4h4"/><path d="M4 13a8 8 0 0 0 14.3 4.6L20 16"/><path d="M20 20v-4h-4"/>`)
	iconPlus     = icon(`<path d="M12 5v14M5 12h14"/>`)
	iconCopy     = icon(`<rect x="8" y="8" width="12" height="12" rx="2"/><path d="M16 8V6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h2"/>`)
	iconPaste    = icon(`<rect x="5" y="4" width="14" height="17" rx="2"/><path d="M9 4V3h6v1M9 11h6M9 15h4"/>`)
	iconCamera   = icon(`<path d="M4 8h3l2-3h6l2 3h3a1 1 0 0 1 1 1v10a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V9a1 1 0 0 1 1-1z"/><circle cx="12" cy="13.5" r="3.5"/>`)
	iconImage    = icon(`<rect x="3" y="4" width="18" height="16" rx="2"/><circle cx="9" cy="10" r="1.8"/><path d="m4 18 5-5 4 4 3-3 4 4"/>`)
	iconSend     = icon(`<path d="M21 3 10 14"/><path d="M21 3 14.5 21 10 14 3 9.5z"/>`)
	iconTrash    = icon(`<path d="M4 7h16M10 11v6M14 11v6M6 7l1 13h10l1-13M9 7V4h6v3"/>`)
	iconCheck    = icon(`<path d="M5 12.5 10 17 19 7"/>`)
	iconDots     = icon(`<circle cx="6" cy="12" r="1"/><circle cx="12" cy="12" r="1"/><circle cx="18" cy="12" r="1"/>`)
	iconExport   = icon(`<path d="M12 3v12M7 8l5-5 5 5"/><path d="M5 14v5a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2v-5"/>`)
	iconAlert    = icon(`<path d="M12 3 2.5 20h19z"/><path d="M12 10v4.5M12 17.5v.01"/>`)
	iconReader   = icon(`<rect x="3" y="8" width="18" height="11" rx="2"/><path d="M7 8V5h10v3"/><path d="M7 14h4"/>`)
	iconModem    = icon(`<rect x="3" y="12" width="18" height="8" rx="2"/><path d="M16 12V8.5"/><path d="M13.5 6.5a3.5 3.5 0 0 1 5 0M11.5 4.5a6.5 6.5 0 0 1 9 0"/><path d="M7 16h.01M10 16h.01"/>`)
	iconChevron  = icon(`<path d="m9 6 6 6-6 6"/>`)
	iconExternal = icon(`<path d="M14 4h6v6M20 4l-9 9"/><path d="M18 14v5a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V7a1 1 0 0 1 1-1h5"/>`)
	iconInfo     = icon(`<circle cx="12" cy="12" r="9"/><path d="M12 11v5.5M12 7.5v.01"/>`)
)

// deviceIcon is the icon of the devices of backend: a modem for AT, a
// card reader for PC/SC.
func deviceIcon(backend string) *ui.SVG {
	if backend == settings.BackendAT {
		return iconModem
	}
	return iconReader
}
