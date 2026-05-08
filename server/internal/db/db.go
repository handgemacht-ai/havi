package db

import "strings"

type Backend string

const (
	BackendSQLite   Backend = "sqlite"
	BackendPostgres Backend = "postgres"
)

func DetectBackend(url string) Backend {
	if strings.HasPrefix(url, "postgres://") || strings.HasPrefix(url, "postgresql://") {
		return BackendPostgres
	}
	return BackendSQLite
}
