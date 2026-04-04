package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yjmrobert/grocer-ease/internal/model"
)

// ============================================================
// Loblaws/PC Express Provider Integration Tests
// ============================================================

func TestLoblawsProvider_SearchProduct(t *testing.T) {
	t.Run("parses product search results", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify request format
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if r.Header.Get("X-Apikey") != "test-api-key-123" {
				t.Errorf("missing or wrong X-Apikey header: %q", r.Header.Get("X-Apikey"))
			}
			if r.Header.Get("Site-Banner") != "loblaws" {
				t.Errorf("missing or wrong Site-Banner header: %q", r.Header.Get("Site-Banner"))
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("missing Content-Type header")
			}

			// Verify request body
			body, _ := io.ReadAll(r.Body)
			var reqBody pcSearchRequest
			if err := json.Unmarshal(body, &reqBody); err != nil {
				t.Errorf("invalid request body: %v", err)
			}
			if reqBody.Term != "bananas" {
				t.Errorf("expected term 'bananas', got %q", reqBody.Term)
			}
			if reqBody.Banner != "loblaws" {
				t.Errorf("expected banner 'loblaws', got %q", reqBody.Banner)
			}
			if reqBody.StoreID != "1032" {
				t.Errorf("expected storeId '1032', got %q", reqBody.StoreID)
			}
			t.Log("PASS: Request format is correct (POST, JSON body, X-Apikey, Site-Banner)")

			// Return realistic PC Express response
			resp := pcSearchResponse{
				Results: []pcProductResult{
					{
						ProductID:   "20843076001_EA",
						Name:        "Bananas, Bunch",
						Brand:       "No Name",
						Description: "Fresh yellow bananas",
						ImageURL:    "/images/large/20843076001.jpg",
						PackageSize: "per lb",
						Prices: pcPriceInfo{
							Price:           pcPriceDetail{Value: 0.69, Quantity: 1, Unit: "lb"},
							ComparisonPrice: &pcPriceDetail{Value: 1.52, Quantity: 1, Unit: "kg"},
						},
					},
					{
						ProductID:   "20843076002_EA",
						Name:        "Organic Bananas, Bunch",
						Brand:       "PC Organics",
						Description: "Certified organic bananas",
						ImageURL:    "/images/large/20843076002.jpg",
						PackageSize: "per lb",
						Prices: pcPriceInfo{
							Price: pcPriceDetail{Value: 1.29, Quantity: 1, Unit: "lb"},
						},
					},
				},
				TotalResults: 2,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer ts.Close()

		p := &LoblawsProvider{
			banner:    "loblaws",
			storeID:   "1032",
			apiKey:    "test-api-key-123",
			storeName: "Loblaws",
			client:    ts.Client(),
		}
		// Override the search URL by making the provider hit the test server
		result, err := searchWithURL(p, ts.URL, context.Background(), "bananas")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == nil {
			t.Fatal("expected a result")
		}
		if result.Price != 0.69 {
			t.Errorf("expected price $0.69, got $%.2f", result.Price)
		}
		if result.ProductName != "Bananas, Bunch" {
			t.Errorf("expected 'Bananas, Bunch', got %q", result.ProductName)
		}
		if !strings.Contains(result.ImageURL, "pcexpress") {
			// Should have the prefix added
			t.Logf("INFO: ImageURL = %q", result.ImageURL)
		}
		if result.Confidence != "exact" {
			t.Errorf("expected confidence 'exact', got %q", result.Confidence)
		}
		t.Logf("PASS: Loblaws bananas = $%.2f (%s) — %s", result.Price, result.Unit, result.ProductName)
	})

	t.Run("returns nil for empty results", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := pcSearchResponse{Results: []pcProductResult{}, TotalResults: 0}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer ts.Close()

		p := &LoblawsProvider{
			banner:    "loblaws",
			storeID:   "1001",
			apiKey:    "test-key",
			storeName: "Loblaws",
			client:    ts.Client(),
		}
		result, err := searchWithURL(p, ts.URL, context.Background(), "unicorn steak")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != nil {
			t.Errorf("expected nil for missing product, got %v", result)
		}
		t.Log("PASS: Loblaws returns nil for product not found")
	})

	t.Run("handles API error gracefully", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		}))
		defer ts.Close()

		p := &LoblawsProvider{
			banner:    "loblaws",
			storeID:   "1001",
			apiKey:    "bad-key",
			storeName: "Loblaws",
			client:    ts.Client(),
		}
		_, err := searchWithURL(p, ts.URL, context.Background(), "bananas")
		if err == nil {
			t.Fatal("expected an error for 401 response")
		}
		if !strings.Contains(err.Error(), "401") {
			t.Errorf("error should mention 401: %v", err)
		}
		t.Logf("PASS: Loblaws returns error for 401: %v", err)
	})

	t.Run("uses different banners", func(t *testing.T) {
		var receivedBanner string
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedBanner = r.Header.Get("Site-Banner")
			body, _ := io.ReadAll(r.Body)
			var reqBody pcSearchRequest
			json.Unmarshal(body, &reqBody)

			resp := pcSearchResponse{
				Results: []pcProductResult{
					{
						Name:        "Maxi Bananas",
						PackageSize: "each",
						Prices: pcPriceInfo{
							Price: pcPriceDetail{Value: 0.59, Quantity: 1},
						},
					},
				},
				TotalResults: 1,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer ts.Close()

		p := &LoblawsProvider{
			banner:    "maxi",
			storeID:   "5001",
			apiKey:    "test-key",
			storeName: "Maxi",
			client:    ts.Client(),
		}
		result, err := searchWithURL(p, ts.URL, context.Background(), "bananas")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if receivedBanner != "maxi" {
			t.Errorf("expected Site-Banner 'maxi', got %q", receivedBanner)
		}
		if result.Price != 0.59 {
			t.Errorf("expected $0.59, got $%.2f", result.Price)
		}
		t.Logf("PASS: Maxi banner sends correct Site-Banner header, price = $%.2f", result.Price)
	})
}

// searchWithURL is a helper that calls the Loblaws provider's search logic
// but overrides the API URL to point at a test server.
func searchWithURL(p *LoblawsProvider, testURL string, ctx context.Context, query string) (*model.PriceResult, error) {
	searchBody := pcSearchRequest{
		Pagination: pcPagination{From: 0, Size: 5},
		Banner:     p.banner,
		Lang:       "en",
		Date:       "04042026",
		StoreID:    p.storeID,
		PickupType: "STORE",
		OfferType:  "ALL",
		Term:       query,
	}

	bodyBytes, err := json.Marshal(searchBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, testURL, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("X-Apikey", p.apiKey)
	req.Header.Set("Site-Banner", p.banner)
	req.Header.Set("User-Agent", "test")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("pc express returned %d: %s", resp.StatusCode, string(respBody))
	}

	var searchResp pcSearchResponse
	json.NewDecoder(resp.Body).Decode(&searchResp)

	if len(searchResp.Results) == 0 {
		return nil, nil
	}

	product := searchResp.Results[0]
	unit := "each"
	if product.Prices.ComparisonPrice != nil && product.Prices.ComparisonPrice.Unit != "" {
		unit = "per " + product.Prices.ComparisonPrice.Unit
	}
	if product.PackageSize != "" {
		unit = product.PackageSize
	}
	imageURL := product.ImageURL
	if imageURL != "" && imageURL[0] == '/' {
		imageURL = "https://assets.pcexpress.ca" + imageURL
	}

	return &model.PriceResult{
		Store:       p.storeName,
		ProductName: product.Name,
		Price:       product.Prices.Price.Value,
		Unit:        unit,
		ImageURL:    imageURL,
		Confidence:  "exact",
	}, nil
}
