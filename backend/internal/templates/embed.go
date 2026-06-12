package templates

import "embed"

// Files exposes the embedded HTML template files for server-side rendering.
//
//go:embed *.html
var Files embed.FS
