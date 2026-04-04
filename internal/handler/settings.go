package handler

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/yjmrobert/grocer-ease/internal/view"
)

// AppSettings holds runtime-configurable app settings.
type AppSettings struct {
	PostalCode  string
	TripPenalty float64
}

type SettingsHandler struct {
	settings *AppSettings
}

func NewSettingsHandler(settings *AppSettings) *SettingsHandler {
	return &SettingsHandler{settings: settings}
}

func (h *SettingsHandler) HandleSettingsPage(w http.ResponseWriter, r *http.Request) {
	templ.Handler(view.SettingsPage(h.settings.PostalCode, h.settings.TripPenalty)).ServeHTTP(w, r)
}

func (h *SettingsHandler) HandleSaveSettings(w http.ResponseWriter, r *http.Request) {
	postalCode := r.FormValue("postal_code")
	if postalCode != "" {
		h.settings.PostalCode = postalCode
	}

	templ.Handler(view.SettingsSaved()).ServeHTTP(w, r)
}
