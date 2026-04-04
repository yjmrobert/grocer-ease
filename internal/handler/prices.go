package handler

import (
	"log"
	"net/http"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/yjmrobert/grocer-ease/internal/provider"
	"github.com/yjmrobert/grocer-ease/internal/store"
	"github.com/yjmrobert/grocer-ease/internal/view"
)

type PriceHandler struct {
	listStore *store.ListStore
	providers []provider.PriceProvider
}

func NewPriceHandler(ls *store.ListStore, providers []provider.PriceProvider) *PriceHandler {
	return &PriceHandler{
		listStore: ls,
		providers: providers,
	}
}

func (h *PriceHandler) HandleComparePrices(w http.ResponseWriter, r *http.Request) {
	listID := chi.URLParam(r, "listId")

	items, err := h.listStore.GetItems(listID)
	if err != nil {
		log.Printf("error getting items: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if len(h.providers) == 0 {
		templ.Handler(view.PriceGridEmpty()).ServeHTTP(w, r)
		return
	}

	// TODO: Phase 2 — query providers for prices
	// For now, show the empty message
	templ.Handler(view.PriceGridEmpty()).ServeHTTP(w, r)
	_ = items // will be used when providers are implemented
}
