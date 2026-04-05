package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/yjmrobert/grocer-ease/internal/service"
	"github.com/yjmrobert/grocer-ease/internal/store"
)

// securityHeaders sets standard security response headers.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

func NewRouter(listStore *store.ListStore, priceService *service.PriceService, cacheStore *store.PriceCacheStore, settingsStore *store.SettingsStore) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(securityHeaders)
	r.Use(middleware.Timeout(30 * time.Second))

	lh := NewListHandler(listStore)
	ph := NewPriceHandler(listStore, priceService)
	oh := NewOptimizeHandler(listStore, priceService)
	ah := NewAutocompleteHandler(cacheStore)
	sh := NewSettingsHandler(settingsStore)

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
