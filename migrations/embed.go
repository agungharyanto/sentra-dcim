// Package migrations embeds the SQL migration files (golang-migrate format).
package migrations

import "embed"

// FS contains all *.up.sql and *.down.sql files.
//
//go:embed *.sql
var FS embed.FS
