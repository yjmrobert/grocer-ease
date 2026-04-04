package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/yjmrobert/grocer-ease/internal/model"
	"github.com/yjmrobert/grocer-ease/internal/provider"
	"github.com/yjmrobert/grocer-ease/internal/store"
	"github.com/yjmrobert/grocer-ease/internal/view"
)

type PriceService struct {
	providers  []provider.PriceProvider
	cacheStore *store.PriceCacheStore
}

func NewPriceService(providers []provider.PriceProvider, cacheStore *store.PriceCacheStore) *PriceService {
	return &PriceService{
		providers:  providers,
		cacheStore: cacheStore,
	}
}

// HasProviders returns true if at least one price provider is configured.
func (s *PriceService) HasProviders() bool {
	return len(s.providers) > 0
}

// itemStoreResult holds a single price lookup result for one item at one store.
type itemStoreResult struct {
	ItemName string
	Store    string
	Result   *model.PriceResult
}

// ComparePrices queries all providers for all items concurrently, using cache when available.
// Returns a PriceGridData ready for rendering.
func (s *PriceService) ComparePrices(ctx context.Context, items []model.GroceryItem) view.PriceGridData {
	storeNames := make([]string, len(s.providers))
	for i, p := range s.providers {
		storeNames[i] = p.Store()
	}

	itemNames := make([]string, len(items))
	for i, item := range items {
		itemNames[i] = item.Name
	}

	// Query all item×store combinations concurrently
	var mu sync.Mutex
	var wg sync.WaitGroup
	results := make([]itemStoreResult, 0, len(items)*len(s.providers))

	for _, item := range items {
		for _, prov := range s.providers {
			wg.Add(1)
			go func(itemName string, p provider.PriceProvider) {
				defer wg.Done()
				result := s.lookupPrice(ctx, itemName, p)
				mu.Lock()
				results = append(results, itemStoreResult{
					ItemName: itemName,
					Store:    p.Store(),
					Result:   result,
				})
				mu.Unlock()
			}(item.Name, prov)
		}
	}
	wg.Wait()

	// Build the prices map and totals
	prices := make(map[string]map[string]string)
	totals := make(map[string]float64)

	// Initialize prices map
	for _, name := range itemNames {
		prices[name] = make(map[string]string)
		for _, storeName := range storeNames {
			prices[name][storeName] = "Not found"
		}
	}

	// Fill in results
	for _, r := range results {
		if r.Result != nil {
			prices[r.ItemName][r.Store] = fmt.Sprintf("$%.2f", r.Result.Price)
			totals[r.Store] += r.Result.Price
		}
	}

	return view.PriceGridData{
		Items:  itemNames,
		Stores: storeNames,
		Prices: prices,
		Totals: totals,
	}
}

// lookupPrice checks the cache first, then queries the provider.
func (s *PriceService) lookupPrice(ctx context.Context, itemName string, p provider.PriceProvider) *model.PriceResult {
	normalizedQuery := strings.ToLower(strings.TrimSpace(itemName))

	// Check cache
	cached, err := s.cacheStore.Get(normalizedQuery, p.Store())
	if err != nil {
		log.Printf("cache lookup error for %q at %s: %v", itemName, p.Store(), err)
	}
	if cached != nil {
		return &model.PriceResult{
			Store:       cached.Store,
			ProductName: cached.ProductName,
			Price:       cached.Price,
			Unit:        cached.Unit,
			Confidence:  "exact",
		}
	}

	// Query provider
	result, err := p.SearchProduct(ctx, normalizedQuery)
	if err != nil {
		log.Printf("provider %s error for %q: %v", p.Store(), itemName, err)
		return nil
	}
	if result == nil {
		return nil
	}

	// Cache the result
	if err := s.cacheStore.Set(normalizedQuery, p.Store(), result.ProductName, result.Price, result.Unit); err != nil {
		log.Printf("cache store error for %q at %s: %v", itemName, p.Store(), err)
	}

	return result
}
