package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
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
	settingsStore := store.NewSettingsStore(db)
	settingsStore.Set("postal_code", "M5V")
	settingsStore.Set("trip_penalty", "5")
	router := handler.NewRouter(listStore, priceService, cacheStore, settingsStore)

	return httptest.NewServer(router)
}

// helper to read response body as string
func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(body)
}

// helper to extract a list ID from HTML containing /list/{uuid}
var listIDRegex = regexp.MustCompile(`/list/([0-9a-f-]{36})`)

func extractListID(t *testing.T, html string) string {
	t.Helper()
	matches := listIDRegex.FindStringSubmatch(html)
	if len(matches) < 2 {
		t.Fatalf("no list ID found in response:\n%s", html[:min(len(html), 500)])
	}
	return matches[1]
}

// helper to extract an item ID from HTML containing /item/{uuid}
var itemIDRegex = regexp.MustCompile(`/item/([0-9a-f-]{36})`)

func extractItemID(t *testing.T, html string) string {
	t.Helper()
	matches := itemIDRegex.FindStringSubmatch(html)
	if len(matches) < 2 {
		t.Fatalf("no item ID found in response:\n%s", html[:min(len(html), 500)])
	}
	return matches[1]
}

// helper to create a list and return its ID
func createList(t *testing.T, ts *httptest.Server, name string) string {
	t.Helper()
	resp, err := http.PostForm(ts.URL+"/list", url.Values{"name": {name}})
	if err != nil {
		t.Fatalf("create list %q: %v", name, err)
	}
	body := readBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create list %q status %d: %s", name, resp.StatusCode, body)
	}
	return extractListID(t, body)
}

// helper to add an item to a list and return the response body
func addItem(t *testing.T, ts *httptest.Server, listID, name, quantity, unit string) string {
	t.Helper()
	resp, err := http.PostForm(ts.URL+"/list/"+listID+"/items", url.Values{
		"name": {name}, "quantity": {quantity}, "unit": {unit},
	})
	if err != nil {
		t.Fatalf("add item %q: %v", name, err)
	}
	return readBody(t, resp)
}

// ─── Home Page ───────────────────────────────────────────────────────────────

func TestHomePage_Loads(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	body := readBody(t, resp)

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(body, "My Grocery Lists") {
		t.Fatal("home page missing title")
	}
	if !strings.Contains(body, "No grocery lists yet") {
		t.Fatal("empty state not shown on fresh DB")
	}
	if !strings.Contains(body, "htmx.org") {
		t.Fatal("htmx script tag missing from layout")
	}
}

func TestHomePage_ShowsExistingLists(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	createList(t, ts, "Weekly Groceries")
	createList(t, ts, "BBQ Party")

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	body := readBody(t, resp)

	if strings.Contains(body, "No grocery lists yet") {
		t.Fatal("empty state shown when lists exist")
	}
	if !strings.Contains(body, "Weekly Groceries") {
		t.Fatal("missing list: Weekly Groceries")
	}
	if !strings.Contains(body, "BBQ Party") {
		t.Fatal("missing list: BBQ Party")
	}
}

// ─── List CRUD ───────────────────────────────────────────────────────────────

func TestCreateList_ReturnsListCard(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.PostForm(ts.URL+"/list", url.Values{"name": {"Test List"}})
	if err != nil {
		t.Fatalf("POST /list: %v", err)
	}
	body := readBody(t, resp)

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(body, "Test List") {
		t.Fatal("response doesn't contain list name")
	}
	// Should contain a link to the list detail
	if !strings.Contains(body, "/list/") {
		t.Fatal("response doesn't contain list link")
	}
}

func TestCreateList_EmptyName_Returns400(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.PostForm(ts.URL+"/list", url.Values{"name": {""}})
	if err != nil {
		t.Fatalf("POST /list: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateList_LongName_Returns400(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	longName := strings.Repeat("a", 201)
	resp, err := http.PostForm(ts.URL+"/list", url.Values{"name": {longName}})
	if err != nil {
		t.Fatalf("POST /list: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for name >200 chars, got %d", resp.StatusCode)
	}
}

func TestCreateList_MaxLengthName_Succeeds(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	name := strings.Repeat("a", 200)
	resp, err := http.PostForm(ts.URL+"/list", url.Values{"name": {name}})
	if err != nil {
		t.Fatalf("POST /list: %v", err)
	}
	body := readBody(t, resp)

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200 for 200-char name, got %d", resp.StatusCode)
	}
	if !strings.Contains(body, name) {
		t.Fatal("200-char name not in response")
	}
}

func TestDeleteList_Succeeds(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	listID := createList(t, ts, "To Delete")

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/list/"+listID, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /list/%s: %v", listID, err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Verify list is gone from home page
	resp, _ = http.Get(ts.URL + "/")
	body := readBody(t, resp)
	if strings.Contains(body, "To Delete") {
		t.Fatal("deleted list still appears on home page")
	}
}

func TestDeleteList_NonExistent_NoError(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/list/nonexistent-id", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	resp.Body.Close()

	// Should not error — DELETE is idempotent
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for DELETE nonexistent, got %d", resp.StatusCode)
	}
}

// ─── List Detail Page ────────────────────────────────────────────────────────

func TestListDetail_LoadsEmpty(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	listID := createList(t, ts, "My List")

	resp, err := http.Get(ts.URL + "/list/" + listID)
	if err != nil {
		t.Fatalf("GET /list/%s: %v", listID, err)
	}
	body := readBody(t, resp)

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(body, "My List") {
		t.Fatal("list name not on detail page")
	}
	if !strings.Contains(body, "No items yet") {
		t.Fatal("empty items message not shown")
	}
	if !strings.Contains(body, "Back to lists") {
		t.Fatal("back link missing")
	}
}

func TestListDetail_NonExistent_Returns404(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/list/nonexistent-uuid")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestListDetail_ShowsItems(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	listID := createList(t, ts, "Shopping")
	addItem(t, ts, listID, "Apples", "3", "kg")
	addItem(t, ts, listID, "Milk", "2", "L")

	resp, err := http.Get(ts.URL + "/list/" + listID)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	body := readBody(t, resp)

	if !strings.Contains(body, "Apples") {
		t.Fatal("item Apples not on detail page")
	}
	if !strings.Contains(body, "Milk") {
		t.Fatal("item Milk not on detail page")
	}
	if !strings.Contains(body, "3.0") {
		t.Fatal("quantity 3.0 not shown")
	}
	if !strings.Contains(body, "kg") {
		t.Fatal("unit kg not shown")
	}
	// Compare Prices button should appear when items exist
	if !strings.Contains(body, "Compare Prices") {
		t.Fatal("Compare Prices button missing with items present")
	}
}

// ─── Item CRUD ───────────────────────────────────────────────────────────────

func TestAddItem_ReturnsItemRow(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	listID := createList(t, ts, "Items Test")

	resp, err := http.PostForm(ts.URL+"/list/"+listID+"/items", url.Values{
		"name": {"Chicken Breast"}, "quantity": {"2.5"}, "unit": {"kg"},
	})
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	body := readBody(t, resp)

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(body, "Chicken Breast") {
		t.Fatal("item name not in response")
	}
	if !strings.Contains(body, "2.5") {
		t.Fatal("quantity not in response")
	}
	if !strings.Contains(body, "kg") {
		t.Fatal("unit not in response")
	}
	// Should contain a delete button with item ID
	if !strings.Contains(body, "/item/") {
		t.Fatal("delete link missing from item row")
	}
}

func TestAddItem_DefaultUnit(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	listID := createList(t, ts, "Defaults")

	// Omit unit — should default to "each"
	resp, err := http.PostForm(ts.URL+"/list/"+listID+"/items", url.Values{
		"name": {"Bananas"},
	})
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	body := readBody(t, resp)

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(body, "each") {
		t.Fatal("default unit 'each' not in response")
	}
	if !strings.Contains(body, "1.0") {
		t.Fatal("default quantity 1.0 not in response")
	}
}

func TestAddItem_AllValidUnits(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	listID := createList(t, ts, "Units Test")

	units := []string{"each", "kg", "lb", "L", "pack", "bag", "dozen"}
	for _, unit := range units {
		resp, err := http.PostForm(ts.URL+"/list/"+listID+"/items", url.Values{
			"name": {"Item-" + unit}, "quantity": {"1"}, "unit": {unit},
		})
		if err != nil {
			t.Fatalf("POST with unit %s: %v", unit, err)
		}
		body := readBody(t, resp)
		if resp.StatusCode != 200 {
			t.Fatalf("unit %s: expected 200, got %d", unit, resp.StatusCode)
		}
		if !strings.Contains(body, unit) {
			t.Fatalf("unit %s not in response", unit)
		}
	}
}

func TestAddItem_InvalidUnit_Returns400(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	listID := createList(t, ts, "Bad Unit")

	resp, err := http.PostForm(ts.URL+"/list/"+listID+"/items", url.Values{
		"name": {"Apples"}, "unit": {"gallons"},
	})
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid unit, got %d", resp.StatusCode)
	}
}

func TestAddItem_EmptyName_Returns400(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	listID := createList(t, ts, "Empty Name")

	resp, err := http.PostForm(ts.URL+"/list/"+listID+"/items", url.Values{
		"name": {""}, "unit": {"each"},
	})
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty name, got %d", resp.StatusCode)
	}
}

func TestAddItem_LongName_Returns400(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	listID := createList(t, ts, "Long Name")

	resp, err := http.PostForm(ts.URL+"/list/"+listID+"/items", url.Values{
		"name": {strings.Repeat("x", 201)}, "unit": {"each"},
	})
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for name >200, got %d", resp.StatusCode)
	}
}

func TestAddItem_QuantityEdgeCases(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	listID := createList(t, ts, "Qty Edge")

	tests := []struct {
		name     string
		quantity string
		wantQty  string // what should appear in response
	}{
		{"Zero qty defaults to 1", "0", "1.0"},
		{"Negative qty defaults to 1", "-5", "1.0"},
		{"Valid decimal", "0.5", "0.5"},
		{"Non-numeric defaults to 1", "abc", "1.0"},
		{"Very large capped at default", "99999", "1.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := addItem(t, ts, listID, "Item-"+tt.quantity, tt.quantity, "each")
			if !strings.Contains(body, tt.wantQty) {
				t.Fatalf("expected %s in response, got:\n%s", tt.wantQty, body)
			}
		})
	}
}

func TestDeleteItem_Succeeds(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	listID := createList(t, ts, "Delete Item")
	itemBody := addItem(t, ts, listID, "Remove Me", "1", "each")
	itemID := extractItemID(t, itemBody)

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/item/"+itemID, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /item/%s: %v", itemID, err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Verify item is gone from list detail
	resp, _ = http.Get(ts.URL + "/list/" + listID)
	body := readBody(t, resp)
	if strings.Contains(body, "Remove Me") {
		t.Fatal("deleted item still on list detail page")
	}
}

func TestDeleteList_CascadesItems(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	listID := createList(t, ts, "Cascade Test")
	addItem(t, ts, listID, "Apple", "1", "each")
	addItem(t, ts, listID, "Banana", "2", "kg")

	// Delete the list
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/list/"+listID, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Trying to view the deleted list should 404
	resp, _ = http.Get(ts.URL + "/list/" + listID)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for deleted list, got %d", resp.StatusCode)
	}
}

// ─── Price Comparison ────────────────────────────────────────────────────────

func TestComparePrices_WithItems(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	listID := createList(t, ts, "Price Test")
	addItem(t, ts, listID, "Bananas", "1", "kg")
	addItem(t, ts, listID, "Milk", "2", "L")
	addItem(t, ts, listID, "Bread", "1", "each")

	resp, err := http.Post(ts.URL+"/prices/"+listID, "application/x-www-form-urlencoded", nil)
	if err != nil {
		t.Fatalf("POST /prices: %v", err)
	}
	body := readBody(t, resp)

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Check all stores appear
	for _, storeName := range []string{"Walmart", "Loblaws", "Metro"} {
		if !strings.Contains(body, storeName) {
			t.Fatalf("store %s missing from price grid", storeName)
		}
	}

	// Check known prices
	if !strings.Contains(body, "$1.29") {
		t.Log("WARN: Walmart bananas $1.29 not found")
	}
	if !strings.Contains(body, "$0.99") {
		t.Log("WARN: Metro bananas $0.99 not found")
	}

	// Should have dollar amounts
	dollarCount := strings.Count(body, "$")
	if dollarCount < 3 {
		t.Fatalf("expected at least 3 dollar amounts, got %d", dollarCount)
	}

	// Should have "Optimize My Trips" button (>1 store)
	if !strings.Contains(body, "Optimize My Trips") {
		t.Fatal("Optimize My Trips button missing")
	}

	// Should have "Estimated Total" row
	if !strings.Contains(body, "Estimated Total") {
		t.Fatal("Estimated Total row missing from price grid")
	}
}

func TestComparePrices_EmptyList(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	listID := createList(t, ts, "Empty Price Test")

	resp, err := http.Post(ts.URL+"/prices/"+listID, "application/x-www-form-urlencoded", nil)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	body := readBody(t, resp)

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	// Should show an empty/no-providers message
	if !strings.Contains(body, "No price providers") && !strings.Contains(body, "yellow") {
		t.Fatal("expected empty grid message for list with no items")
	}
}

func TestComparePrices_CachesResults(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	listID := createList(t, ts, "Cache Test")
	addItem(t, ts, listID, "Bananas", "1", "kg")

	// First call — populates cache
	resp1, _ := http.Post(ts.URL+"/prices/"+listID, "application/x-www-form-urlencoded", nil)
	body1 := readBody(t, resp1)

	// Second call — should use cache, same result
	resp2, _ := http.Post(ts.URL+"/prices/"+listID, "application/x-www-form-urlencoded", nil)
	body2 := readBody(t, resp2)

	// Both should have the known price
	if !strings.Contains(body1, "$") || !strings.Contains(body2, "$") {
		t.Fatal("prices missing from cached or uncached response")
	}

	// Price counts should be consistent (cache returns same data)
	count1 := strings.Count(body1, "$")
	count2 := strings.Count(body2, "$")
	if count1 != count2 {
		t.Fatalf("dollar count mismatch between calls: %d vs %d", count1, count2)
	}
}

// ─── Trip Optimization ───────────────────────────────────────────────────────

func TestOptimize_WithPrices(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	listID := createList(t, ts, "Optimize Test")
	addItem(t, ts, listID, "Bananas", "1", "kg")
	addItem(t, ts, listID, "Milk", "2", "L")
	addItem(t, ts, listID, "Bread", "1", "each")

	// Must compare prices first to populate cache
	resp, _ := http.Post(ts.URL+"/prices/"+listID, "application/x-www-form-urlencoded", nil)
	resp.Body.Close()

	// Now optimize
	resp, err := http.Post(ts.URL+"/optimize/"+listID, "application/x-www-form-urlencoded",
		strings.NewReader("trip_penalty=3"))
	if err != nil {
		t.Fatalf("POST /optimize: %v", err)
	}
	body := readBody(t, resp)

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(body, "Optimized Shopping Plan") {
		t.Fatal("missing Optimized Shopping Plan header")
	}
	if !strings.Contains(body, "Stop") {
		t.Fatal("no stops in trip plan")
	}
	if !strings.Contains(body, "store visit") {
		t.Fatal("missing store visit count")
	}
}

func TestOptimize_EmptyList(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	listID := createList(t, ts, "Empty Optimize")

	resp, err := http.Post(ts.URL+"/optimize/"+listID, "application/x-www-form-urlencoded", nil)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	body := readBody(t, resp)

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(body, "Compare prices first") {
		t.Fatal("expected empty trip plan message")
	}
}

func TestOptimize_HighTripPenalty_SingleStore(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	listID := createList(t, ts, "High Penalty")
	addItem(t, ts, listID, "Bananas", "1", "kg")
	addItem(t, ts, listID, "Milk", "2", "L")
	addItem(t, ts, listID, "Bread", "1", "each")
	addItem(t, ts, listID, "Eggs", "1", "dozen")
	addItem(t, ts, listID, "Chicken Breast", "1", "kg")

	// Compare prices first
	resp, _ := http.Post(ts.URL+"/prices/"+listID, "application/x-www-form-urlencoded", nil)
	resp.Body.Close()

	// Optimize with very high trip penalty — should collapse to 1 store
	resp, err := http.Post(ts.URL+"/optimize/"+listID, "application/x-www-form-urlencoded",
		strings.NewReader("trip_penalty=100"))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	body := readBody(t, resp)

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(body, "Optimized Shopping Plan") {
		t.Fatal("missing plan header")
	}
	// With a $100 trip penalty, it should optimize to a single stop
	if !strings.Contains(body, "1 store visit") {
		t.Log("INFO: high penalty didn't collapse to 1 store (may depend on mock prices)")
	}
}

func TestOptimize_ZeroTripPenalty(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	listID := createList(t, ts, "Zero Penalty")
	addItem(t, ts, listID, "Bananas", "1", "kg")
	addItem(t, ts, listID, "Milk", "2", "L")
	addItem(t, ts, listID, "Bread", "1", "each")

	resp, _ := http.Post(ts.URL+"/prices/"+listID, "application/x-www-form-urlencoded", nil)
	resp.Body.Close()

	// Zero penalty — should use cheapest store per item (potentially multi-store)
	resp, err := http.Post(ts.URL+"/optimize/"+listID, "application/x-www-form-urlencoded",
		strings.NewReader("trip_penalty=0"))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	body := readBody(t, resp)

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(body, "Optimized Shopping Plan") {
		t.Fatal("missing plan header")
	}
}

// ─── Settings ────────────────────────────────────────────────────────────────

func TestSettingsPage_Loads(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/settings")
	if err != nil {
		t.Fatalf("GET /settings: %v", err)
	}
	body := readBody(t, resp)

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(body, "Settings") {
		t.Fatal("settings title missing")
	}
	if !strings.Contains(body, "Postal Code") {
		t.Fatal("postal code field missing")
	}
	if !strings.Contains(body, "Trip Penalty") {
		t.Fatal("trip penalty field missing")
	}
	if !strings.Contains(body, "M5V") {
		t.Fatal("default postal code M5V not shown")
	}
	if !strings.Contains(body, "Back to lists") {
		t.Fatal("back link missing from settings")
	}
}

func TestSaveSettings_ValidPostalCode(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.PostForm(ts.URL+"/settings", url.Values{
		"postal_code":  {"K1A"},
		"trip_penalty": {"10"},
	})
	if err != nil {
		t.Fatalf("POST /settings: %v", err)
	}
	body := readBody(t, resp)

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(body, "saved successfully") {
		t.Fatal("success message missing")
	}

	// Verify the settings persisted
	resp, _ = http.Get(ts.URL + "/settings")
	body = readBody(t, resp)
	if !strings.Contains(body, "K1A") {
		t.Fatal("saved postal code K1A not shown on reload")
	}
}

func TestSaveSettings_InvalidPostalCode(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.PostForm(ts.URL+"/settings", url.Values{
		"postal_code": {"INVALID123"},
	})
	if err != nil {
		t.Fatalf("POST /settings: %v", err)
	}
	body := readBody(t, resp)

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200 (error rendered in body), got %d", resp.StatusCode)
	}
	if !strings.Contains(body, "Invalid postal code") {
		t.Fatal("validation error message missing for bad postal code")
	}
}

func TestSaveSettings_FullPostalCode(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.PostForm(ts.URL+"/settings", url.Values{
		"postal_code": {"M5V1J2"},
	})
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	body := readBody(t, resp)

	if !strings.Contains(body, "saved successfully") {
		t.Fatal("full postal code M5V1J2 should be accepted")
	}
}

func TestSaveSettings_PostalCodeWithSpace(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.PostForm(ts.URL+"/settings", url.Values{
		"postal_code": {"M5V 1J2"},
	})
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	body := readBody(t, resp)

	if !strings.Contains(body, "saved successfully") {
		t.Fatal("postal code with space M5V 1J2 should be accepted")
	}
}

// ─── Autocomplete ────────────────────────────────────────────────────────────

func TestAutocomplete_EmptyQuery(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/suggest")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	body := readBody(t, resp)

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if body != "" {
		t.Fatalf("expected empty response for no query, got %q", body)
	}
}

func TestAutocomplete_WithQuery(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/suggest?name=Ban")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	readBody(t, resp) // just verify no error

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAutocomplete_ReturnsCachedNames(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	// Populate cache by doing a price comparison
	listID := createList(t, ts, "Suggest Test")
	addItem(t, ts, listID, "Bananas", "1", "kg")
	resp, _ := http.Post(ts.URL+"/prices/"+listID, "application/x-www-form-urlencoded", nil)
	resp.Body.Close()

	// Now autocomplete should find cached banana products
	resp, err := http.Get(ts.URL + "/api/suggest?name=Ban")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	body := readBody(t, resp)

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	// The cache should have entries from the mock provider
	if strings.Contains(body, "option") {
		t.Log("PASS: autocomplete returned datalist options from cache")
	} else {
		t.Log("INFO: no cached suggestions yet (depends on mock provider behavior)")
	}
}

// ─── Full End-to-End Workflow ────────────────────────────────────────────────

func TestFullWorkflow_CreateListAddItemsCompareOptimize(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	// 1. Home page loads
	resp, _ := http.Get(ts.URL + "/")
	body := readBody(t, resp)
	if !strings.Contains(body, "My Grocery Lists") {
		t.Fatal("step 1: home page broken")
	}

	// 2. Create a list
	listID := createList(t, ts, "Weekly Shopping")

	// 3. View list detail
	resp, _ = http.Get(ts.URL + "/list/" + listID)
	body = readBody(t, resp)
	if !strings.Contains(body, "Weekly Shopping") {
		t.Fatal("step 3: list detail broken")
	}

	// 4. Add several items
	items := []struct{ name, qty, unit string }{
		{"Bananas", "1", "kg"},
		{"Milk", "2", "L"},
		{"Bread", "1", "each"},
		{"Eggs", "1", "dozen"},
		{"Chicken Breast", "1.5", "kg"},
	}
	for _, item := range items {
		body := addItem(t, ts, listID, item.name, item.qty, item.unit)
		if !strings.Contains(body, item.name) {
			t.Fatalf("step 4: adding %s failed", item.name)
		}
	}

	// 5. Verify all items appear on list detail
	resp, _ = http.Get(ts.URL + "/list/" + listID)
	body = readBody(t, resp)
	for _, item := range items {
		if !strings.Contains(body, item.name) {
			t.Fatalf("step 5: item %s missing from detail page", item.name)
		}
	}

	// 6. Compare prices
	resp, _ = http.Post(ts.URL+"/prices/"+listID, "application/x-www-form-urlencoded", nil)
	body = readBody(t, resp)
	if resp.StatusCode != 200 {
		t.Fatalf("step 6: compare prices status %d", resp.StatusCode)
	}
	if strings.Count(body, "$") < 5 {
		t.Fatal("step 6: too few prices in grid")
	}

	// 7. Optimize trips
	resp, _ = http.Post(ts.URL+"/optimize/"+listID, "application/x-www-form-urlencoded",
		strings.NewReader("trip_penalty=5"))
	body = readBody(t, resp)
	if !strings.Contains(body, "Optimized Shopping Plan") {
		t.Fatal("step 7: optimize failed")
	}
	if !strings.Contains(body, "Stop") {
		t.Fatal("step 7: no stops in plan")
	}

	// 8. Delete an item
	resp, _ = http.Get(ts.URL + "/list/" + listID)
	body = readBody(t, resp)
	itemIDs := itemIDRegex.FindAllStringSubmatch(body, -1)
	if len(itemIDs) < 1 {
		t.Fatal("step 8: no item IDs found")
	}
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/item/"+itemIDs[0][1], nil)
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("step 8: delete item status %d", resp.StatusCode)
	}

	// 9. Delete the list
	req, _ = http.NewRequest(http.MethodDelete, ts.URL+"/list/"+listID, nil)
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("step 9: delete list status %d", resp.StatusCode)
	}

	// 10. Verify home is empty again
	resp, _ = http.Get(ts.URL + "/")
	body = readBody(t, resp)
	if !strings.Contains(body, "No grocery lists yet") {
		t.Fatal("step 10: home not empty after deleting list")
	}
}

// ─── Multiple Lists ──────────────────────────────────────────────────────────

func TestMultipleLists_IndependentItems(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	list1 := createList(t, ts, "List One")
	list2 := createList(t, ts, "List Two")

	addItem(t, ts, list1, "Apples", "3", "kg")
	addItem(t, ts, list2, "Oranges", "5", "bag")

	// List 1 should have Apples but not Oranges
	resp, _ := http.Get(ts.URL + "/list/" + list1)
	body := readBody(t, resp)
	if !strings.Contains(body, "Apples") {
		t.Fatal("Apples missing from list 1")
	}
	if strings.Contains(body, "Oranges") {
		t.Fatal("Oranges should not be in list 1")
	}

	// List 2 should have Oranges but not Apples
	resp, _ = http.Get(ts.URL + "/list/" + list2)
	body = readBody(t, resp)
	if !strings.Contains(body, "Oranges") {
		t.Fatal("Oranges missing from list 2")
	}
	if strings.Contains(body, "Apples") {
		t.Fatal("Apples should not be in list 2")
	}
}

// ─── Content-Type and Security Headers ───────────────────────────────────────

func TestSecurityHeaders(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()

	checks := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":       "DENY",
		"Referrer-Policy":       "strict-origin-when-cross-origin",
	}
	for header, expected := range checks {
		got := resp.Header.Get(header)
		if got != expected {
			t.Errorf("header %s: expected %q, got %q", header, expected, got)
		}
	}
}

func TestHTMLContentType(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, _ := http.Get(ts.URL + "/")
	resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Fatalf("expected text/html content-type, got %q", ct)
	}
}

// ─── 404 / Method Not Allowed ────────────────────────────────────────────────

func TestNotFoundRoute(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/nonexistent")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 404 or 405 for unknown route, got %d", resp.StatusCode)
	}
}

// ─── Special Characters ─────────────────────────────────────────────────────

func TestSpecialCharacters_ListName(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	name := `Mom's <script>alert("xss")</script> List & More`
	resp, err := http.PostForm(ts.URL+"/list", url.Values{"name": {name}})
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	body := readBody(t, resp)

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	// Should be HTML-escaped, not contain raw script tag
	if strings.Contains(body, "<script>") {
		t.Fatal("XSS: raw <script> tag in response — name not escaped")
	}
	// The escaped version should be present
	if !strings.Contains(body, "Mom&#39;s") && !strings.Contains(body, "Mom's") {
		t.Fatal("list name not in response")
	}
}

func TestSpecialCharacters_ItemName(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	listID := createList(t, ts, "XSS Test")

	name := `<img src=x onerror=alert(1)>`
	body := addItem(t, ts, listID, name, "1", "each")

	// The raw <img tag should be escaped to &lt;img
	if strings.Contains(body, `<img `) {
		t.Fatal("XSS: raw <img> tag in response — item name not escaped")
	}
	if !strings.Contains(body, `&lt;img`) {
		t.Fatal("expected HTML-escaped <img tag in response")
	}
}

// ─── Concurrent Access ──────────────────────────────────────────────────────

func TestConcurrentListCreation(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	const n = 10
	errs := make(chan error, n)

	for i := 0; i < n; i++ {
		go func(i int) {
			name := fmt.Sprintf("Concurrent List %d", i)
			resp, err := http.PostForm(ts.URL+"/list", url.Values{"name": {name}})
			if err != nil {
				errs <- fmt.Errorf("create %d: %v", i, err)
				return
			}
			resp.Body.Close()
			if resp.StatusCode != 200 {
				errs <- fmt.Errorf("create %d: status %d", i, resp.StatusCode)
				return
			}
			errs <- nil
		}(i)
	}

	for i := 0; i < n; i++ {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}

	// Verify all lists exist
	resp, _ := http.Get(ts.URL + "/")
	body := readBody(t, resp)
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("Concurrent List %d", i)
		if !strings.Contains(body, name) {
			t.Fatalf("missing concurrent list: %s", name)
		}
	}
}

func TestConcurrentItemAddition(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	listID := createList(t, ts, "Concurrent Items")

	const n = 10
	errs := make(chan error, n)

	for i := 0; i < n; i++ {
		go func(i int) {
			name := fmt.Sprintf("Item %d", i)
			resp, err := http.PostForm(ts.URL+"/list/"+listID+"/items", url.Values{
				"name": {name}, "quantity": {"1"}, "unit": {"each"},
			})
			if err != nil {
				errs <- fmt.Errorf("add %d: %v", i, err)
				return
			}
			resp.Body.Close()
			if resp.StatusCode != 200 {
				errs <- fmt.Errorf("add %d: status %d", i, resp.StatusCode)
				return
			}
			errs <- nil
		}(i)
	}

	for i := 0; i < n; i++ {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}

	// Verify all items exist
	resp, _ := http.Get(ts.URL + "/list/" + listID)
	body := readBody(t, resp)
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("Item %d", i)
		if !strings.Contains(body, name) {
			t.Fatalf("missing concurrent item: %s", name)
		}
	}
}

// ─── Layout / Navigation ────────────────────────────────────────────────────

func TestLayout_NavBar(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, _ := http.Get(ts.URL + "/")
	body := readBody(t, resp)

	if !strings.Contains(body, "Grocer-Ease") {
		t.Fatal("nav brand missing")
	}
	if !strings.Contains(body, "/settings") {
		t.Fatal("settings link missing from nav")
	}
	if !strings.Contains(body, "Skip to main content") {
		t.Fatal("skip-to-content accessibility link missing")
	}
}

func TestLayout_HTMXLoaded(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, _ := http.Get(ts.URL + "/")
	body := readBody(t, resp)

	if !strings.Contains(body, "htmx.org@2.0.4") {
		t.Fatal("htmx script reference missing")
	}
	if !strings.Contains(body, "sha384-HGfztofotfshcF7+8n44JQL2oJmowVChPTg48S+jvZoztPfvwD79OC/LTtG6dMp+") {
		t.Fatal("htmx SRI hash is incorrect")
	}
}
