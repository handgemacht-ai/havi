package db

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
)

func ConnectPostgres(ctx context.Context, dbURL string) (*pgxpool.Pool, error) {
	if dbURL == "" {
		return nil, fmt.Errorf("database URL must not be empty")
	}
	return pgxpool.New(ctx, dbURL)
}

func MigratePostgres(ctx context.Context, pool *pgxpool.Pool, migrationsDir string) error {
	files, err := readMigrationFiles(migrationsDir)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return nil
	}
	for _, name := range files {
		data, err := os.ReadFile(filepath.Join(migrationsDir, name))
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, string(data)); err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
	}
	return nil
}

func readMigrationFiles(migrationsDir string) ([]string, error) {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		log.Printf("WARN: migrations directory %q not readable: %v — skipping migrations", migrationsDir, err)
		return nil, nil
	}
	var sqlFiles []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".sql" {
			sqlFiles = append(sqlFiles, e.Name())
		}
	}
	if len(sqlFiles) == 0 {
		log.Printf("WARN: no SQL files found in %q — skipping migrations", migrationsDir)
	}
	sort.Strings(sqlFiles)
	return sqlFiles, nil
}
