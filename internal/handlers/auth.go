package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/sessions"
)

var (
	sessionStore = sessions.NewCookieStore([]byte("secret-key-change-in-production"))
	sessionName  = "sentinel-session"
)

// LoginHandler handles admin login
func (h *Handler) LoginHandler(w http.ResponseWriter, r *http.Request) {
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

	// Get user by username
	user, err := h.Store.GetUserByUsername(r.Context(), req.Username)
	if err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Check password
	if !user.CheckPassword(req.Password) {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Create session
	session, _ := sessionStore.Get(r, sessionName)
	session.Values["user_id"] = user.ID
	session.Values["username"] = user.Username
	session.Values["role"] = user.Role
	session.Save(r, w)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success":  true,
		"user":     user,
		"redirect": "/admin/dashboard",
	})
}

// LogoutHandler handles logout
func (h *Handler) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := sessionStore.Get(r, sessionName)
	session.Values["user_id"] = nil
	session.Options.MaxAge = -1
	session.Save(r, w)

	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}

// AuthMiddleware checks if user is authenticated
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionStore.Get(r, sessionName)
		userID, ok := session.Values["user_id"].(int)
		if !ok || userID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// AdminMiddleware checks if user is admin
func AdminMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionStore.Get(r, sessionName)
		role, ok := session.Values["role"].(string)
		if !ok || role != "admin" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

// GetCurrentUser returns the current user from session
func GetCurrentUser(r *http.Request) (int, string, string) {
	session, _ := sessionStore.Get(r, sessionName)
	userID, _ := session.Values["user_id"].(int)
	username, _ := session.Values["username"].(string)
	role, _ := session.Values["role"].(string)
	return userID, username, role
}

// InitSession initializes a default admin user if none exists
func (h *Handler) InitSession(ctx context.Context) {
	users, err := h.Store.GetUsers(ctx)
	if err != nil || len(users) == 0 {
		// Create default admin
		user, err := h.Store.CreateUser(ctx, "admin", "admin123", "admin")
		if err != nil {
			log.Println("Failed to create default admin:", err)
		} else {
			fmt.Printf("Created default admin user: %s / admin123\n", user.Username)
		}
	}
}
