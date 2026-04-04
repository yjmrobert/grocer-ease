package handler

import (
	"log"
	"net/http"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/yjmrobert/grocer-ease/internal/service"
	"github.com/yjmrobert/grocer-ease/internal/store"
	"github.com/yjmrobert/grocer-ease/internal/view"
)

type PriceHandler struct {
	listStore    *store.ListStore
	priceService *service.PriceService
}

func NewPriceHandler(ls *store.ListStore, ps *service.PriceService) *PriceHandler {
	return &PriceHandler{
		listStore:    ls,
		priceService: ps,
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

	if len(items) == 0 {
		templ.Handler(view.PriceGridEmpty()).ServeHTTP(w, r)
		return
	}

	if !h.priceService.HasProviders() {
		templ.Handler(view.PriceGridEmpty()).ServeHTTP(w, r)
		return
	}

	gridData := h.priceService.ComparePrices(r.Context(), items)
	templ.Handler(view.PriceResults(gridData, listID)).ServeHTTP(w, r)
}
