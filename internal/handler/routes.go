package handler

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/yjmrobert/grocer-ease/internal/service"
	"github.com/yjmrobert/grocer-ease/internal/store"
)

func NewRouter(listStore *store.ListStore, priceService *service.PriceService) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	lh := NewListHandler(listStore)
	ph := NewPriceHandler(listStore, priceService)
	oh := NewOptimizeHandler(listStore, priceService)

	// Pages
	r.Get("/", lh.HandleHome)
	r.Get("/list/{id}", lh.HandleListDetail)

	// List CRUD (HTMX)
	r.Post("/list", lh.HandleCreateList)
	r.Delete("/list/{id}", lh.HandleDeleteList)

	// Item CRUD (HTMX)
	r.Post("/list/{id}/items", lh.HandleAddItem)
	r.Delete("/item/{id}", lh.HandleDeleteItem)

	// Price comparison (HTMX)
	r.Post("/prices/{listId}", ph.HandleComparePrices)

	// Trip optimization (HTMX)
	r.Post("/optimize/{listId}", oh.HandleOptimize)

	return r
}
