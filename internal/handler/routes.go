package handler

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/yjmrobert/grocer-ease/internal/service"
	"github.com/yjmrobert/grocer-ease/internal/store"
)

func NewRouter(listStore *store.ListStore, priceService *service.PriceService, cacheStore *store.PriceCacheStore, settings *AppSettings) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	lh := NewListHandler(listStore)
	ph := NewPriceHandler(listStore, priceService)
	oh := NewOptimizeHandler(listStore, priceService)
	ah := NewAutocompleteHandler(cacheStore)
	sh := NewSettingsHandler(settings)

	// Pages
	r.Get("/", lh.HandleHome)
	r.Get("/list/{id}", lh.HandleListDetail)
	r.Get("/settings", sh.HandleSettingsPage)

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

	// API
	r.Get("/api/suggest", ah.HandleSuggest)

	// Settings
	r.Post("/settings", sh.HandleSaveSettings)

	return r
}
