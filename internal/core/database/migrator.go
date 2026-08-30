package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RunMigrations applies pending *.up.sql files and records their numeric version.
func RunMigrations(ctx context.Context, db *pgxpool.Pool, directory string) error {
	if _, err := db.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version BIGINT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`); err != nil {
		return err
	}
	if err := baselineExistingSchema(ctx, db); err != nil {
		return err
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	type migration struct {
		version    int64
		name, path string
	}
	items := make([]migration, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}
		prefix := strings.SplitN(entry.Name(), "_", 2)[0]
		version, parseErr := strconv.ParseInt(prefix, 10, 64)
		if parseErr != nil {
			return fmt.Errorf("invalid migration name %s", entry.Name())
		}
		items = append(items, migration{version: version, name: entry.Name(), path: filepath.Join(directory, entry.Name())})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].version < items[j].version })
	for _, item := range items {
		var applied bool
		if err = db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, item.version).Scan(&applied); err != nil {
			return err
		}
		if applied {
			continue
		}
		sql, readErr := os.ReadFile(item.path)
		if readErr != nil {
			return readErr
		}
		tx, txErr := db.Begin(ctx)
		if txErr != nil {
			return txErr
		}
		if _, txErr = tx.Exec(ctx, string(sql)); txErr == nil {
			_, txErr = tx.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES($1)`, item.version)
		}
		if txErr != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply migration %s: %w", item.name, txErr)
		}
		if txErr = tx.Commit(ctx); txErr != nil {
			return txErr
		}
	}
	return nil
}

// Baselines databases whose first migrations were applied manually before the runner existed.
func baselineExistingSchema(ctx context.Context, db *pgxpool.Pool) error {
	_, err := db.Exec(ctx, `
		INSERT INTO schema_migrations(version)
		SELECT 1 WHERE to_regclass('public.users') IS NOT NULL
		  AND to_regclass('public.user_profiles') IS NOT NULL
		  AND to_regclass('public.user_credentials') IS NOT NULL
		  AND to_regclass('public.worker_shift_assignments') IS NOT NULL
		  AND to_regclass('public.meal_claims') IS NOT NULL
		ON CONFLICT DO NOTHING;`)
	return err
}
