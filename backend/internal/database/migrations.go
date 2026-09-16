package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type migration struct {
	Version string
	Path    string
}

func Migrate(db *sql.DB, migrationsPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`); err != nil {
		return fmt.Errorf("ensure schema_migrations table: %w", err)
	}

	migrations, err := listMigrations(migrationsPath)
	if err != nil {
		return err
	}

	for _, item := range migrations {
		applied, err := migrationApplied(ctx, db, item.Version)
		if err != nil {
			return err
		}

		if applied {
			continue
		}

		if err := applyMigration(ctx, db, item); err != nil {
			return err
		}
	}

	return nil
}

func listMigrations(migrationsPath string) ([]migration, error) {
	entries, err := os.ReadDir(migrationsPath)
	if err != nil {
		return nil, fmt.Errorf("read migrations directory %q: %w", migrationsPath, err)
	}

	migrations := make([]migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}

		version := strings.TrimSuffix(entry.Name(), ".up.sql")
		migrations = append(migrations, migration{
			Version: version,
			Path:    filepath.Join(migrationsPath, entry.Name()),
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

func migrationApplied(ctx context.Context, db *sql.DB, version string) (bool, error) {
	var exists bool
	if err := db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM schema_migrations WHERE version = $1
		)
	`, version).Scan(&exists); err != nil {
		return false, fmt.Errorf("check migration %s: %w", version, err)
	}

	return exists, nil
}

func applyMigration(ctx context.Context, db *sql.DB, item migration) error {
	content, err := os.ReadFile(item.Path)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", item.Version, err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", item.Version, err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, string(content)); err != nil {
		return fmt.Errorf("apply migration %s: %w", item.Version, err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO schema_migrations (version) VALUES ($1)
	`, item.Version); err != nil {
		return fmt.Errorf("record migration %s: %w", item.Version, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", item.Version, err)
	}

	return nil
}
