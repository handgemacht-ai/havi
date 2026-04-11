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

func Migrate(ctx context.Context, pool *pgxpool.Pool, migrationsDir string) error {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		log.Printf("WARN: migrations directory %q not readable: %v — skipping migrations", migrationsDir, err)
		return nil
	}

	var sqlFiles []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".sql" {
			sqlFiles = append(sqlFiles, e.Name())
		}
	}

	if len(sqlFiles) == 0 {
		log.Printf("WARN: no SQL files found in %q — skipping migrations", migrationsDir)
		return nil
	}

	sort.Strings(sqlFiles)

	for _, name := range sqlFiles {
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
