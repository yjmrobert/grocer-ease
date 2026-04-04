package handler

import (
	"log"
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/yjmrobert/grocer-ease/internal/store"
	"github.com/yjmrobert/grocer-ease/internal/view"
)

type ListHandler struct {
	store *store.ListStore
}

func NewListHandler(s *store.ListStore) *ListHandler {
	return &ListHandler{store: s}
}

func (h *ListHandler) HandleHome(w http.ResponseWriter, r *http.Request) {
	lists, err := h.store.GetAllLists()
	if err != nil {
		log.Printf("error getting lists: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	templ.Handler(view.Home(lists)).ServeHTTP(w, r)
}

func (h *ListHandler) HandleCreateList(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}
	list, err := h.store.CreateList(name)
	if err != nil {
		log.Printf("error creating list: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	templ.Handler(view.ListCard(*list)).ServeHTTP(w, r)
}

func (h *ListHandler) HandleDeleteList(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.store.DeleteList(id); err != nil {
		log.Printf("error deleting list: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *ListHandler) HandleListDetail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	list, err := h.store.GetList(id)
	if err != nil {
		log.Printf("error getting list: %v", err)
		http.Error(w, "List not found", http.StatusNotFound)
		return
	}
	items, err := h.store.GetItems(id)
	if err != nil {
		log.Printf("error getting items: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	templ.Handler(view.ListDetail(list, items)).ServeHTTP(w, r)
}

func (h *ListHandler) HandleAddItem(w http.ResponseWriter, r *http.Request) {
	listID := chi.URLParam(r, "id")
	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}

	quantity := 1.0
	if q := r.FormValue("quantity"); q != "" {
		if parsed, err := strconv.ParseFloat(q, 64); err == nil {
			quantity = parsed
		}
	}
	unit := r.FormValue("unit")
	if unit == "" {
		unit = "each"
	}

	item, err := h.store.AddItem(listID, name, quantity, unit)
	if err != nil {
		log.Printf("error adding item: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	templ.Handler(view.ItemRow(*item)).ServeHTTP(w, r)
}

func (h *ListHandler) HandleDeleteItem(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.store.DeleteItem(id); err != nil {
		log.Printf("error deleting item: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
