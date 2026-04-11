//go:build scenario

package scenarios

import (
	"github.com/handgemacht-ai/scenarigo"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewTestRegistry(pool *pgxpool.Pool, baseURL string) *scenarigo.Registry {
	return scenarigo.NewRegistry(Fixtures(pool, baseURL)...)
}
