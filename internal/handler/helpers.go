package handler

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/yjmrobert/grocer-ease/internal/view"
)

// renderError writes a styled HTML error response suitable for HTMX swaps.
func renderError(w http.ResponseWriter, r *http.Request, msg string, status int) {
	w.WriteHeader(status)
	templ.Handler(view.ErrorMessage(msg)).ServeHTTP(w, r)
}
