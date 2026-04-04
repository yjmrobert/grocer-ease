package handler

import (
	"net/http"
	"regexp"
	"strconv"

	"github.com/a-h/templ"
	"github.com/yjmrobert/grocer-ease/internal/store"
	"github.com/yjmrobert/grocer-ease/internal/view"
)

var postalCodeRegex = regexp.MustCompile(`^[A-Za-z]\d[A-Za-z]\s?\d?[A-Za-z]?\d?$`)

type SettingsHandler struct {
	settingsStore *store.SettingsStore
}

func NewSettingsHandler(ss *store.SettingsStore) *SettingsHandler {
	return &SettingsHandler{settingsStore: ss}
}

func (h *SettingsHandler) HandleSettingsPage(w http.ResponseWriter, r *http.Request) {
	postalCode := h.settingsStore.Get("postal_code", "")
	tripPenaltyStr := h.settingsStore.Get("trip_penalty", "5")
	tripPenalty, _ := strconv.ParseFloat(tripPenaltyStr, 64)
	if tripPenalty == 0 {
		tripPenalty = 5.0
	}

	templ.Handler(view.SettingsPage(postalCode, tripPenalty)).ServeHTTP(w, r)
}

func (h *SettingsHandler) HandleSaveSettings(w http.ResponseWriter, r *http.Request) {
	postalCode := r.FormValue("postal_code")
	if postalCode != "" {
		if !postalCodeRegex.MatchString(postalCode) {
			templ.Handler(view.SettingsError("Invalid postal code format. Use Canadian format like M5V or M5V1J2.")).ServeHTTP(w, r)
			return
		}
		h.settingsStore.Set("postal_code", postalCode)
	}

	tripPenaltyStr := r.FormValue("trip_penalty")
	if tripPenaltyStr != "" {
		if p, err := strconv.ParseFloat(tripPenaltyStr, 64); err == nil && p >= 0 {
			h.settingsStore.Set("trip_penalty", tripPenaltyStr)
		}
	}

	templ.Handler(view.SettingsSaved()).ServeHTTP(w, r)
}
