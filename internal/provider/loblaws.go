package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/yjmrobert/grocer-ease/internal/model"
)

const (
	pcExpressSearchURL = "https://api.pcexpress.ca/product-facade/v3/products/search"
)

// Supported Loblaw banners for the Site-Banner header.
const (
	BannerLoblaws  = "loblaws"
	BannerMaxi     = "maxi"
	BannerNoFrills = "nofrills"
)

// pcSearchRequest is the JSON body for PC Express product search.
type pcSearchRequest struct {
	Pagination pcPagination `json:"pagination"`
	Banner     string       `json:"banner"`
	CartID     string       `json:"cartId"`
	Lang       string       `json:"lang"`
	Date       string       `json:"date"`
	StoreID    string       `json:"storeId"`
	PCId       *string      `json:"pcId"`
	PickupType string       `json:"pickupType"`
	OfferType  string       `json:"offerType"`
	Term       string       `json:"term"`
}

type pcPagination struct {
	From int `json:"from"`
	Size int `json:"size"`
}

// pcSearchResponse is the search result from PC Express.
type pcSearchResponse struct {
	Results      []pcProductResult `json:"results"`
	Pagination   interface{}       `json:"pagination"`
	TotalResults int               `json:"totalResults"`
}

type pcProductResult struct {
	ProductID   string      `json:"productId"`
	Name        string      `json:"name"`
	Brand       string      `json:"brand"`
	Description string      `json:"description"`
	ImageURL    string      `json:"imageUrl"`
	Prices      pcPriceInfo `json:"prices"`
	PackageSize string      `json:"packageSize"`
}

type pcPriceInfo struct {
	Price           pcPriceDetail  `json:"price"`
	ComparisonPrice *pcPriceDetail `json:"comparisonPrice"`
	WasPrice        *pcPriceDetail `json:"wasPrice"`
}

type pcPriceDetail struct {
	Value    float64 `json:"value"`
	Quantity int     `json:"quantity"`
	Unit     string  `json:"unit"`
}

// LoblawsProvider searches products via the PC Express internal API.
// This covers Loblaws, Maxi, No Frills, and other Loblaw-owned banners.
type LoblawsProvider struct {
	banner    string // "loblaws", "maxi", "nofrills"
	storeID   string // specific store location ID
	apiKey    string // X-Apikey header value (extracted from pcexpress.ca JS bundle)
	storeName string // display name
	client    *http.Client
}

// NewLoblawsProvider creates a provider for a Loblaw banner store.
// apiKey must be extracted from the PC Express website's JavaScript bundle.
// storeID is the specific store location (e.g., "1001"). Use an empty string for default.
func NewLoblawsProvider(banner, storeName, storeID, apiKey string) *LoblawsProvider {
	return &LoblawsProvider{
		banner:    banner,
		storeID:   storeID,
		apiKey:    apiKey,
		storeName: storeName,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (p *LoblawsProvider) Store() string {
	return p.storeName
}

func (p *LoblawsProvider) SearchProduct(ctx context.Context, query string) (*model.PriceResult, error) {
	searchBody := pcSearchRequest{
		Pagination: pcPagination{From: 0, Size: 5},
		Banner:     p.banner,
		Lang:       "en",
		Date:       time.Now().Format("02012006"),
		StoreID:    p.storeID,
		PickupType: "STORE",
		OfferType:  "ALL",
		Term:       query,
	}

	bodyBytes, err := json.Marshal(searchBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, pcExpressSearchURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("X-Apikey", p.apiKey)
	req.Header.Set("Site-Banner", p.banner)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Origin", fmt.Sprintf("https://www.%s.ca", p.banner))
	req.Header.Set("Referer", fmt.Sprintf("https://www.%s.ca/", p.banner))

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pc express search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("pc express returned %d: %s", resp.StatusCode, string(respBody))
	}

	var searchResp pcSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(searchResp.Results) == 0 {
		return nil, nil
	}

	// Use the first (most relevant) result
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
