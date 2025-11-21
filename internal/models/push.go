package models

import "time"

type PushSubscription struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	Endpoint  string    `json:"endpoint"`
	P256dh    string    `json:"keys_p256dh"` // Mapped from keys.p256dh
	Auth      string    `json:"keys_auth"`   // Mapped from keys.auth
	CreatedAt time.Time `json:"created_at"`
}
