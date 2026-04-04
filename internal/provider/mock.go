package provider

import (
	"context"
	"math/rand"
	"strings"
	"unicode"

	"github.com/yjmrobert/grocer-ease/internal/model"
)

// MockProvider returns fake prices for testing when external APIs are unavailable.
// Not intended for production use.
type MockProvider struct {
	storeName string
	prices    map[string]float64
}

func NewMockProvider(storeName string, prices map[string]float64) *MockProvider {
	return &MockProvider{storeName: storeName, prices: prices}
}

func (p *MockProvider) Store() string {
	return p.storeName
}

func (p *MockProvider) SearchProduct(_ context.Context, query string) (*model.PriceResult, error) {
	q := strings.ToLower(query)
	if price, ok := p.prices[q]; ok {
		return &model.PriceResult{
			Store:       p.storeName,
			ProductName: capitalize(q),
			Price:       price,
			Unit:        "each",
			Confidence:  "exact",
		}, nil
	}
	// Small chance of random match for testing
	if rand.Float64() < 0.3 {
		return nil, nil
	}
	return &model.PriceResult{
		Store:       p.storeName,
		ProductName: capitalize(q),
		Price:       1.0 + rand.Float64()*10.0,
		Unit:        "each",
		Confidence:  "partial",
	}, nil
}

// capitalize uppercases the first letter of each word.
func capitalize(s string) string {
	prev := ' '
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(rune(prev)) || prev == ' ' {
			prev = r
			return unicode.ToUpper(r)
		}
		prev = r
		return r
	}, s)
}
