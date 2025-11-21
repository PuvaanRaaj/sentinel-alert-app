package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"incident-viewer-go/internal/store"
)

type Handler struct {
	Store     store.Store
	Tmpl      *template.Template
	AdminTmpl map[string]*template.Template
}

func NewHandler(s store.Store, tmpl *template.Template, adminTmpl map[string]*template.Template) *Handler {
	return &Handler{
		Store:     s,
		Tmpl:      tmpl,
		AdminTmpl: adminTmpl,
	}
}

func (h *Handler) RenderAdminPage(w http.ResponseWriter, page string, data any) {
	if tmpl, ok := h.AdminTmpl[page]; ok {
		if err := tmpl.Execute(w, data); err != nil {
			log.Println("Template error:", err)
			http.Error(w, "Template error", http.StatusInternalServerError)
		}
	} else {
		http.Error(w, "Page not found", http.StatusNotFound)
	}
}

func (h *Handler) AdminLoginPage(w http.ResponseWriter, r *http.Request) {
	h.RenderAdminPage(w, "login", nil)
}

func (h *Handler) AdminDashboardPage(w http.ResponseWriter, r *http.Request) {
	userID, username, _ := GetCurrentUser(r)
	h.RenderAdminPage(w, "dashboard", map[string]any{
		"UserID":   userID,
		"Username": username,
	})
}

func (h *Handler) IndexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	alerts, err := h.Store.GetAlerts(r.Context())
	if err != nil {
		log.Println("Failed to get alerts:", err)
		http.Error(w, "Failed to get alerts", http.StatusInternalServerError)
		return
	}

	if err := h.Tmpl.Execute(w, map[string]any{"Alerts": alerts}); err != nil {
		log.Println("template error:", err)
	}
}

func (h *Handler) SSEHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Subscribe to Redis channel
	pubsub := h.Store.Subscribe(r.Context())
	defer pubsub.Close()

	ch := pubsub.Channel()

	// Send initial connection message (optional)
	fmt.Fprintf(w, "data: %s\n\n", "connected")
	w.(http.Flusher).Flush()

	for {
		select {
		case msg := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", msg.Payload)
			w.(http.Flusher).Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (h *Handler) WebhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Try JSON first
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		// Fallback: form/query
		if err := r.ParseForm(); err == nil && len(r.Form) > 0 {
			payload = make(map[string]any)
			for k, v := range r.Form {
				if len(v) > 0 {
					payload[k] = v[0]
				}
			}
		} else {
			payload = map[string]any{"raw": "unparseable payload"}
		}
	}

	source := getString(payload["source"])
	if source == "" {
		source = r.URL.Query().Get("source")
	}
	if source == "" {
		source = "unknown"
	}

	level := getString(payload["level"])
	if level == "" {
		level = getString(payload["severity"])
	}
	if level == "" {
		level = getString(payload["status"])
	}
	if level == "" {
		level = "info"
	}

	title := getString(payload["title"])
	if title == "" {
		title = getString(payload["alert_name"])
	}
	if title == "" {
		title = getString(payload["event"])
	}
	if title == "" {
		title = "Alert"
	}

	var message string
	for _, key := range []string{"message", "description", "detail"} {
		if v, ok := payload[key]; ok {
			message = getString(v)
			if message != "" {
				break
			}
		}
	}
	if message == "" {
		buf, _ := json.MarshalIndent(payload, "", "  ")
		message = string(buf)
	}

	a, err := h.Store.AddAlert(r.Context(), source, level, title, message)
	if err != nil {
		log.Println("Failed to add alert:", err)
		http.Error(w, "Failed to add alert", http.StatusInternalServerError)
		return
	}

	resp := map[string]any{
		"status":     "ok",
		"id":         a.ID,
		"created_at": a.CreatedAt.Format(time.RFC3339),
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// Mimic Telegram: /telegram/bot<TOKEN>/sendMessage
func (h *Handler) TelegramHandler(w http.ResponseWriter, r *http.Request) {
	// Path after /telegram/
	rest := strings.TrimPrefix(r.URL.Path, "/telegram/")
	parts := strings.Split(rest, "/")
	if len(parts) < 2 {
		http.Error(w, "invalid telegram path", http.StatusBadRequest)
		return
	}

	botPart := parts[0] // e.g. "bot123456:ABC"
	method := parts[1]  // e.g. "sendMessage"

	if !strings.HasPrefix(botPart, "bot") {
		http.Error(w, "invalid bot path", http.StatusBadRequest)
		return
	}
	if method != "sendMessage" {
		http.Error(w, "only sendMessage is supported", http.StatusBadRequest)
		return
	}

	// Telegram usually sends form-encoded, but we support JSON too.
	var payload map[string]any

	// Try JSON first
	if r.Header.Get("Content-Type") == "application/json" {
		_ = json.NewDecoder(r.Body).Decode(&payload)
	}
	if payload == nil {
		if err := r.ParseForm(); err == nil && len(r.Form) > 0 {
			payload = make(map[string]any)
			for k, v := range r.Form {
				if len(v) > 0 {
					payload[k] = v[0]
				}
			}
		}
	}
	if payload == nil {
		payload = make(map[string]any)
	}

	chatID := getString(payload["chat_id"])
	if chatID == "" {
		chatID = "unknown"
	}
	text := getString(payload["text"])

	source := "telegram:" + chatID
	title := "Telegram message (chat " + chatID + ")"
	level := "info"
	if text == "" {
		text = "(empty message)"
	}

	a, err := h.Store.AddAlert(r.Context(), source, level, title, text)
	if err != nil {
		log.Println("Failed to add alert:", err)
		http.Error(w, "Failed to add alert", http.StatusInternalServerError)
		return
	}

	resp := map[string]any{
		"ok": true,
		"result": map[string]any{
			"message_id": a.ID,
			"from": map[string]any{
				"id":         0,
				"is_bot":     true,
				"first_name": "LocalAlertBot",
				"username":   "LocalAlertBot",
			},
			"chat": map[string]any{
				"id":   chatID,
				"type": "private",
			},
			"date": a.CreatedAt.Unix(),
			"text": text,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *Handler) ClearHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	h.Store.ClearAlerts(r.Context())
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) SearchHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	level := r.URL.Query().Get("level")
	source := r.URL.Query().Get("source")

	alerts, err := h.Store.SearchAlerts(r.Context(), query, level, source)
	if err != nil {
		log.Println("Search error:", err)
		http.Error(w, "Search failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"alerts": alerts,
		"count":  len(alerts),
	})
}

func getString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	case float64:
		// json numbers
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		return ""
	}
}
