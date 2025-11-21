package handlers

import (
	"encoding/json"
	"incident-viewer-go/internal/models"
	"log"
	"net/http"
)

// Generate2FAHandler generates a new TOTP secret and QR code
func (h *Handler) Generate2FAHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		UserID int `json:"user_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Get user
	user, err := h.AdminStore.GetUser(r.Context(), req.UserID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Generate TOTP key
	key, err := models.GenerateTOTPSecret(user.Username, "Incident Viewer")
	if err != nil {
		http.Error(w, "Failed to generate secret", http.StatusInternalServerError)
		return
	}

	// Generate QR code
	qrCode, err := models.GenerateQRCode(key)
	if err != nil {
		http.Error(w, "Failed to generate QR code", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"secret":  key.Secret(),
		"qr_code": "data:image/png;base64," + qrCode,
		"issuer":  "Incident Viewer",
		"account": user.Username,
	})
}

// Enable2FAHandler verifies the TOTP code and enables 2FA
func (h *Handler) Enable2FAHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		UserID int    `json:"user_id"`
		Secret string `json:"secret"`
		Code   string `json:"code"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Verify the code
	if !models.VerifyTOTPCode(req.Secret, req.Code) {
		http.Error(w, "Invalid verification code", http.StatusUnauthorized)
		return
	}

	// Enable 2FA
	if err := h.AdminStore.UpdateUser2FA(r.Context(), req.UserID, req.Secret, true); err != nil {
		log.Printf("Failed to enable 2FA: %v", err)
		http.Error(w, "Failed to enable 2FA", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"success": true, "message": "2FA enabled successfully"})
}

// Disable2FAHandler disables 2FA for a user (own or admin action)
func (h *Handler) Disable2FAHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		UserID int `json:"user_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Check if user is admin - they cannot disable their own 2FA
	user, err := h.AdminStore.GetUser(r.Context(), req.UserID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	if user.Role == "admin" {
		http.Error(w, "Admins cannot disable their own 2FA", http.StatusForbidden)
		return
	}

	// Disable 2FA
	if err := h.AdminStore.Disable2FA(r.Context(), req.UserID); err != nil {
		log.Printf("Failed to disable 2FA: %v", err)
		http.Error(w, "Failed to disable 2FA", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"success": true, "message": "2FA disabled successfully"})
}

// AdminDisable2FAHandler allows admins to disable 2FA for any user
func (h *Handler) AdminDisable2FAHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		UserID int `json:"user_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Admin can disable any user's 2FA (for account recovery)
	if err := h.AdminStore.Disable2FA(r.Context(), req.UserID); err != nil {
		log.Printf("Failed to disable 2FA: %v", err)
		http.Error(w, "Failed to disable 2FA", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"success": true, "message": "2FA disabled by admin"})
}

// Verify2FALoginHandler verifies 2FA code during login
func (h *Handler) Verify2FALoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		UserID int    `json:"user_id"`
		Code   string `json:"code"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Get user
	user, err := h.AdminStore.GetUser(r.Context(), req.UserID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Verify code
	if !models.VerifyTOTPCode(user.TOTPSecret, req.Code) {
		http.Error(w, "Invalid verification code", http.StatusUnauthorized)
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

	// Create session after successful 2FA
	session, _ := sessionStore.Get(r, sessionName)
	session.Values["user_id"] = user.ID
	session.Values["username"] = user.Username
	session.Values["role"] = user.Role
	session.Save(r, w)

	// Return full login success
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
