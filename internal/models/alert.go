package models

import "time"

type Alert struct {
	ID        int       `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Source    string    `json:"source"`
	Level     string    `json:"level"`
	Title     string    `json:"title"`
	Message   string    `json:"message"`
}
