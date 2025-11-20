package store

import (
	"sync"
	"time"

	"incident-viewer-go/internal/models"
)

type Store struct {
	alerts   []models.Alert
	alertsMu sync.Mutex
	nextID   int
}

func NewStore() *Store {
	return &Store{
		nextID: 1,
	}
}

func (s *Store) AddAlert(source, level, title, message string) models.Alert {
	s.alertsMu.Lock()
	defer s.alertsMu.Unlock()

	a := models.Alert{
		ID:        s.nextID,
		CreatedAt: time.Now().UTC(),
		Source:    source,
		Level:     level,
		Title:     title,
		Message:   message,
	}
	s.nextID++
	s.alerts = append(s.alerts, a)
	return a
}

func (s *Store) ClearAlerts() {
	s.alertsMu.Lock()
	defer s.alertsMu.Unlock()

	s.alerts = nil
	s.nextID = 1
}

func (s *Store) GetAlerts() []models.Alert {
	s.alertsMu.Lock()
	defer s.alertsMu.Unlock()

	out := make([]models.Alert, len(s.alerts))
	copy(out, s.alerts)
	return out
}
