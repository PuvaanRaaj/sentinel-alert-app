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

	// Parse optional chat_id from request body
	var req struct {
		ChatID string `json:"chat_id"` // Optional: specific chat to purge
	}

	// Try to decode JSON body for chat_id parameter
	_ = json.NewDecoder(r.Body).Decode(&req)

	var err error
	var purgedCount string

	if req.ChatID != "" {
		// Purge alerts for specific chat
		err = h.AlertStore.PurgeAlertsByChat(r.Context(), req.ChatID)
		purgedCount = "chat-specific"

		if actorID, _, _ := GetCurrentUser(r); actorID != 0 {
			meta, _ := json.Marshal(map[string]string{"chat_id": req.ChatID})
			_ = h.AdminStore.InsertAudit(r.Context(), actorID, "purge_alerts_by_chat", "system", 0, string(meta))
		}
	} else {
		// Purge all alerts
		err = h.AlertStore.PurgeAllAlerts(r.Context())
		purgedCount = "all"

		if actorID, _, _ := GetCurrentUser(r); actorID != 0 {
			_ = h.AdminStore.InsertAudit(r.Context(), actorID, "purge_alerts", "system", 0, "{}")
		}
	}

	if err != nil {
		log.Printf("Failed to purge alerts: %v", err)
		http.Error(w, "Failed to purge alerts", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"scope":   purgedCount,
	})
}
