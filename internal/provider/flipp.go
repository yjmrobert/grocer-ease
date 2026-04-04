package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/yjmrobert/grocer-ease/internal/model"
)

const (
	flippSearchURL = "https://backflipp.wishabi.com/flipp/items/search"
	flippItemURL   = "https://backflipp.wishabi.com/flipp/items"
)

// flippSearchResponse represents the top-level Flipp search API response.
type flippSearchResponse struct {
	Items []flippSearchItem `json:"items"`
}

type flippSearchItem struct {
	FlyerItemID int    `json:"flyer_item_id"`
	MerchantID  int    `json:"merchant_id"`
	Merchant    string `json:"merchant"`
	Name        string `json:"name"`
	Price       string `json:"price"`
	PrePriceText string `json:"pre_price_text"`
	PostPriceText string `json:"post_price_text"`
}

// flippItemDetail represents a single item detail from Flipp.
type flippItemDetail struct {
	ID            int     `json:"id"`
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	Price         float64 `json:"current_price"`
	PrePriceText  string  `json:"pre_price_text"`
	PostPriceText string  `json:"post_price_text"`
	CutoutImageURL string `json:"cutout_image_url"`
	Merchant      string  `json:"merchant"`
	ValidFrom     string  `json:"valid_from"`
	ValidTo       string  `json:"valid_to"`
	FlyerID       int     `json:"flyer_id"`
}

// FlippProvider fetches grocery deal prices from Flipp flyer data.
// Flipp aggregates weekly flyer deals from Canadian retailers.
type FlippProvider struct {
	postalCode string
	storeName  string // filter to a specific merchant, e.g. "Walmart", "Loblaws", "Maxi"
	client     *http.Client
}

// NewFlippProvider creates a Flipp provider filtered to a specific store.
// postalCode should be a Canadian postal code (e.g., "M5V1J2" or "M5V").
// storeName filters results to a specific merchant (case-insensitive partial match).
// If storeName is empty, returns the cheapest result from any store.
func NewFlippProvider(storeName, postalCode string) *FlippProvider {
	return &FlippProvider{
		postalCode: postalCode,
		storeName:  storeName,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (p *FlippProvider) Store() string {
	if p.storeName == "" {
		return "Flipp (Best Deal)"
	}
	return p.storeName + " (Flipp)"
}

func (p *FlippProvider) SearchProduct(ctx context.Context, query string) (*model.PriceResult, error) {
	// Search Flipp for the item
	searchURL := fmt.Sprintf("%s?%s", flippSearchURL, url.Values{
		"q":           {query},
		"postal_code": {p.postalCode},
	}.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("flipp search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("flipp search returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var searchResp flippSearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, fmt.Errorf("decode search response: %w", err)
	}

	if len(searchResp.Items) == 0 {
		return nil, nil // not found
	}

	// Filter by store name if specified
	var match *flippSearchItem
	for i, item := range searchResp.Items {
		if p.storeName == "" {
			match = &searchResp.Items[i]
			break
		}
		if strings.Contains(strings.ToLower(item.Merchant), strings.ToLower(p.storeName)) {
			match = &searchResp.Items[i]
			break
		}
	}
	if match == nil {
		return nil, nil // not found at this store
	}

	// Fetch full item details
	detail, err := p.fetchItemDetail(ctx, match.FlyerItemID)
	if err != nil {
		log.Printf("flipp item detail fetch failed for %d, using search data: %v", match.FlyerItemID, err)
		// Fall back to search data — try to parse the price string
		return parseFlippSearchItem(match), nil
	}

	return &model.PriceResult{
		Store:       p.Store(),
		ProductName: detail.Name,
		Price:       detail.Price,
		Unit:        parseFlippUnit(detail.PrePriceText, detail.PostPriceText),
		ImageURL:    detail.CutoutImageURL,
		Confidence:  "partial", // flyer prices are sale prices, not regular
	}, nil
}

func (p *FlippProvider) fetchItemDetail(ctx context.Context, itemID int) (*flippItemDetail, error) {
	itemURL := fmt.Sprintf("%s/%d", flippItemURL, itemID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, itemURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("item detail returned %d", resp.StatusCode)
	}

	var detail flippItemDetail
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		return nil, err
	}
	return &detail, nil
}

func parseFlippSearchItem(item *flippSearchItem) *model.PriceResult {
	// Try to parse price from the string (e.g., "$4.99", "2/$5.00", "4.99/lb")
	price := parsePrice(item.Price)
	if price == 0 {
		return nil
	}
	return &model.PriceResult{
		Store:       item.Merchant,
		ProductName: item.Name,
		Price:       price,
		Unit:        parseFlippUnit(item.PrePriceText, item.PostPriceText),
		Confidence:  "partial",
	}
}

func parseFlippUnit(prePriceText, postPriceText string) string {
	combined := strings.ToLower(prePriceText + " " + postPriceText)
	switch {
	case strings.Contains(combined, "/lb"):
		return "per lb"
	case strings.Contains(combined, "/kg"):
		return "per kg"
	case strings.Contains(combined, "/100g"):
		return "per 100g"
	case strings.Contains(combined, "/l"):
		return "per L"
	default:
		return "each"
	}
}
