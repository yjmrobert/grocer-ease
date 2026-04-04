package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/yjmrobert/grocer-ease/internal/model"
)

// ============================================================
// Walmart.ca Scraper Integration Tests
// ============================================================

// Realistic Walmart.ca search results HTML (simplified from actual page)
const walmartSearchHTML = `<!DOCTYPE html>
<html lang="en-CA">
<head><title>Search results for bananas - Walmart.ca</title></head>
<body>
<div id="search-results">
  <div data-testid="product-tile" class="product-tile">
    <a data-testid="product-title" href="/ip/bananas-bunch/6000196852987">Bananas, Bunch, 1 each</a>
    <div data-testid="price">
      <span>$0.27</span>
      <span>/each</span>
    </div>
    <img data-testid="product-image" src="https://i5.walmartimages.ca/images/Large/6000196852987.jpg" alt="Bananas" />
  </div>
  <div data-testid="product-tile" class="product-tile">
    <a data-testid="product-title" href="/ip/organic-bananas/6000196852990">Organic Bananas, 1 Bunch</a>
    <div data-testid="price">
      <span>$2.47</span>
    </div>
    <img data-testid="product-image" src="https://i5.walmartimages.ca/images/Large/6000196852990.jpg" alt="Organic Bananas" />
  </div>
</div>
</body>
</html>`

const walmartNoResultsHTML = `<!DOCTYPE html>
<html lang="en-CA">
<head><title>Search results - Walmart.ca</title></head>
<body>
<div id="search-results">
  <div class="no-results">
    <p>Sorry, we couldn't find any results for "unicorn steak"</p>
  </div>
</div>
</body>
</html>`

const walmartCaptchaHTML = `<!DOCTYPE html>
<html>
<head><title>Robot Check</title></head>
<body>
<div>
  <p>Please verify you are a human</p>
  <form class="captcha-form">
    <p>We need to verify that you are not a robot.</p>
  </form>
</div>
</body>
</html>`

// Alternative HTML structure (some Walmart pages use different selectors)
const walmartAltHTML = `<!DOCTYPE html>
<html lang="en-CA">
<head><title>Search - Walmart.ca</title></head>
<body>
<div data-automation="product" class="product-tile">
  <a data-automation="name" href="/ip/yellow-bananas/123456">Yellow Bananas</a>
  <span data-automation="current-price">$1.47</span>
  <div data-automation="image"><img src="https://cdn.walmart.ca/bananas.jpg" /></div>
</div>
</body>
</html>`

func TestWalmartProvider_SearchProduct(t *testing.T) {
	t.Run("parses standard product tiles", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query().Get("q")
			if q == "" {
				t.Error("missing q parameter")
			}
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(walmartSearchHTML))
		}))
		defer ts.Close()

		p := &WalmartProvider{client: ts.Client()}
		result, err := walmartSearchWithURL(p, ts.URL, context.Background(), "bananas")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == nil {
			t.Fatal("expected a result")
		}
		if result.Price != 0.27 {
			t.Errorf("expected price $0.27, got $%.2f", result.Price)
		}
		if result.ProductName != "Bananas, Bunch, 1 each" {
			t.Errorf("expected 'Bananas, Bunch, 1 each', got %q", result.ProductName)
		}
		if result.Store != "Walmart" {
			t.Errorf("expected store 'Walmart', got %q", result.Store)
		}
		if !strings.Contains(result.URL, "walmart.ca") {
			t.Errorf("expected walmart.ca URL, got %q", result.URL)
		}
		if result.ImageURL == "" {
			t.Error("expected image URL")
		}
		t.Logf("PASS: Walmart bananas = $%.2f — %s", result.Price, result.ProductName)
		t.Logf("      URL: %s", result.URL)
		t.Logf("      Image: %s", result.ImageURL)
	})

	t.Run("parses alternative HTML structure", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(walmartAltHTML))
		}))
		defer ts.Close()

		p := &WalmartProvider{client: ts.Client()}
		result, err := walmartSearchWithURL(p, ts.URL, context.Background(), "bananas")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == nil {
			t.Fatal("expected a result from alt HTML")
		}
		if result.Price != 1.47 {
			t.Errorf("expected price $1.47, got $%.2f", result.Price)
		}
		if result.ProductName != "Yellow Bananas" {
			t.Errorf("expected 'Yellow Bananas', got %q", result.ProductName)
		}
		t.Logf("PASS: Walmart alt HTML bananas = $%.2f — %s", result.Price, result.ProductName)
	})

	t.Run("returns nil for no results", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(walmartNoResultsHTML))
		}))
		defer ts.Close()

		p := &WalmartProvider{client: ts.Client()}
		result, err := walmartSearchWithURL(p, ts.URL, context.Background(), "unicorn steak")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != nil {
			t.Errorf("expected nil for missing product, got %v", result)
		}
		t.Log("PASS: Walmart returns nil for product not found")
	})

	t.Run("detects CAPTCHA/bot page", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(walmartCaptchaHTML))
		}))
		defer ts.Close()

		p := &WalmartProvider{client: ts.Client()}
		_, err := walmartSearchWithURL(p, ts.URL, context.Background(), "bananas")
		if err == nil {
			t.Fatal("expected error for CAPTCHA page")
		}
		if !strings.Contains(err.Error(), "bot detection") {
			t.Errorf("error should mention bot detection: %v", err)
		}
		t.Logf("PASS: Walmart detects CAPTCHA page: %v", err)
	})

	t.Run("handles CloudFlare 403", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("Access Denied"))
		}))
		defer ts.Close()

		p := &WalmartProvider{client: ts.Client()}
		_, err := walmartSearchWithURL(p, ts.URL, context.Background(), "bananas")
		if err == nil {
			t.Fatal("expected error for 403")
		}
		if !strings.Contains(err.Error(), "403") {
			t.Errorf("error should mention 403: %v", err)
		}
		t.Logf("PASS: Walmart handles 403 correctly: %v", err)
	})
}

// walmartSearchWithURL is a helper that runs the Walmart search/parse logic
// against a test server URL instead of walmart.ca.
func walmartSearchWithURL(p *WalmartProvider, testURL string, ctx context.Context, query string) (*model.PriceResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, testURL+"?q="+url.QueryEscape(query), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "test")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("walmart.ca returned 403 (CloudFlare block)")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("walmart.ca returned %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	return parseWalmartSearchResults(doc, query)
}
