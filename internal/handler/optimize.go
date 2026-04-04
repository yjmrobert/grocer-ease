package handler

import (
	"log"
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/yjmrobert/grocer-ease/internal/service"
	"github.com/yjmrobert/grocer-ease/internal/store"
	"github.com/yjmrobert/grocer-ease/internal/view"
)

type OptimizeHandler struct {
	listStore    *store.ListStore
	priceService *service.PriceService
}

func NewOptimizeHandler(ls *store.ListStore, ps *service.PriceService) *OptimizeHandler {
	return &OptimizeHandler{
		listStore:    ls,
		priceService: ps,
	}
}

func (h *OptimizeHandler) HandleOptimize(w http.ResponseWriter, r *http.Request) {
	listID := chi.URLParam(r, "listId")

	items, err := h.listStore.GetItems(listID)
	if err != nil {
		log.Printf("error getting items: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if len(items) == 0 || !h.priceService.HasProviders() {
		templ.Handler(view.TripPlanEmpty()).ServeHTTP(w, r)
		return
	}

	// Get trip penalty from form (defaults to $5)
	tripPenalty := 5.0
	if penaltyStr := r.FormValue("trip_penalty"); penaltyStr != "" {
		if p, err := strconv.ParseFloat(penaltyStr, 64); err == nil && p >= 0 {
			tripPenalty = p
		}
	}

	// Re-fetch prices (uses cache) and optimize
	gridData := h.priceService.ComparePrices(r.Context(), items)
	plan := service.OptimizeTripPlan(gridData, items, tripPenalty)

	if len(plan.Trips) == 0 {
		templ.Handler(view.TripPlanEmpty()).ServeHTTP(w, r)
		return
	}

	templ.Handler(view.TripPlan(plan)).ServeHTTP(w, r)
}
