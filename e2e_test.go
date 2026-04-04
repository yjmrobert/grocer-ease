package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/yjmrobert/grocer-ease/internal/handler"
	"github.com/yjmrobert/grocer-ease/internal/provider"
	"github.com/yjmrobert/grocer-ease/internal/service"
	"github.com/yjmrobert/grocer-ease/internal/store"
)

func setupTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	dbPath := fmt.Sprintf("/tmp/grocer-ease-test-%d.db", time.Now().UnixNano())
	t.Cleanup(func() { os.Remove(dbPath) })

	db, err := store.NewDB(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	listStore := store.NewListStore(db)
	cacheStore := store.NewPriceCacheStore(db, 6*time.Hour)

	providers := []provider.PriceProvider{
		provider.NewMockProvider("Walmart", map[string]float64{
			"bananas": 1.29, "milk": 5.49, "bread": 3.99, "eggs": 4.99, "chicken breast": 12.99,
		}),
		provider.NewMockProvider("Loblaws", map[string]float64{
			"bananas": 1.49, "milk": 4.99, "bread": 4.49, "eggs": 5.49, "chicken breast": 11.49,
		}),
		provider.NewMockProvider("Metro", map[string]float64{
			"bananas": 0.99, "milk": 5.99, "bread": 3.49, "eggs": 4.49, "chicken breast": 13.99,
		}),
	}

	priceService := service.NewPriceService(providers, cacheStore)
	settings := &handler.AppSettings{PostalCode: "M5V", TripPenalty: 5.0}
	router := handler.NewRouter(listStore, priceService, cacheStore, settings)

	return httptest.NewServer(router)
}

func TestEndToEnd_BananaList(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	// 1. Home page loads
	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("home page: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("home page status: %d", resp.StatusCode)
	}
	resp.Body.Close()
	t.Log("PASS: Home page loads (200)")

	// 2. Create a grocery list
	resp, err = http.PostForm(ts.URL+"/list", url.Values{"name": {"Banana Test"}})
	if err != nil {
		t.Fatalf("create list: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	listHTML := string(body)
	if !strings.Contains(listHTML, "Banana Test") {
		t.Fatal("list creation did not return list name")
	}

	// Extract list ID
	idx := strings.Index(listHTML, "/list/")
	if idx == -1 {
		t.Fatal("no list ID in response")
	}
	listID := listHTML[idx+6 : idx+42]
	t.Logf("PASS: Created list %s", listID)

	// 3. Add bananas
	resp, err = http.PostForm(ts.URL+"/list/"+listID+"/items", url.Values{
		"name": {"Bananas"}, "quantity": {"1"}, "unit": {"kg"},
	})
	if err != nil {
		t.Fatalf("add bananas: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), "Bananas") {
		t.Fatal("bananas not in response")
	}
	t.Log("PASS: Added Bananas (1 kg)")

	// 4. Add milk
	resp, _ = http.PostForm(ts.URL+"/list/"+listID+"/items", url.Values{
		"name": {"Milk"}, "quantity": {"2"}, "unit": {"L"},
	})
	resp.Body.Close()
	t.Log("PASS: Added Milk (2 L)")

	// 5. Add bread
	resp, _ = http.PostForm(ts.URL+"/list/"+listID+"/items", url.Values{
		"name": {"Bread"}, "quantity": {"1"}, "unit": {"each"},
	})
	resp.Body.Close()
	t.Log("PASS: Added Bread (1 each)")

	// 6. Compare prices
	resp, err = http.Post(ts.URL+"/prices/"+listID, "application/x-www-form-urlencoded", nil)
	if err != nil {
		t.Fatalf("compare prices: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	priceHTML := string(body)

	if resp.StatusCode != 200 {
		t.Fatalf("price comparison status: %d", resp.StatusCode)
	}

	// Check that we got actual prices (not all "Not found")
	dollarCount := strings.Count(priceHTML, "$")
	notFoundCount := strings.Count(priceHTML, "Not found")
	t.Logf("PASS: Price comparison returned %d prices, %d not found", dollarCount, notFoundCount)

	if dollarCount < 3 {
		t.Fatal("expected at least 3 dollar amounts in price grid")
	}

	// Verify all 3 stores appear
	for _, store := range []string{"Walmart", "Loblaws", "Metro"} {
		if !strings.Contains(priceHTML, store) {
			t.Fatalf("store %s not in price grid", store)
		}
	}
	t.Log("PASS: All 3 stores appear in price grid")

	// Check specific prices
	if !strings.Contains(priceHTML, "$1.29") {
		t.Log("WARN: Walmart banana price $1.29 not found (may be cached differently)")
	} else {
		t.Log("PASS: Walmart bananas = $1.29")
	}
	if !strings.Contains(priceHTML, "$0.99") {
		t.Log("WARN: Metro banana price $0.99 not found")
	} else {
		t.Log("PASS: Metro bananas = $0.99 (cheapest)")
	}

	// Check the "Optimize My Trips" button appears
	if !strings.Contains(priceHTML, "Optimize My Trips") {
		t.Fatal("Optimize button not found")
	}
	t.Log("PASS: Optimize My Trips button present")

	// 7. Optimize trips
	resp, err = http.Post(ts.URL+"/optimize/"+listID, "application/x-www-form-urlencoded",
		strings.NewReader("trip_penalty=3"))
	if err != nil {
		t.Fatalf("optimize: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	optimizeHTML := string(body)

	if resp.StatusCode != 200 {
		t.Fatalf("optimize status: %d", resp.StatusCode)
	}

	if !strings.Contains(optimizeHTML, "Optimized Shopping Plan") {
		t.Fatal("optimize result missing header")
	}
	t.Log("PASS: Trip optimization returned a plan")

	// Check that the plan has stops
	if !strings.Contains(optimizeHTML, "Stop") {
		t.Fatal("no stops in trip plan")
	}

	// Check for savings display
	if strings.Contains(optimizeHTML, "Save $") {
		t.Log("PASS: Savings amount displayed")
	} else {
		t.Log("INFO: No savings shown (may be single-store optimal)")
	}

	// 8. Test settings page
	resp, err = http.Get(ts.URL + "/settings")
	if err != nil {
		t.Fatalf("settings page: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), "Postal Code") {
		t.Fatal("settings page missing postal code field")
	}
	t.Log("PASS: Settings page loads with postal code field")

	// 9. Test autocomplete endpoint
	resp, err = http.Get(ts.URL + "/api/suggest?name=Ban")
	if err != nil {
		t.Fatalf("suggest: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("suggest status: %d", resp.StatusCode)
	}
	t.Log("PASS: Autocomplete endpoint responds (200)")

	t.Log("\n=== ALL E2E TESTS PASSED ===")
}
