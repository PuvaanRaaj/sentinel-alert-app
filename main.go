package main

import (
	"context"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"

	"incident-viewer-go/internal/handlers"
	"incident-viewer-go/internal/store"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using defaults")
	}

	// Redis Configuration
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	redisPassword := os.Getenv("REDIS_PASSWORD")
	redisDBStr := os.Getenv("REDIS_DB")
	redisDB := 0
	if redisDBStr != "" {
		if db, err := strconv.Atoi(redisDBStr); err == nil {
			redisDB = db
		}
	}

	// Initialize Redis store (for alerts)
	redisStore := store.NewRedisStore(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       redisDB,
	})

	// PostgreSQL Configuration
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	// Initialize Admin store (PostgreSQL)
	adminStore, err := store.NewPostgresStore(databaseURL)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Run database migrations
	ctx := context.Background()
	if err := adminStore.RunMigrations(ctx); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	log.Println("Database migrations completed")

	// Seed admin user
	if err := seedAdmin(ctx, adminStore); err != nil {
		log.Printf("Failed to seed admin user: %v", err)
	}

	// Parse templates
	tmplPath := filepath.Join("web", "templates", "index.html")
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		log.Fatalf("Failed to parse template: %v", err)
	}

	// Parse admin templates
	adminTmpl := make(map[string]*template.Template)
	adminTemplates := map[string]string{
		"login":     filepath.Join("web", "templates", "admin", "login.html"),
		"dashboard": filepath.Join("web", "templates", "admin", "dashboard.html"),
	}
	for name, path := range adminTemplates {
		t, err := template.ParseFiles(path)
		if err != nil {
			log.Printf("Failed to parse admin template %s: %v", name, err)
		} else {
			adminTmpl[name] = t
		}
	}

	// Initialize handlers with both stores
	h := handlers.NewHandler(redisStore, adminStore, tmpl, adminTmpl)

	// Initialize default admin user
	h.InitSession(ctx)

	// Public routes
	http.HandleFunc("/", h.IndexHandler)
	http.HandleFunc("/webhook", h.WebhookHandler)
	http.HandleFunc("/telegram/", h.TelegramHandler)
	http.HandleFunc("/clear", h.ClearHandler)
	http.HandleFunc("/events", h.SSEHandler)
	http.HandleFunc("/api/login", h.PublicLoginHandler)
	http.HandleFunc("/api/search", h.SearchHandler)
	http.HandleFunc("/api/chats", h.GetChatsPublicHandler)

	// Admin routes (login/logout)
	http.HandleFunc("/admin/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			h.AdminLoginPage(w, r)
		} else {
			h.LoginHandler(w, r)
		}
	})
	http.HandleFunc("/admin/verify-2fa", h.VerifyAdmin2FAHandler)
	http.HandleFunc("/admin/logout", h.LogoutHandler)
	http.HandleFunc("/admin/dashboard", handlers.AuthMiddleware(handlers.AdminMiddleware(h.AdminDashboardPage)))

	// Admin API routes (protected)
	http.HandleFunc("/api/admin/users", handlers.AuthMiddleware(handlers.AdminMiddleware(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.GetUsersHandler(w, r)
		case http.MethodPost:
			h.CreateUserHandler(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})))
	http.HandleFunc("/api/admin/users/", handlers.AuthMiddleware(handlers.AdminMiddleware(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			h.UpdateUserHandler(w, r)
		case http.MethodDelete:
			h.DeleteUserHandler(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})))

	// Bot management
	http.HandleFunc("/api/admin/bots", handlers.AuthMiddleware(handlers.AdminMiddleware(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.GetBotsHandler(w, r)
		case http.MethodPost:
			h.CreateBotHandler(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})))
	http.HandleFunc("/api/admin/bots/", handlers.AuthMiddleware(handlers.AdminMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			h.DeleteBotHandler(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})))

	// Chat management
	http.HandleFunc("/api/admin/chats", handlers.AuthMiddleware(handlers.AdminMiddleware(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.GetChatsHandler(w, r)
		case http.MethodPost:
			h.CreateChatHandler(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})))
	http.HandleFunc("/api/admin/chats/", handlers.AuthMiddleware(handlers.AdminMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			h.DeleteChatHandler(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})))
	http.HandleFunc("/api/admin/purge", handlers.AuthMiddleware(handlers.AdminMiddleware(h.PurgeAlertsHandler)))

	// User management routes
	http.HandleFunc("/api/user/profile", h.UpdateProfileHandler)
	http.HandleFunc("/api/user/change-password", h.ChangePasswordHandler)
	http.HandleFunc("/api/user/me", h.GetCurrentUserHandler)

	// Admin user management
	http.HandleFunc("/api/admin/reset-password", handlers.AuthMiddleware(handlers.AdminMiddleware(h.AdminResetPasswordHandler)))

	// 2FA routes
	http.HandleFunc("/api/user/2fa/generate", h.Generate2FAHandler)
	http.HandleFunc("/api/user/2fa/enable", h.Enable2FAHandler)
	http.HandleFunc("/api/user/2fa/disable", h.Disable2FAHandler)
	http.HandleFunc("/api/login/verify-2fa", h.Verify2FALoginHandler)
	http.HandleFunc("/api/admin/disable-2fa", handlers.AuthMiddleware(handlers.AdminMiddleware(h.AdminDisable2FAHandler)))

	// Bot webhook (public)
	http.HandleFunc("/bot/", h.BotWebhookHandler)

	// Serve static files (PWA assets)
	fs := http.FileServer(http.Dir("web/static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Println("Listening on :" + port)
	log.Println("Default admin: admin / admin123")
	log.Println("Admin dashboard: http://localhost:" + port + "/admin/login")
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

// seedAdmin creates a default admin user if one doesn't exist
func seedAdmin(ctx context.Context, s store.AdminStore) error {
	// Check if admin exists
	_, err := s.GetUserByUsername(ctx, "admin")
	if err == nil {
		return nil // Admin already exists
	}

	log.Println("Seeding default admin user...")
	_, err = s.CreateUser(ctx, "admin", "admin123", "admin")
	if err != nil {
		return err
	}
	log.Println("Default admin user created: admin / admin123")
	return nil
}
