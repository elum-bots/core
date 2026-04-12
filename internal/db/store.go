package db

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/elum-bots/core/internal/db/sqlc"
	_ "github.com/mattn/go-sqlite3"
)

//go:embed schema/*.sql
var schemaFS embed.FS

const migrationsTableName = "schema_migrations"

type Store struct {
	db                *sql.DB
	Queries           *sqlc.Queries
	Users             *UserRepository
	Mandatory         *MandatoryRepository
	Posts             *PostRepository
	Tasks             *TaskRepository
	Payments          *PaymentRepository
	IntegrationTokens *IntegrationTokenRepository
	Track             *TrackRepository
	Broadcasts        *BroadcastRepository
	Metrics           *MetricsRepository
	Stats             *StatsRepository
}

func Open(ctx context.Context, sqlitePath string) (*Store, error) {
	if strings.TrimSpace(sqlitePath) == "" {
		return nil, errors.New("sqlite path is empty")
	}

	if err := os.MkdirAll(filepath.Dir(sqlitePath), 0o755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", sqlitePath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := configureSQLite(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	if err := applyMigrations(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}

	q := sqlc.New(db)
	store := &Store{
		db:      db,
		Queries: q,
	}
	store.Users = NewUserRepository(store, q)
	store.Mandatory = NewMandatoryRepository(q)
	store.Posts = NewPostRepository(q)
	store.Tasks = NewTaskRepository(store, q)
	store.Payments = NewPaymentRepository(q)
	store.IntegrationTokens = NewIntegrationTokenRepository(q)
	store.Track = NewTrackRepository(store, q)
	store.Broadcasts = NewBroadcastRepository(store, q)
	store.Metrics = NewMetricsRepository(q)
	store.Stats = NewStatsRepository(q)
	return store, nil
}

func (s *Store) DB() *sql.DB {
	if s == nil {
		return nil
	}
	return s.db
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) PingContext(ctx context.Context) error {
	if s == nil || s.db == nil {
		return errors.New("db is not initialized")
	}
	return s.db.PingContext(ctx)
}

func (s *Store) WithTx(ctx context.Context, fn func(*sqlc.Queries) error) error {
	if s == nil || s.db == nil {
		return errors.New("db is not initialized")
	}
	if fn == nil {
		return errors.New("tx func is nil")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	q := s.Queries.WithTx(tx)
	if err := fn(q); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func configureSQLite(ctx context.Context, db *sql.DB) error {
	pragmas := []string{
		"PRAGMA foreign_keys = ON;",
		"PRAGMA journal_mode = WAL;",
		"PRAGMA busy_timeout = 5000;",
	}
	for _, stmt := range pragmas {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("sqlite pragma %q: %w", stmt, err)
		}
	}
	return nil
}

func applyMigrations(ctx context.Context, db *sql.DB) error {
	entries, err := fs.ReadDir(schemaFS, "schema")
	if err != nil {
		return err
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	hasMigrations, err := hasTable(ctx, db, migrationsTableName)
	if err != nil {
		return err
	}
	if !hasMigrations {
		hasAppTables, err := hasNonSystemTables(ctx, db)
		if err != nil {
			return err
		}
		if hasAppTables {
			return errors.New("legacy sqlite database without schema_migrations is not supported; recreate the database")
		}
		if err := createMigrationsTable(ctx, db); err != nil {
			return err
		}
	}

	applied, err := loadAppliedMigrations(ctx, db)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if applied[entry.Name()] {
			continue
		}
		body, err := schemaFS.ReadFile("schema/" + entry.Name())
		if err != nil {
			return err
		}
		sqlText := strings.TrimSpace(string(body))
		if sqlText == "" {
			continue
		}

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, sqlText); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", entry.Name(), err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (name, applied_at) VALUES (?, ?)`, entry.Name(), nowUTC()); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %s: %w", entry.Name(), err)
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}

func createMigrationsTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
  name TEXT PRIMARY KEY,
  applied_at TEXT NOT NULL
);`)
	if err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	return nil
}

func loadAppliedMigrations(ctx context.Context, db *sql.DB) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT name FROM schema_migrations ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out[name] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func hasTable(ctx context.Context, db *sql.DB, tableName string) (bool, error) {
	var count int
	if err := db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`,
		strings.TrimSpace(tableName),
	).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func hasNonSystemTables(ctx context.Context, db *sql.DB) (bool, error) {
	var count int
	if err := db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`,
	).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}
