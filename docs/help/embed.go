// Package helpdocs embeds the Help Center pages (docs/help/{id,en}) into the backend binary.
// Conventions: docs/help/README.md. Served by internal/help.
package helpdocs

import "embed"

// FS holds <locale>/<slug>/index.md and <locale>/<slug>/<sub>.md for every supported locale, and the shared images
// in assets/ (PNG/SVG, referenced as /api/v1/help/assets/<file>).
//
//go:embed id en assets
var FS embed.FS
