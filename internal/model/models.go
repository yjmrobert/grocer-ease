package model

import "time"

type GroceryList struct {
	ID        string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type GroceryItem struct {
	ID       string
	ListID   string
	Name     string
	Quantity float64
	Unit     string
}

type PriceCache struct {
	ID          string
	ItemQuery   string
	Store       string
	ProductName string
	Price       float64
	Unit        string
	FetchedAt   time.Time
}

type PriceResult struct {
	Store       string
	ProductName string
	Price       float64
	Unit        string
	ImageURL    string
	URL         string
	Confidence  string // "exact" or "partial"
}

type TripPlan struct {
	Trips     []Trip
	TotalCost float64
	Savings   float64
}

type Trip struct {
	Store    string
	Items    []GroceryItem
	Subtotal float64
}
