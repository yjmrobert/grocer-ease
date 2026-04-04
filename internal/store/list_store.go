package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/yjmrobert/grocer-ease/internal/model"
)

type ListStore struct {
	db *sql.DB
}

func NewListStore(db *sql.DB) *ListStore {
	return &ListStore{db: db}
}

func (s *ListStore) CreateList(name string) (*model.GroceryList, error) {
	list := &model.GroceryList{
		ID:        uuid.New().String(),
		Name:      name,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_, err := s.db.Exec(
		"INSERT INTO grocery_lists (id, name, created_at, updated_at) VALUES (?, ?, ?, ?)",
		list.ID, list.Name, list.CreatedAt, list.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create list: %w", err)
	}
	return list, nil
}

func (s *ListStore) GetList(id string) (*model.GroceryList, error) {
	list := &model.GroceryList{}
	err := s.db.QueryRow(
		"SELECT id, name, created_at, updated_at FROM grocery_lists WHERE id = ?", id,
	).Scan(&list.ID, &list.Name, &list.CreatedAt, &list.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get list: %w", err)
	}
	return list, nil
}

func (s *ListStore) GetAllLists() ([]model.GroceryList, error) {
	rows, err := s.db.Query("SELECT id, name, created_at, updated_at FROM grocery_lists ORDER BY updated_at DESC")
	if err != nil {
		return nil, fmt.Errorf("get all lists: %w", err)
	}
	defer rows.Close()

	var lists []model.GroceryList
	for rows.Next() {
		var l model.GroceryList
		if err := rows.Scan(&l.ID, &l.Name, &l.CreatedAt, &l.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan list: %w", err)
		}
		lists = append(lists, l)
	}
	return lists, nil
}

func (s *ListStore) DeleteList(id string) error {
	_, err := s.db.Exec("DELETE FROM grocery_lists WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete list: %w", err)
	}
	return nil
}

func (s *ListStore) AddItem(listID, name string, quantity float64, unit string) (*model.GroceryItem, error) {
	item := &model.GroceryItem{
		ID:       uuid.New().String(),
		ListID:   listID,
		Name:     name,
		Quantity: quantity,
		Unit:     unit,
	}
	_, err := s.db.Exec(
		"INSERT INTO grocery_items (id, list_id, name, quantity, unit) VALUES (?, ?, ?, ?, ?)",
		item.ID, item.ListID, item.Name, item.Quantity, item.Unit,
	)
	if err != nil {
		return nil, fmt.Errorf("add item: %w", err)
	}
	// Update list timestamp
	s.db.Exec("UPDATE grocery_lists SET updated_at = ? WHERE id = ?", time.Now(), listID)
	return item, nil
}

func (s *ListStore) GetItems(listID string) ([]model.GroceryItem, error) {
	rows, err := s.db.Query(
		"SELECT id, list_id, name, quantity, unit FROM grocery_items WHERE list_id = ? ORDER BY name",
		listID,
	)
	if err != nil {
		return nil, fmt.Errorf("get items: %w", err)
	}
	defer rows.Close()

	var items []model.GroceryItem
	for rows.Next() {
		var item model.GroceryItem
		if err := rows.Scan(&item.ID, &item.ListID, &item.Name, &item.Quantity, &item.Unit); err != nil {
			return nil, fmt.Errorf("scan item: %w", err)
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *ListStore) DeleteItem(id string) error {
	_, err := s.db.Exec("DELETE FROM grocery_items WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete item: %w", err)
	}
	return nil
}

func (s *ListStore) UpdateItem(id, name string, quantity float64, unit string) error {
	_, err := s.db.Exec(
		"UPDATE grocery_items SET name = ?, quantity = ?, unit = ? WHERE id = ?",
		name, quantity, unit, id,
	)
	if err != nil {
		return fmt.Errorf("update item: %w", err)
	}
	return nil
}
