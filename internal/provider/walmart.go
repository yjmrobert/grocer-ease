package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/yjmrobert/grocer-ease/internal/model"
)

const walmartSearchURL = "https://www.walmart.ca/search"

// WalmartProvider scrapes product search results from Walmart.ca.
// Note: Walmart.ca uses CloudFlare bot protection. This provider attempts
// direct HTTP requests with browser-like headers. If blocked (403), you may
// need to switch to a headless browser approach using Rod.
type WalmartProvider struct {
	client *http.Client
}

func NewWalmartProvider() *WalmartProvider {
	return &WalmartProvider{
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (p *WalmartProvider) Store() string {
	return "Walmart"
}

func (p *WalmartProvider) SearchProduct(ctx context.Context, query string) (*model.PriceResult, error) {
	searchURL := fmt.Sprintf("%s?%s", walmartSearchURL, url.Values{
		"q": {query},
	}.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Mimic a real browser request
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-CA,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("walmart search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("walmart.ca returned 403 (CloudFlare block) — consider using a headless browser provider")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("walmart.ca returned %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}

	return parseWalmartSearchResults(doc, query)
}

func parseWalmartSearchResults(doc *goquery.Document, query string) (*model.PriceResult, error) {
	// Walmart.ca uses various CSS selectors for product cards.
	// These selectors may change over time as Walmart updates their site.
	// Common patterns observed:
	//   - [data-testid="product-tile"] for product cards
	//   - Product title in an anchor or span inside the card
	//   - Price in elements with data-testid="price" or similar

	var result *model.PriceResult

	// Try multiple known selector patterns
	selectors := []struct {
		card  string
		title string
		price string
		image string
		link  string
	}{
		{
			card:  "[data-testid='product-tile']",
			title: "[data-testid='product-title']",
			price: "[data-testid='price'] span",
			image: "img[data-testid='product-image']",
			link:  "a[data-testid='product-title']",
		},
		{
			card:  ".product-tile",
			title: ".product-title",
			price: ".price-current",
			image: ".product-image img",
			link:  ".product-title a",
		},
		{
			card:  "[data-automation='product']",
			title: "[data-automation='name']",
			price: "[data-automation='current-price']",
			image: "[data-automation='image'] img",
			link:  "[data-automation='name'] a",
		},
	}

	for _, sel := range selectors {
		cards := doc.Find(sel.card)
		if cards.Length() == 0 {
			continue
		}

		// Use the first product card
		card := cards.First()

		title := strings.TrimSpace(card.Find(sel.title).Text())
		priceText := strings.TrimSpace(card.Find(sel.price).Text())
		imageURL, _ := card.Find(sel.image).Attr("src")
		linkHref, _ := card.Find(sel.link).Attr("href")

		if title == "" || priceText == "" {
			continue
		}

		price := parsePrice(priceText)
		if price == 0 {
			continue
		}

		productURL := ""
		if linkHref != "" {
			if strings.HasPrefix(linkHref, "/") {
				productURL = "https://www.walmart.ca" + linkHref
			} else {
				productURL = linkHref
			}
		}

		result = &model.PriceResult{
			Store:       "Walmart",
			ProductName: title,
			Price:       price,
			Unit:        "each",
			ImageURL:    imageURL,
			URL:         productURL,
			Confidence:  "exact",
		}
		break
	}

	if result == nil {
		// Check if we hit a CAPTCHA or bot detection page
		pageText := doc.Text()
		if strings.Contains(strings.ToLower(pageText), "captcha") ||
			strings.Contains(strings.ToLower(pageText), "robot") ||
			strings.Contains(strings.ToLower(pageText), "verify") {
			return nil, fmt.Errorf("walmart.ca bot detection triggered — headless browser required")
		}
		return nil, nil // genuinely not found
	}

	return result, nil
}
