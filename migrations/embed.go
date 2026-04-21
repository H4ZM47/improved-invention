package migrations

import "embed"

// Files contains embedded SQL migration assets.
//
//go:embed *.sql
var Files embed.FS
