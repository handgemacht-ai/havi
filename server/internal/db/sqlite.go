package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

func ConnectSQLite(dbURL string) (*sql.DB, error) {
	path := sqlitePath(dbURL)
	if path == "" {
		return nil, fmt.Errorf("sqlite path must not be empty")
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create sqlite parent dir: %w", err)
	}

	dsn := "file:" + path + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func sqlitePath(dbURL string) string {
	if dbURL == "" {
		return ""
	}
	if strings.HasPrefix(dbURL, "sqlite://") {
		raw := strings.TrimPrefix(dbURL, "sqlite://")
		if u, err := url.Parse("sqlite://" + raw); err == nil && u.Path != "" {
			return u.Path
		}
		return raw
	}
	return dbURL
}

func MigrateSQLite(ctx context.Context, db *sql.DB, migrationsDir string) error {
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
		if _, err := db.ExecContext(ctx, string(data)); err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
	}
	return nil
}
