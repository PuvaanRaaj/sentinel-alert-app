package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/SherClockHolmes/webpush-go"
)

var (
	vapidPrivateKey string
	vapidPublicKey  string
)

func init() {
	// Check for VAPID keys in env, or generate them
	vapidPrivateKey = os.Getenv("VAPID_PRIVATE_KEY")
	vapidPublicKey = os.Getenv("VAPID_PUBLIC_KEY")

	if vapidPrivateKey == "" || vapidPublicKey == "" {
		log.Println("VAPID keys not found in environment. Generating new keys...")
		privateKey, publicKey, err := webpush.GenerateVAPIDKeys()
		if err != nil {
			log.Fatal("Failed to generate VAPID keys:", err)
		}
		vapidPrivateKey = privateKey
		vapidPublicKey = publicKey
		log.Printf("Generated VAPID Keys:\nVAPID_PRIVATE_KEY=%s\nVAPID_PUBLIC_KEY=%s\n(Add these to your .env file to persist them)", privateKey, publicKey)
	}
}

// GetVAPIDKeyHandler returns the public VAPID key
func (h *Handler) GetVAPIDKeyHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"publicKey": vapidPublicKey,
	})
}

// SubscribePushHandler saves a push subscription
func (h *Handler) SubscribePushHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get current user
	session, _ := sessionStore.Get(r, sessionName)
	userID, ok := session.Values["user_id"].(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Endpoint string `json:"endpoint"`
		Keys     struct {
			P256dh string `json:"p256dh"`
			Auth   string `json:"auth"`
		} `json:"keys"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := h.AdminStore.SavePushSubscription(r.Context(), userID, req.Endpoint, req.Keys.P256dh, req.Keys.Auth); err != nil {
		log.Printf("Failed to save subscription: %v", err)
		http.Error(w, "Failed to save subscription", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// SendPushNotification sends a push notification to all subscribers
func (h *Handler) SendPushNotification(message string) {
	subs, err := h.AdminStore.GetPushSubscriptions(context.Background())
	if err != nil {
		log.Printf("Failed to get subscriptions: %v", err)
		return
	}

	for _, sub := range subs {
		s := &webpush.Subscription{
			Endpoint: sub.Endpoint,
			Keys: webpush.Keys{
				P256dh: sub.P256dh,
				Auth:   sub.Auth,
			},
		}

		// Send Notification
		resp, err := webpush.SendNotification([]byte(message), s, &webpush.Options{
			Subscriber:      "mailto:admin@example.com", // Should be configurable
			VAPIDPublicKey:  vapidPublicKey,
			VAPIDPrivateKey: vapidPrivateKey,
			TTL:             30,
		})
		if err != nil {
			log.Printf("Failed to send push to %s: %v", sub.Endpoint, err)
			continue
		}
		defer resp.Body.Close()
	}
}
