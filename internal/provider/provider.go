package provider

import (
	"context"

	"github.com/yjmrobert/grocer-ease/internal/model"
)

type PriceProvider interface {
	Store() string
	SearchProduct(ctx context.Context, query string) (*model.PriceResult, error)
}
