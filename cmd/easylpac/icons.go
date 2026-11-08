package main

import "github.com/egoist/mygo/ui"

// icon builds a 24×24 line icon drawn in the current text color.
func icon(shapes string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">` + shapes + `</svg>`))
}

var (
	iconSIM      = icon(`<path d="M7 3h7l5 5v11a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2z"/><rect x="8.5" y="11" width="7" height="6.5" rx="1"/><path d="M12 11v6.5M8.5 14.25h7"/>`)
	iconBell     = icon(`<path d="M6 16V11a6 6 0 0 1 12 0v5l1.5 2h-15z"/><path d="M10 20.5a2.2 2.2 0 0 0 4 0"/>`)
	iconChip     = icon(`<rect x="6" y="6" width="12" height="12" rx="2"/><rect x="9.5" y="9.5" width="5" height="5" rx=".8"/><path d="M9 3v3M15 3v3M9 18v3M15 18v3M3 9h3M3 15h3M18 9h3M18 15h3"/>`)
	iconSettings = icon(`<circle cx="12" cy="12" r="3"/><path d="M12 2.5v3M12 18.5v3M2.5 12h3M18.5 12h3M5.3 5.3l2.1 2.1M16.6 16.6l2.1 2.1M5.3 18.7l2.1-2.1M16.6 7.4l2.1-2.1"/>`)
	iconLog      = icon(`<rect x="4" y="3" width="16" height="18" rx="2"/><path d="M8 8h8M8 12h8M8 16h5"/>`)
	iconRefresh  = icon(`<path d="M20 11a8 8 0 0 0-14.3-4.6L4 8"/><path d="M4 4v4h4"/><path d="M4 13a8 8 0 0 0 14.3 4.6L20 16"/><path d="M20 20v-4h-4"/>`)
	iconPlus     = icon(`<path d="M12 5v14M5 12h14"/>`)
	iconCopy     = icon(`<rect x="8" y="8" width="12" height="12" rx="2"/><path d="M16 8V6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h2"/>`)
	iconPaste    = icon(`<rect x="5" y="4" width="14" height="17" rx="2"/><path d="M9 4V3h6v1M9 11h6M9 15h4"/>`)
	iconImage    = icon(`<rect x="3" y="4" width="18" height="16" rx="2"/><circle cx="9" cy="10" r="1.8"/><path d="m4 18 5-5 4 4 3-3 4 4"/>`)
	iconSend     = icon(`<path d="M21 3 10 14"/><path d="M21 3 14.5 21 10 14 3 9.5z"/>`)
	iconTrash    = icon(`<path d="M4 7h16M10 11v6M14 11v6M6 7l1 13h10l1-13M9 7V4h6v3"/>`)
	iconCheck    = icon(`<path d="M5 12.5 10 17 19 7"/>`)
	iconDots     = icon(`<circle cx="6" cy="12" r="1"/><circle cx="12" cy="12" r="1"/><circle cx="18" cy="12" r="1"/>`)
	iconExport   = icon(`<path d="M12 3v12M7 8l5-5 5 5"/><path d="M5 14v5a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2v-5"/>`)
	iconAlert    = icon(`<path d="M12 3 2.5 20h19z"/><path d="M12 10v4.5M12 17.5v.01"/>`)
	iconReader   = icon(`<rect x="3" y="8" width="18" height="11" rx="2"/><path d="M7 8V5h10v3"/><path d="M7 14h4"/>`)
	iconChevron  = icon(`<path d="m9 6 6 6-6 6"/>`)
	iconExternal = icon(`<path d="M14 4h6v6M20 4l-9 9"/><path d="M18 14v5a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V7a1 1 0 0 1 1-1h5"/>`)
	iconInfo     = icon(`<circle cx="12" cy="12" r="9"/><path d="M12 11v5.5M12 7.5v.01"/>`)
)
