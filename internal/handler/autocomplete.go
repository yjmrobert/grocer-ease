package handler

import (
	"fmt"
	"net/http"

	"github.com/yjmrobert/grocer-ease/internal/store"
)

type AutocompleteHandler struct {
	cacheStore *store.PriceCacheStore
}

func NewAutocompleteHandler(cs *store.PriceCacheStore) *AutocompleteHandler {
	return &AutocompleteHandler{cacheStore: cs}
}

// HandleSuggest returns <option> elements for a datalist, based on cached product names.
func (h *AutocompleteHandler) HandleSuggest(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("name")
	if q == "" {
		w.Header().Set("Content-Type", "text/html")
		return
	}

	names, err := h.cacheStore.GetProductNames(q, 10)
	if err != nil {
		w.Header().Set("Content-Type", "text/html")
		return
	}

	w.Header().Set("Content-Type", "text/html")
	for _, name := range names {
		fmt.Fprintf(w, `<option value="%s"></option>`, name)
	}
}
