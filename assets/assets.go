// Package assets holds files embedded in the app.
package assets

import _ "embed"

// Icon is the app icon, 256×256 PNG.
//
//go:embed icon256.png
var Icon []byte
