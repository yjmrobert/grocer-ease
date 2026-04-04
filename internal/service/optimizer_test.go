package service

import (
	"math"
	"testing"

	"github.com/yjmrobert/grocer-ease/internal/model"
	"github.com/yjmrobert/grocer-ease/internal/view"
)

func TestOptimizeTripPlan_SingleStore(t *testing.T) {
	grid := view.PriceGridData{
		Items:  []string{"Milk", "Bread", "Eggs"},
		Stores: []string{"StoreA"},
		Prices: map[string]map[string]string{
			"Milk":  {"StoreA": "$4.00"},
			"Bread": {"StoreA": "$3.00"},
			"Eggs":  {"StoreA": "$5.00"},
		},
		Totals: map[string]float64{"StoreA": 12.00},
	}
	items := []model.GroceryItem{
		{Name: "Milk", Quantity: 1, Unit: "each"},
		{Name: "Bread", Quantity: 1, Unit: "each"},
		{Name: "Eggs", Quantity: 1, Unit: "each"},
	}

	plan := OptimizeTripPlan(grid, items, 5.0)

	if len(plan.Trips) != 1 {
		t.Fatalf("expected 1 trip, got %d", len(plan.Trips))
	}
	if plan.Trips[0].Store != "StoreA" {
		t.Errorf("expected StoreA, got %s", plan.Trips[0].Store)
	}
	if math.Abs(plan.TotalCost-12.00) > 0.01 {
		t.Errorf("expected total $12.00, got $%.2f", plan.TotalCost)
	}
}

func TestOptimizeTripPlan_SplitsAcrossStores(t *testing.T) {
	grid := view.PriceGridData{
		Items:  []string{"Milk", "Bread", "Eggs", "Chicken", "Rice"},
		Stores: []string{"StoreA", "StoreB"},
		Prices: map[string]map[string]string{
			"Milk":    {"StoreA": "$4.00", "StoreB": "$3.00"},
			"Bread":   {"StoreA": "$3.00", "StoreB": "$4.00"},
			"Eggs":    {"StoreA": "$5.00", "StoreB": "$3.00"},
			"Chicken": {"StoreA": "$12.00", "StoreB": "$8.00"},
			"Rice":    {"StoreA": "$6.00", "StoreB": "$9.00"},
		},
		Totals: map[string]float64{"StoreA": 30.00, "StoreB": 27.00},
	}
	items := []model.GroceryItem{
		{Name: "Milk", Quantity: 1, Unit: "each"},
		{Name: "Bread", Quantity: 1, Unit: "each"},
		{Name: "Eggs", Quantity: 1, Unit: "each"},
		{Name: "Chicken", Quantity: 1, Unit: "each"},
		{Name: "Rice", Quantity: 1, Unit: "each"},
	}

	// With a low trip penalty ($1), splitting is worthwhile
	plan := OptimizeTripPlan(grid, items, 1.0)

	if len(plan.Trips) != 2 {
		t.Fatalf("expected 2 trips, got %d", len(plan.Trips))
	}
	// Optimal: Bread+Rice at StoreA ($9), Milk+Eggs+Chicken at StoreB ($14) = $23
	if math.Abs(plan.TotalCost-23.00) > 0.01 {
		t.Errorf("expected total $23.00, got $%.2f", plan.TotalCost)
	}
}

func TestOptimizeTripPlan_MergesSmallTrips(t *testing.T) {
	grid := view.PriceGridData{
		Items:  []string{"Milk", "Bread"},
		Stores: []string{"StoreA", "StoreB"},
		Prices: map[string]map[string]string{
			"Milk":  {"StoreA": "$4.00", "StoreB": "$3.50"},
			"Bread": {"StoreA": "$3.00", "StoreB": "$3.50"},
		},
		Totals: map[string]float64{"StoreA": 7.00, "StoreB": 7.00},
	}
	items := []model.GroceryItem{
		{Name: "Milk", Quantity: 1, Unit: "each"},
		{Name: "Bread", Quantity: 1, Unit: "each"},
	}

	// With a high trip penalty ($10), it's not worth splitting
	plan := OptimizeTripPlan(grid, items, 10.0)

	if len(plan.Trips) != 1 {
		t.Fatalf("expected 1 trip (merged), got %d", len(plan.Trips))
	}
}

func TestOptimizeTripPlan_EmptyGrid(t *testing.T) {
	grid := view.PriceGridData{
		Items:  []string{},
		Stores: []string{},
		Prices: map[string]map[string]string{},
		Totals: map[string]float64{},
	}

	plan := OptimizeTripPlan(grid, nil, 5.0)

	if len(plan.Trips) != 0 {
		t.Fatalf("expected 0 trips for empty grid, got %d", len(plan.Trips))
	}
}

func TestParseGridPrice(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"$4.99", 4.99},
		{"$12.00", 12.00},
		{"$0.99", 0.99},
		{"Not found", 0},
		{"", 0},
	}
	for _, tt := range tests {
		got := parseGridPrice(tt.input)
		if math.Abs(got-tt.expected) > 0.01 {
			t.Errorf("parseGridPrice(%q) = %f, want %f", tt.input, got, tt.expected)
		}
	}
}
