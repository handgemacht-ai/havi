// Package migrations exposes the embedded SQL migration files.
package migrations

import "embed"

//go:embed sqlite/*.sql
var SQLite embed.FS

//go:embed postgres/*.sql
var Postgres embed.FS
