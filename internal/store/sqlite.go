package store

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

func NewDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	// SQLite supports only one writer; limit connections to prevent "database is locked" errors.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)
	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migrate db: %w", err)
	}
	return db, nil
}

func migrate(db *sql.DB) error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS grocery_lists (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS grocery_items (
			id TEXT PRIMARY KEY,
			list_id TEXT NOT NULL,
			name TEXT NOT NULL,
			quantity REAL NOT NULL DEFAULT 1,
			unit TEXT NOT NULL DEFAULT 'each',
			FOREIGN KEY (list_id) REFERENCES grocery_lists(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS price_cache (
			id TEXT PRIMARY KEY,
			item_query TEXT NOT NULL,
			store TEXT NOT NULL,
			product_name TEXT NOT NULL,
			price REAL NOT NULL,
			unit TEXT NOT NULL,
			fetched_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_price_cache_query_store ON price_cache(item_query, store)`,
		`CREATE INDEX IF NOT EXISTS idx_grocery_items_list_id ON grocery_items(list_id)`,
		`CREATE TABLE IF NOT EXISTS app_settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
	}
	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			return fmt.Errorf("exec migration: %w", err)
		}
	}
	return nil
}
