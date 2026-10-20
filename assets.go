package app

import "embed"

// Files contains the static client and the built-in demo dictionaries.
//
//go:embed public dictionaries
var Files embed.FS
