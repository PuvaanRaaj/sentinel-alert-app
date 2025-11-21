package handlers

import (
	"encoding/json"
	"net/http"
)

// GetChatsPublicHandler returns all chats (for main dashboard)
func (h *Handler) GetChatsPublicHandler(w http.ResponseWriter, r *http.Request) {
	chats, err := h.AdminStore.GetChats(r.Context())
	if err != nil {
		http.Error(w, "Failed to get chats", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"chats": chats})
}
