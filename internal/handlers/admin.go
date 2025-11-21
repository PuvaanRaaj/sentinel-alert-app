package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// === User Management ===

func (h *Handler) GetUsersHandler(w http.ResponseWriter, r *http.Request) {
	users, err := h.Store.GetUsers(r.Context())
	if err != nil {
		http.Error(w, "Failed to get users", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"users": users})
}

func (h *Handler) CreateUserHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	user, err := h.Store.CreateUser(r.Context(), req.Username, req.Password, req.Role)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"success": true, "user": user})
}

func (h *Handler) UpdateUserHandler(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/admin/users/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var req struct {
		Username string `json:"username"`
		Role     string `json:"role"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := h.Store.UpdateUser(r.Context(), id, req.Username, req.Role); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"success": true})
}

func (h *Handler) DeleteUserHandler(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/admin/users/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	if err := h.Store.DeleteUser(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"success": true})
}

// === Bot Management ===

func (h *Handler) GetBotsHandler(w http.ResponseWriter, r *http.Request) {
	bots, err := h.Store.GetBots(r.Context())
	if err != nil {
		http.Error(w, "Failed to get bots", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"bots": bots})
}

func (h *Handler) CreateBotHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	userID, _, _ := GetCurrentUser(r)
	bot, err := h.Store.CreateBot(r.Context(), req.Name, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"success": true, "bot": bot})
}

func (h *Handler) DeleteBotHandler(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/admin/bots/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	if err := h.Store.DeleteBot(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"success": true})
}

// === Chat Management ===

func (h *Handler) GetChatsHandler(w http.ResponseWriter, r *http.Request) {
	chats, err := h.Store.GetChats(r.Context())
	if err != nil {
		http.Error(w, "Failed to get chats", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"chats": chats})
}

func (h *Handler) CreateChatHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ChatID string `json:"chat_id"`
		Name   string `json:"name"`
		BotID  int    `json:"bot_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	chat, err := h.Store.CreateChat(r.Context(), req.ChatID, req.Name, req.BotID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"success": true, "chat": chat})
}

func (h *Handler) DeleteChatHandler(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/admin/chats/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	if err := h.Store.DeleteChat(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"success": true})
}

// === Bot Webhook Handler ===

func (h *Handler) BotWebhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Extract token from path: /bot{token}/sendMessage
	path := r.URL.Path
	if !strings.HasPrefix(path, "/bot") || !strings.Contains(path, "/sendMessage") {
		http.Error(w, "Invalid path", http.StatusNotFound)
		return
	}

	// Get token between /bot and /sendMessage
	token := strings.TrimPrefix(path, "/bot")
	token = strings.TrimSuffix(token, "/sendMessage")

	// Validate bot token
	bot, err := h.Store.GetBotByToken(r.Context(), token)
	if err != nil {
		log.Printf("Invalid bot token: %s", token)
		http.Error(w, "Invalid bot token", http.StatusUnauthorized)
		return
	}

	// Parse message (Telegram-like format)
	var req struct {
		ChatID string `json:"chat_id"`
		Text   string `json:"text"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Create alert
	alert, err := h.Store.AddAlert(r.Context(), fmt.Sprintf("bot:%s", bot.Name), "info", "Bot Message", req.Text)
	if err != nil {
		log.Println("AddAlert error:", err)
		http.Error(w, "Failed to create alert", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success":    true,
		"message_id": alert.ID,
	})
}
