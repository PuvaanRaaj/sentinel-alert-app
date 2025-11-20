package models

import "time"

type Alert struct {
	ID        int
	CreatedAt time.Time
	Source    string
	Level     string
	Title     string
	Message   string
}
