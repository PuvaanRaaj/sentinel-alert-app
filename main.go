package main

import (
	"html/template"
	"log"
	"net/http"
	"path/filepath"

	"incident-viewer-go/internal/handlers"
	"incident-viewer-go/internal/store"
)

func main() {
	// Initialize store
	s := store.NewStore()

	// Parse templates
	tmplPath := filepath.Join("web", "templates", "index.html")
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		log.Fatalf("Failed to parse template: %v", err)
	}

	// Initialize handlers
	h := handlers.NewHandler(s, tmpl)

	// Register routes
	http.HandleFunc("/", h.IndexHandler)
	http.HandleFunc("/webhook", h.WebhookHandler)
	http.HandleFunc("/telegram/", h.TelegramHandler)
	http.HandleFunc("/clear", h.ClearHandler)

	// Serve static files (PWA assets)
	fs := http.FileServer(http.Dir("web/static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	log.Println("Listening on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
