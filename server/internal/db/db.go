package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(ctx context.Context, dbURL string) (*pgxpool.Pool, error) {
	if dbURL == "" {
		return nil, fmt.Errorf("database URL must not be empty")
	}
	return pgxpool.New(ctx, dbURL)
}
