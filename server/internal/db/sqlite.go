package db

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
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

func MigrateSQLite(ctx context.Context, db *sql.DB, fsys fs.FS) error {
	files, err := readMigrationFiles(fsys)
	if err != nil {
		return err
	}
	for _, name := range files {
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		if _, err := db.ExecContext(ctx, string(data)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
	}
	return nil
}
