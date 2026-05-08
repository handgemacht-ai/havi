//go:build scenario

package scenarios

import (
	"database/sql"

	"github.com/handgemacht-ai/scenarigo"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Backend struct {
	Postgres *pgxpool.Pool
	SQLite   *sql.DB
}

func NewTestRegistry(b Backend, baseURL string) *scenarigo.Registry {
	return scenarigo.NewRegistry(Fixtures(b, baseURL)...)
}
