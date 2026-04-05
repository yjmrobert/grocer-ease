package service

import (
	"math"
	"sort"

	"github.com/yjmrobert/grocer-ease/internal/model"
	"github.com/yjmrobert/grocer-ease/internal/view"
)

const defaultTripPenalty = 5.0 // $5 penalty per additional store visit

// OptimizeTripPlan takes a price grid and builds an optimized trip plan.
// It uses a greedy algorithm: assign each item to its cheapest store,
// then merge low-savings trips into other stores when the savings don't
// justify an extra trip (based on tripPenalty).
func OptimizeTripPlan(grid view.PriceGridData, items []model.GroceryItem, tripPenalty float64) model.TripPlan {
	if tripPenalty <= 0 {
		tripPenalty = defaultTripPenalty
	}

	// Build a numeric price matrix: itemName -> storeName -> price (0 = not found)
	priceMatrix := make(map[string]map[string]float64)
	for _, itemName := range grid.Items {
		priceMatrix[itemName] = make(map[string]float64)
		for _, storeName := range grid.Stores {
			priceStr := grid.Prices[itemName][storeName]
			if priceStr != "Not found" {
				priceMatrix[itemName][storeName] = model.ParsePriceString(priceStr)
			}
		}
	}

	// Step 1: Find the cheapest single-store total (baseline)
	bestSingleStore := ""
	bestSingleTotal := math.MaxFloat64
	for _, storeName := range grid.Stores {
		total := grid.Totals[storeName]
		if total > 0 && total < bestSingleTotal {
			bestSingleTotal = total
			bestSingleStore = storeName
		}
	}
	if bestSingleStore == "" {
		// No prices found at all
		return model.TripPlan{}
	}

	// Step 2: Assign each item to its cheapest store
	itemAssignment := make(map[string]string) // itemName -> storeName
	for _, itemName := range grid.Items {
		bestStore := ""
		bestPrice := math.MaxFloat64
		for _, storeName := range grid.Stores {
			price := priceMatrix[itemName][storeName]
			if price > 0 && price < bestPrice {
				bestPrice = price
				bestStore = storeName
			}
		}
		if bestStore != "" {
			itemAssignment[itemName] = bestStore
		}
	}

	// Step 3: Group items by assigned store
	storeItems := make(map[string][]string) // storeName -> []itemName
	for itemName, storeName := range itemAssignment {
		storeItems[storeName] = append(storeItems[storeName], itemName)
	}

	// Step 4: Merge small trips — if the savings from a store don't justify the trip
	// Calculate savings per store (vs buying those items at next-cheapest store)
	for {
		merged := false
		for storeName, itemNames := range storeItems {
			if len(storeItems) <= 1 {
				break // don't merge if we're down to 1 store
			}

			// Calculate how much we save by shopping at this store vs next-best
			savingsAtThisStore := 0.0
			for _, itemName := range itemNames {
				currentPrice := priceMatrix[itemName][storeName]
				nextBestPrice := nextCheapestPrice(priceMatrix[itemName], storeName)
				if nextBestPrice > 0 {
					savingsAtThisStore += nextBestPrice - currentPrice
				}
			}

			// If savings don't justify the trip, reassign items to their next-best store
			if savingsAtThisStore < tripPenalty {
				for _, itemName := range itemNames {
					nextStore := nextCheapestStore(priceMatrix[itemName], storeName)
					if nextStore != "" {
						storeItems[nextStore] = append(storeItems[nextStore], itemName)
					}
				}
				delete(storeItems, storeName)
				merged = true
				break // restart the loop after modifying the map
			}
		}
		if !merged {
			break
		}
	}

	// Step 5: Build the trip plan
	var trips []model.Trip
	optimizedTotal := 0.0

	// Sort store names for consistent output
	sortedStores := make([]string, 0, len(storeItems))
	for storeName := range storeItems {
		sortedStores = append(sortedStores, storeName)
	}
	sort.Strings(sortedStores)

	// Build an item lookup for quantities/units
	itemLookup := make(map[string]model.GroceryItem)
	for _, item := range items {
		itemLookup[item.Name] = item
	}

	for _, storeName := range sortedStores {
		itemNames := storeItems[storeName]
		sort.Strings(itemNames)

		var tripItems []model.GroceryItem
		subtotal := 0.0

		for _, itemName := range itemNames {
			price := priceMatrix[itemName][storeName]
			subtotal += price

			if item, ok := itemLookup[itemName]; ok {
				tripItems = append(tripItems, item)
			} else {
				tripItems = append(tripItems, model.GroceryItem{Name: itemName, Quantity: 1, Unit: "each"})
			}
		}

		optimizedTotal += subtotal
		trips = append(trips, model.Trip{
			Store:    storeName,
			Items:    tripItems,
			Subtotal: subtotal,
		})
	}

	savings := bestSingleTotal - optimizedTotal
	if savings < 0 {
		savings = 0
	}

	return model.TripPlan{
		Trips:     trips,
		TotalCost: optimizedTotal,
		Savings:   savings,
	}
}

// nextCheapestPrice returns the cheapest price for an item excluding the given store.
func nextCheapestPrice(storePrices map[string]float64, excludeStore string) float64 {
	best := 0.0
	for store, price := range storePrices {
		if store == excludeStore || price == 0 {
			continue
		}
		if best == 0 || price < best {
			best = price
		}
	}
	return best
}

// nextCheapestStore returns the store name with the cheapest price excluding the given store.
func nextCheapestStore(storePrices map[string]float64, excludeStore string) string {
	bestStore := ""
	bestPrice := 0.0
	for store, price := range storePrices {
		if store == excludeStore || price == 0 {
			continue
		}
		if bestPrice == 0 || price < bestPrice {
			bestPrice = price
			bestStore = store
		}
	}
	return bestStore
}

