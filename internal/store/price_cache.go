package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/yjmrobert/grocer-ease/internal/model"
)

type PriceCacheStore struct {
	db  *sql.DB
	ttl time.Duration
}

func NewPriceCacheStore(db *sql.DB, ttl time.Duration) *PriceCacheStore {
	return &PriceCacheStore{db: db, ttl: ttl}
}

func (s *PriceCacheStore) Get(query, store string) (*model.PriceCache, error) {
	entry := &model.PriceCache{}
	err := s.db.QueryRow(
		`SELECT id, item_query, store, product_name, price, unit, fetched_at
		 FROM price_cache
		 WHERE item_query = ? AND store = ? AND fetched_at > ?`,
		query, store, time.Now().Add(-s.ttl),
	).Scan(&entry.ID, &entry.ItemQuery, &entry.Store, &entry.ProductName, &entry.Price, &entry.Unit, &entry.FetchedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get price cache: %w", err)
	}
	return entry, nil
}

// GetProductNames returns distinct cached product names matching a prefix, for autocomplete.
func (s *PriceCacheStore) GetProductNames(prefix string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(
		`SELECT DISTINCT product_name FROM price_cache
		 WHERE product_name LIKE ? COLLATE NOCASE
		 ORDER BY product_name
		 LIMIT ?`,
		prefix+"%", limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get product names: %w", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

func (s *PriceCacheStore) Set(query, storeName, productName string, price float64, unit string) error {
	// Delete any existing entry for this query+store
	s.db.Exec("DELETE FROM price_cache WHERE item_query = ? AND store = ?", query, storeName)

	_, err := s.db.Exec(
		`INSERT INTO price_cache (id, item_query, store, product_name, price, unit, fetched_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), query, storeName, productName, price, unit, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("set price cache: %w", err)
	}
	return nil
}
