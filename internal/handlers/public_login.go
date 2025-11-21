package handlers

import (
	"encoding/json"
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

// PublicLoginHandler handles login for main dashboard (all users)
func (h *Handler) PublicLoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Get user from database
	user, err := h.AdminStore.GetUserByUsername(r.Context(), req.Username)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid credentials"})
		return
	}

	// Check password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid credentials"})
		return
	}

	// Check if 2FA is enabled
	if user.TOTPEnabled {
		// Return 2FA required response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"requires_2fa": true,
			"user_id":      user.ID,
			"totp_enabled": true,
		})
		return
	}

	// Get user's allowed chats
	var allowedChats []any
	if user.Role == "admin" || user.Role == "developer" {
		// Admin/developer see all chats
		chats, _ := h.AdminStore.GetChats(r.Context())
		for _, chat := range chats {
			allowedChats = append(allowedChats, map[string]any{
				"id":      chat.ID,
				"chat_id": chat.ChatID,
				"name":    chat.Name,
				"bot_id":  chat.BotID,
			})
		}
	} else {
		// Regular user sees only assigned chats
		chats, _ := h.AdminStore.GetUserChats(r.Context(), user.ID)
		for _, chat := range chats {
			allowedChats = append(allowedChats, map[string]any{
				"id":      chat.ID,
				"chat_id": chat.ChatID,
				"name":    chat.Name,
				"bot_id":  chat.BotID,
			})
		}
	}

	// Create session
	session, _ := sessionStore.Get(r, sessionName)
	session.Values["user_id"] = user.ID
	session.Values["username"] = user.Username
	session.Values["role"] = user.Role
	session.Save(r, w)

	// Return user info (without password hash)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"user": map[string]any{
			"id":           user.ID,
			"username":     user.Username,
			"role":         user.Role,
			"totp_enabled": user.TOTPEnabled,
		},
		"allowed_chats": allowedChats,
	})
}
