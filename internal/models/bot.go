package models

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

type Bot struct {
	ID        int       `json:"id"`
	Token     string    `json:"token"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	CreatedBy int       `json:"created_by"`
}

type Chat struct {
	ID        int       `json:"id"`
	ChatID    string    `json:"chat_id"`
	Name      string    `json:"name"`
	BotID     int       `json:"bot_id"`
	CreatedAt time.Time `json:"created_at"`
}

// GenerateToken creates a random bot token
func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
