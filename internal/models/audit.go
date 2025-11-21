package models

import "time"

type AuditLog struct {
	ID         int       `json:"id"`
	ActorID    int       `json:"actor_id"`
	Action     string    `json:"action"`
	TargetType string    `json:"target_type"`
	TargetID   int       `json:"target_id,omitempty"`
	Metadata   string    `json:"metadata,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}
