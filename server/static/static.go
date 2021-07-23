package static

import "embed"

// Files holds our static content.
//go:embed *
var Content embed.FS
