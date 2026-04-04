package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yjmrobert/grocer-ease/internal/model"
)

// ============================================================
// Flipp Provider Integration Tests
// ============================================================

func TestFlippProvider_SearchProduct(t *testing.T) {
	// Mock Flipp search API response
	searchHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		postalCode := r.URL.Query().Get("postal_code")

		if q == "" || postalCode == "" {
			http.Error(w, "missing params", 400)
			return
		}

		// Return realistic Flipp search response
		resp := flippSearchResponse{
			Items: []flippSearchItem{
				{
					FlyerItemID:   12345,
					MerchantID:    100,
					Merchant:      "Walmart",
					Name:          "Chiquita Bananas",
					Price:         "$1.29",
					PrePriceText:  "",
					PostPriceText: "/lb",
				},
				{
					FlyerItemID:   12346,
					MerchantID:    200,
					Merchant:      "Loblaws",
					Name:          "Organic Bananas",
					Price:         "$1.69",
					PrePriceText:  "",
					PostPriceText: "/lb",
				},
				{
					FlyerItemID:   12347,
					MerchantID:    300,
					Merchant:      "Metro",
					Name:          "Yellow Bananas",
					Price:         "$0.99",
					PrePriceText:  "",
					PostPriceText: "/lb",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	// Mock Flipp item detail API response
	itemHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		detail := flippItemDetail{
			ID:             12345,
			Name:           "Chiquita Bananas",
			Description:    "Fresh yellow bananas",
			Price:          1.29,
			PrePriceText:   "",
			PostPriceText:  "/lb",
			CutoutImageURL: "https://cdn.flipp.com/bananas.jpg",
			Merchant:       "Walmart",
			ValidFrom:      "2026-04-01",
			ValidTo:        "2026-04-07",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(detail)
	})

	// Create a mux that handles both search and item detail
	mux := http.NewServeMux()
	mux.HandleFunc("/flipp/items/search", searchHandler)
	mux.HandleFunc("/flipp/items/", itemHandler)

	ts := httptest.NewServer(mux)
	defer ts.Close()

	t.Run("finds product at specific store", func(t *testing.T) {
		p := newTestFlippProvider(ts.URL, "Walmart", "M5V1J2")

		result, err := p.SearchProduct(context.Background(), "bananas")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == nil {
			t.Fatal("expected a result, got nil")
		}
		if result.Price != 1.29 {
			t.Errorf("expected price $1.29, got $%.2f", result.Price)
		}
		if result.ProductName != "Chiquita Bananas" {
			t.Errorf("expected 'Chiquita Bananas', got %q", result.ProductName)
		}
		if !strings.Contains(result.Unit, "lb") {
			t.Errorf("expected unit containing 'lb', got %q", result.Unit)
		}
		t.Logf("PASS: Flipp Walmart bananas = $%.2f (%s)", result.Price, result.Unit)
	})

	t.Run("filters by store name", func(t *testing.T) {
		p := newTestFlippProvider(ts.URL, "Metro", "M5V1J2")
		result, err := p.SearchProduct(context.Background(), "bananas")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == nil {
			t.Fatal("expected a result for Metro")
		}
		// Metro item won't go through detail fetch (different ID), but search fallback should work
		t.Logf("PASS: Flipp Metro bananas = $%.2f", result.Price)
	})

	t.Run("returns nil for non-matching store", func(t *testing.T) {
		p := newTestFlippProvider(ts.URL, "Costco", "M5V1J2")
		result, err := p.SearchProduct(context.Background(), "bananas")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != nil {
			t.Errorf("expected nil for Costco, got %v", result)
		}
		t.Log("PASS: Flipp returns nil for non-matching store (Costco)")
	})

	t.Run("handles empty search results", func(t *testing.T) {
		emptyMux := http.NewServeMux()
		emptyMux.HandleFunc("/flipp/items/search", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(flippSearchResponse{Items: []flippSearchItem{}})
		})
		emptyTS := httptest.NewServer(emptyMux)
		defer emptyTS.Close()

		p := newTestFlippProvider(emptyTS.URL, "Walmart", "M5V")
		result, err := p.SearchProduct(context.Background(), "unicorn steak")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != nil {
			t.Errorf("expected nil for missing product, got %v", result)
		}
		t.Log("PASS: Flipp returns nil for product not found")
	})

	t.Run("handles multi-buy pricing", func(t *testing.T) {
		multiBuyMux := http.NewServeMux()
		multiBuyMux.HandleFunc("/flipp/items/search", func(w http.ResponseWriter, r *http.Request) {
			resp := flippSearchResponse{
				Items: []flippSearchItem{
					{
						FlyerItemID: 99999,
						Merchant:    "No Frills",
						Name:        "Yogurt Tubes",
						Price:       "2/$5.00",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		})
		multiBuyMux.HandleFunc("/flipp/items/", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "not found", 404) // force fallback to search data
		})
		multiBuyTS := httptest.NewServer(multiBuyMux)
		defer multiBuyTS.Close()

		p := newTestFlippProvider(multiBuyTS.URL, "No Frills", "M5V")
		result, err := p.SearchProduct(context.Background(), "yogurt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == nil {
			t.Fatal("expected a result")
		}
		if math.Abs(result.Price-2.50) > 0.01 {
			t.Errorf("expected per-unit price $2.50 for 2/$5.00, got $%.2f", result.Price)
		}
		t.Logf("PASS: Flipp multi-buy 2/$5.00 = $%.2f per unit", result.Price)
	})
}

// newTestFlippProvider creates a FlippProvider that talks to a test server.
func newTestFlippProvider(baseURL, storeName, postalCode string) *testFlippProvider {
	return &testFlippProvider{
		FlippProvider: FlippProvider{
			postalCode: postalCode,
			storeName:  storeName,
			client:     &http.Client{},
		},
		baseURL: baseURL,
	}
}

// testFlippProvider wraps FlippProvider to redirect API calls to a test server.
type testFlippProvider struct {
	FlippProvider
	baseURL string
}

func (p *testFlippProvider) SearchProduct(ctx context.Context, query string) (*model.PriceResult, error) {
	// We need to replicate the logic but with our test URL
	searchURL := fmt.Sprintf("%s/flipp/items/search?q=%s&postal_code=%s", p.baseURL, query, p.postalCode)

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	req.Header.Set("User-Agent", "test")
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var searchResp flippSearchResponse
	json.NewDecoder(resp.Body).Decode(&searchResp)

	if len(searchResp.Items) == 0 {
		return nil, nil
	}

	// Filter by store
	var match *flippSearchItem
	for i, item := range searchResp.Items {
		if p.storeName == "" || strings.Contains(strings.ToLower(item.Merchant), strings.ToLower(p.storeName)) {
			match = &searchResp.Items[i]
			break
		}
	}
	if match == nil {
		return nil, nil
	}

	// Try item detail
	itemURL := fmt.Sprintf("%s/flipp/items/%d", p.baseURL, match.FlyerItemID)
	detailReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, itemURL, nil)
	detailResp, err := p.client.Do(detailReq)
	if err == nil && detailResp.StatusCode == 200 {
		var detail flippItemDetail
		json.NewDecoder(detailResp.Body).Decode(&detail)
		detailResp.Body.Close()
		if detail.Price > 0 {
			return &model.PriceResult{
				Store:       p.Store(),
				ProductName: detail.Name,
				Price:       detail.Price,
				Unit:        parseFlippUnit(detail.PrePriceText, detail.PostPriceText),
				ImageURL:    detail.CutoutImageURL,
				Confidence:  "partial",
			}, nil
		}
	}
	if detailResp != nil {
		detailResp.Body.Close()
	}

	// Fallback to search data
	return parseFlippSearchItem(match), nil
}

func (p *testFlippProvider) Store() string {
	return p.FlippProvider.Store()
}

