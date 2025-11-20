package handlers

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"incident-viewer-go/internal/store"
)

type Handler struct {
	Store *store.Store
	Tmpl  *template.Template
}

func NewHandler(s *store.Store, tmpl *template.Template) *Handler {
	return &Handler{
		Store: s,
		Tmpl:  tmpl,
	}
}

func (h *Handler) IndexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// The template expects .Alerts.
	alerts := h.Store.GetAlerts()
	// We can pass alerts directly if we import models, but let's keep it simple.

	if err := h.Tmpl.Execute(w, map[string]any{"Alerts": alerts}); err != nil {
		log.Println("template error:", err)
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

	a := h.Store.AddAlert(source, level, title, message)

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

	a := h.Store.AddAlert(source, level, title, text)

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
	h.Store.ClearAlerts()
	http.Redirect(w, r, "/", http.StatusSeeOther)
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
