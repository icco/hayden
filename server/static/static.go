// Package static embeds the server's static assets and seed config.
package static

import "embed"

// Content holds our embedded static content.
//
//go:embed *
var Content embed.FS
