package handlers

import (
	"encoding/json"
	"log"
	"net/http"
)

// === Admin Purge Handler ===

func (h *Handler) PurgeAlertsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := h.AlertStore.PurgeAllAlerts(r.Context()); err != nil {
		log.Printf("Failed to purge alerts: %v", err)
		http.Error(w, "Failed to purge alerts", http.StatusInternalServerError)
		return
	}

	if actorID, _, _ := GetCurrentUser(r); actorID != 0 {
		_ = h.AdminStore.InsertAudit(r.Context(), actorID, "purge_alerts", "system", 0, "{}")
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}
