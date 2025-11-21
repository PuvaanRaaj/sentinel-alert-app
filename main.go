package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	"incident-viewer-go/internal/handlers"
	"incident-viewer-go/internal/models"
	"incident-viewer-go/internal/store"
)

var (
	reqCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sentinel_http_requests_total",
			Help: "Total HTTP requests",
		},
		[]string{"path", "method", "status"},
	)
	reqDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sentinel_http_request_duration_seconds",
			Help:    "HTTP request durations",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"path", "method"},
	)
)

func init() {
	prometheus.MustRegister(reqCount, reqDuration)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Flush implements http.Flusher when the underlying writer supports it
func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func tracingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID := fmt.Sprintf("%x", rand.Int63())
		ctx := context.WithValue(r.Context(), "trace_id", traceID)
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r.WithContext(ctx))
		log.Printf("[trace=%s] %s %s %d %s", traceID, r.Method, r.URL.Path, rec.status, time.Since(start))
	})
}

func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		reqCount.WithLabelValues(r.URL.Path, r.Method, fmt.Sprintf("%d", rec.status)).Inc()
		reqDuration.WithLabelValues(r.URL.Path, r.Method).Observe(time.Since(start).Seconds())
	})
}

func rateLimitMiddleware(rl *rateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := strings.Split(r.RemoteAddr, ":")[0]
			if !rl.allow(ip) {
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func idempotencyMiddleware(store *idempotencyStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("Idempotency-Key")
			if key != "" && store.seen(key) {
				http.Error(w, "duplicate request", http.StatusConflict)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func hmacMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if secret == "" {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sig := r.Header.Get("X-Sentinel-Signature")
			if sig == "" {
				http.Error(w, "missing signature", http.StatusUnauthorized)
				return
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "invalid body", http.StatusBadRequest)
				return
			}
			r.Body = io.NopCloser(bytes.NewBuffer(body)) // restore for downstream
			mac := hmac.New(sha256.New, []byte(secret))
			mac.Write(body)
			expected := hex.EncodeToString(mac.Sum(nil))
			if !hmac.Equal([]byte(sig), []byte(expected)) {
				http.Error(w, "invalid signature", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type rateLimiter struct {
	mu     sync.Mutex
	tokens map[string]*tokenBucket
	rate   float64
	burst  float64
	refill time.Duration
}

type tokenBucket struct {
	tokens float64
	last   time.Time
}

type idempotencyStore struct {
	mu    sync.Mutex
	items map[string]time.Time
	ttl   time.Duration
}

func newRateLimiter(rate int, burst int, refill time.Duration) *rateLimiter {
	return &rateLimiter{
		tokens: make(map[string]*tokenBucket),
		rate:   float64(rate),
		burst:  float64(burst),
		refill: refill,
	}
}

func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	bucket, ok := rl.tokens[key]
	if !ok {
		rl.tokens[key] = &tokenBucket{tokens: rl.burst - 1, last: now}
		return true
	}

	elapsed := now.Sub(bucket.last)
	bucket.tokens = minFloat(rl.burst, bucket.tokens+rl.rate*elapsed.Seconds()/rl.refill.Seconds())
	if bucket.tokens < 1 {
		return false
	}
	bucket.tokens--
	bucket.last = now
	return true
}

func newIdempotencyStore(ttl time.Duration) *idempotencyStore {
	return &idempotencyStore{items: make(map[string]time.Time), ttl: ttl}
}

func (s *idempotencyStore) seen(key string) bool {
	if key == "" {
		return false
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	if exp, ok := s.items[key]; ok && exp.After(now) {
		return true
	}
	s.items[key] = now.Add(s.ttl)
	return false
}

func (s *idempotencyStore) cleanupLoop(ctx context.Context) {
	t := time.NewTicker(s.ttl)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			now := time.Now()
			s.mu.Lock()
			for k, exp := range s.items {
				if exp.Before(now) {
					delete(s.items, k)
				}
			}
			s.mu.Unlock()
		}
	}
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func wrap(h http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

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

	// Observability helpers
	rl := newRateLimiter(60, 30, time.Second)
	idStore := newIdempotencyStore(10 * time.Minute)
	go idStore.cleanupLoop(ctx)
	webhookSecret := os.Getenv("WEBHOOK_SECRET")

	mux := http.NewServeMux()

	// Public routes
	mux.HandleFunc("/", h.IndexHandler)
	mux.Handle("/webhook", wrap(http.HandlerFunc(h.WebhookHandler), rateLimitMiddleware(rl), idempotencyMiddleware(idStore), hmacMiddleware(webhookSecret)))
	mux.Handle("/telegram/", wrap(http.HandlerFunc(h.TelegramHandler), rateLimitMiddleware(rl)))
	mux.Handle("/clear", http.HandlerFunc(h.ClearHandler))
	mux.Handle("/events", http.HandlerFunc(h.SSEHandler))
	mux.Handle("/api/login", http.HandlerFunc(h.PublicLoginHandler))
	mux.Handle("/api/login/verify-2fa", http.HandlerFunc(h.Verify2FALoginHandler))
	mux.Handle("/api/search", http.HandlerFunc(h.SearchHandler))
	mux.Handle("/api/chats", http.HandlerFunc(h.GetChatsPublicHandler))

	// Admin routes (login/logout)
	mux.HandleFunc("/admin/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			h.AdminLoginPage(w, r)
		} else {
			h.LoginHandler(w, r)
		}
	})
	mux.HandleFunc("/admin/verify-2fa", h.VerifyAdmin2FAHandler)
	mux.HandleFunc("/admin/logout", h.LogoutHandler)
	mux.Handle("/admin/dashboard", handlers.AuthMiddleware(handlers.AdminMiddleware(http.HandlerFunc(h.AdminDashboardPage))))

	// Admin API routes (protected)
	mux.Handle("/api/admin/users", handlers.AuthMiddleware(handlers.AdminMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.GetUsersHandler(w, r)
		case http.MethodPost:
			h.CreateUserHandler(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))))
	mux.Handle("/api/admin/users/", handlers.AuthMiddleware(handlers.AdminMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			h.UpdateUserHandler(w, r)
		case http.MethodDelete:
			h.DeleteUserHandler(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))))

	// Bot management
	mux.Handle("/api/admin/bots", handlers.AuthMiddleware(handlers.AdminMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.GetBotsHandler(w, r)
		case http.MethodPost:
			h.CreateBotHandler(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))))
	mux.Handle("/api/admin/bots/", handlers.AuthMiddleware(handlers.AdminMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			h.DeleteBotHandler(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))))

	// Chat management
	mux.Handle("/api/admin/chats", handlers.AuthMiddleware(handlers.AdminMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.GetChatsHandler(w, r)
		case http.MethodPost:
			h.CreateChatHandler(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))))
	mux.Handle("/api/admin/chats/", handlers.AuthMiddleware(handlers.AdminMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			h.DeleteChatHandler(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))))
	mux.Handle("/api/admin/purge", handlers.AuthMiddleware(handlers.AdminMiddleware(http.HandlerFunc(h.PurgeAlertsHandler))))

	// User management routes
	mux.Handle("/api/user/profile", http.HandlerFunc(h.UpdateProfileHandler))
	mux.Handle("/api/user/change-password", http.HandlerFunc(h.ChangePasswordHandler))
	mux.Handle("/api/user/me", http.HandlerFunc(h.GetCurrentUserHandler))

	// Admin user management
	mux.Handle("/api/admin/reset-password", handlers.AuthMiddleware(handlers.AdminMiddleware(http.HandlerFunc(h.AdminResetPasswordHandler))))
	mux.Handle("/api/admin/audit", handlers.AuthMiddleware(handlers.AdminMiddleware(http.HandlerFunc(h.GetAuditLogs))))

	// Serve sw.js at root for Service Worker scope
	mux.HandleFunc("/sw.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		http.ServeFile(w, r, "web/static/sw.js")
	})

	// 2FA routes
	mux.Handle("/api/user/2fa/generate", http.HandlerFunc(h.Generate2FAHandler))
	mux.Handle("/api/user/2fa/enable", http.HandlerFunc(h.Enable2FAHandler))
	mux.Handle("/api/user/2fa/disable", http.HandlerFunc(h.Disable2FAHandler))
	mux.Handle("/api/admin/disable-2fa", handlers.AuthMiddleware(handlers.AdminMiddleware(http.HandlerFunc(h.AdminDisable2FAHandler))))

	// Bot webhook (public)
	mux.Handle("/bot/", wrap(http.HandlerFunc(h.BotWebhookHandler), rateLimitMiddleware(rl), idempotencyMiddleware(idStore), hmacMiddleware(webhookSecret)))

	// Push Notification routes
	mux.Handle("/api/push/vapid-public-key", http.HandlerFunc(h.GetVAPIDKeyHandler))
	mux.Handle("/api/push/subscribe", http.HandlerFunc(h.SubscribePushHandler))

	// New Webhook Integrations
	mux.Handle("/api/slack/webhook", wrap(http.HandlerFunc(h.SlackWebhookHandler), rateLimitMiddleware(rl), idempotencyMiddleware(idStore), hmacMiddleware(webhookSecret)))
	mux.Handle("/api/discord/webhook", wrap(http.HandlerFunc(h.DiscordWebhookHandler), rateLimitMiddleware(rl), idempotencyMiddleware(idStore), hmacMiddleware(webhookSecret)))

	// Swagger UI
	mux.HandleFunc("/swagger/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/static/swagger/"+strings.TrimPrefix(r.URL.Path, "/swagger/"))
	})

	// Health/ready/metrics
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		if err := redisStore.Ping(context.Background()); err != nil {
			http.Error(w, "redis not ready", http.StatusServiceUnavailable)
			return
		}
		if err := adminStore.Ping(context.Background()); err != nil {
			http.Error(w, "db not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	})
	mux.Handle("/metrics", promhttp.Handler())

	// Start background listener for push notifications
	go func() {
		pubsub := redisStore.Subscribe(context.Background())
		defer pubsub.Close()
		ch := pubsub.Channel()

		for msg := range ch {
			var alert models.Alert
			if err := json.Unmarshal([]byte(msg.Payload), &alert); err == nil {
				h.SendPushNotification(fmt.Sprintf("🚨 %s: %s", alert.Title, alert.Message))
			} else {
				h.SendPushNotification("New Incident Alert Received!")
			}
		}
	}()

	// Serve static files (PWA assets)
	fs := http.FileServer(http.Dir("web/static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	rootHandler := wrap(mux, tracingMiddleware, metricsMiddleware)

	log.Println("Listening on :" + port)
	log.Println("Default admin: admin / admin123")
	log.Println("Admin dashboard: http://localhost:" + port + "/admin/login")
	if err := http.ListenAndServe(":"+port, rootHandler); err != nil {
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
