package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"incident-viewer-go/internal/models"
)

// === User Management ===

func (h *Handler) GetUsersHandler(w http.ResponseWriter, r *http.Request) {
	users, err := h.AdminStore.GetUsers(r.Context())
	if err != nil {
		http.Error(w, "Failed to get users", http.StatusInternalServerError)
		return
	}

	type chatView struct {
		ID     int    `json:"id"`
		ChatID string `json:"chat_id"`
		Name   string `json:"name"`
		BotID  int    `json:"bot_id"`
	}

	respUsers := make([]map[string]any, 0, len(users))
	for _, u := range users {
		chats := []chatView{}
		if u.Role != "admin" && u.Role != "developer" {
			if assigned, err := h.AdminStore.GetUserChats(r.Context(), u.ID); err == nil {
				for _, c := range assigned {
					chats = append(chats, chatView{
						ID:     c.ID,
						ChatID: c.ChatID,
						Name:   c.Name,
						BotID:  c.BotID,
					})
				}
			}
		}
		respUsers = append(respUsers, map[string]any{
			"id":            u.ID,
			"username":      u.Username,
			"role":          u.Role,
			"totp_enabled":  u.TOTPEnabled,
			"chats":         chats,
			"created_at":    u.CreatedAt,
			"last_password": u.LastPasswordChange,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"users": respUsers})
}

func (h *Handler) CreateUserHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
		ChatIDs  []int  `json:"chat_ids"` // New: chat permissions
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Validate role
	if req.Role != "admin" && req.Role != "developer" && req.Role != "user" {
		http.Error(w, "Invalid role", http.StatusBadRequest)
		return
	}

	user, err := h.AdminStore.CreateUser(r.Context(), req.Username, req.Password, req.Role)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if actorID, _, _ := GetCurrentUser(r); actorID != 0 {
		meta, _ := json.Marshal(map[string]any{"username": req.Username, "role": req.Role, "chat_ids": req.ChatIDs})
		_ = h.AdminStore.InsertAudit(r.Context(), actorID, "create_user", "user", user.ID, string(meta))
	}

	// Assign chat permissions for non-admin users
	if req.Role != "admin" && len(req.ChatIDs) > 0 {
		for _, chatID := range req.ChatIDs {
			if err := h.AdminStore.AssignChatToUser(r.Context(), user.ID, chatID); err != nil {
				log.Printf("Failed to assign chat %d to user %d: %v", chatID, user.ID, err)
			}
		}
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
		ChatIDs  []int  `json:"chat_ids"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.Role != "admin" && req.Role != "developer" && req.Role != "user" {
		http.Error(w, "Invalid role", http.StatusBadRequest)
		return
	}

	if err := h.AdminStore.UpdateUser(r.Context(), id, req.Username, req.Role); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Manage chat assignments for non-admin roles
	if req.Role != "admin" && len(req.ChatIDs) > 0 {
		currentChats, _ := h.AdminStore.GetUserChats(r.Context(), id)
		desired := make(map[int]struct{})
		for _, cid := range req.ChatIDs {
			desired[cid] = struct{}{}
		}
		// Remove missing
		for _, chat := range currentChats {
			if _, ok := desired[chat.ID]; !ok {
				_ = h.AdminStore.RemoveChatFromUser(r.Context(), id, chat.ID)
			}
		}
		// Add new
		for cid := range desired {
			_ = h.AdminStore.AssignChatToUser(r.Context(), id, cid)
		}
	}

	if actorID, _, _ := GetCurrentUser(r); actorID != 0 {
		meta, _ := json.Marshal(map[string]any{"username": req.Username, "role": req.Role, "chat_ids": req.ChatIDs})
		_ = h.AdminStore.InsertAudit(r.Context(), actorID, "update_user", "user", id, string(meta))
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

	if err := h.AdminStore.DeleteUser(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if actorID, _, _ := GetCurrentUser(r); actorID != 0 {
		_ = h.AdminStore.InsertAudit(r.Context(), actorID, "delete_user", "user", id, "{}")
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"success": true})
}

// === Bot Management ===

func (h *Handler) GetBotsHandler(w http.ResponseWriter, r *http.Request) {
	bots, err := h.AdminStore.GetBots(r.Context())
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
	bot, err := h.AdminStore.CreateBot(r.Context(), req.Name, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if userID != 0 {
		meta, _ := json.Marshal(map[string]any{"name": req.Name})
		_ = h.AdminStore.InsertAudit(r.Context(), userID, "create_bot", "bot", bot.ID, string(meta))
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

	if err := h.AdminStore.DeleteBot(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if actorID, _, _ := GetCurrentUser(r); actorID != 0 {
		_ = h.AdminStore.InsertAudit(r.Context(), actorID, "delete_bot", "bot", id, "{}")
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"success": true})
}

// === Chat Management ===

func (h *Handler) GetChatsHandler(w http.ResponseWriter, r *http.Request) {
	chats, err := h.AdminStore.GetChats(r.Context())
	if err != nil {
		http.Error(w, "Failed to get chats", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"chats": chats})
}

func (h *Handler) CreateChatHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		BotID int    `json:"bot_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Auto-generate unique chat ID
	chatID := fmt.Sprintf("chat_%d_%d", req.BotID, time.Now().UnixNano())

	chat, err := h.AdminStore.CreateChat(r.Context(), chatID, req.Name, req.BotID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if actorID, _, _ := GetCurrentUser(r); actorID != 0 {
		meta, _ := json.Marshal(map[string]any{"name": req.Name, "bot_id": req.BotID, "chat_id": chat.ChatID})
		_ = h.AdminStore.InsertAudit(r.Context(), actorID, "create_chat", "chat", chat.ID, string(meta))
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

	if err := h.AdminStore.DeleteChat(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if actorID, _, _ := GetCurrentUser(r); actorID != 0 {
		_ = h.AdminStore.InsertAudit(r.Context(), actorID, "delete_chat", "chat", id, "{}")
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"success": true})
}

// Audit listing
func (h *Handler) GetAuditLogs(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	logs, err := h.AdminStore.ListAudit(r.Context(), limit)
	if err != nil {
		http.Error(w, "Failed to load audit logs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"logs": logs,
	})
}

// === Bot Webhook Handler ===

func (h *Handler) BotWebhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Extract token from path: /bot/{token}/sendMessage
	path := r.URL.Path

	// Remove /bot/ prefix to get token/sendMessage
	path = strings.TrimPrefix(path, "/bot/")

	// Check if it ends with /sendMessage
	if !strings.HasSuffix(path, "/sendMessage") {
		http.Error(w, "Invalid path - must end with /sendMessage", http.StatusNotFound)
		return
	}

	// Get token between start and /sendMessage
	token := strings.TrimSuffix(path, "/sendMessage")

	if token == "" {
		http.Error(w, "Missing bot token", http.StatusBadRequest)
		return
	}

	// Validate bot token
	bot, err := h.AdminStore.GetBotByToken(r.Context(), token)
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

	// Create alert with chat_id in source for filtering
	source := fmt.Sprintf("bot:%s:chat:%s", bot.Name, req.ChatID)
	alert, err := h.AlertStore.AddAlert(r.Context(), source, "info", "Bot Message", req.Text)
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
