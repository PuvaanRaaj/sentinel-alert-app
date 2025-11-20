package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"incident-viewer-go/internal/models"

	"github.com/redis/go-redis/v9"
)

type Store interface {
	AddAlert(ctx context.Context, source, level, title, message string) (models.Alert, error)
	GetAlerts(ctx context.Context) ([]models.Alert, error)
	ClearAlerts(ctx context.Context) error
	Subscribe(ctx context.Context) *redis.PubSub
}

type RedisStore struct {
	client *redis.Client
}

func NewRedisStore(opts *redis.Options) *RedisStore {
	rdb := redis.NewClient(opts)
	return &RedisStore{client: rdb}
}

func (s *RedisStore) AddAlert(ctx context.Context, source, level, title, message string) (models.Alert, error) {
	// Generate ID (simple increment for now, or use timestamp/uuid)
	id, err := s.client.Incr(ctx, "alert:next_id").Result()
	if err != nil {
		return models.Alert{}, err
	}

	a := models.Alert{
		ID:        int(id),
		CreatedAt: time.Now().UTC(),
		Source:    source,
		Level:     level,
		Title:     title,
		Message:   message,
	}

	data, err := json.Marshal(a)
	if err != nil {
		return models.Alert{}, err
	}

	// Store in a list or sorted set. Let's use a List for simplicity, pushing to head (LPUSH)
	if err := s.client.LPush(ctx, "alerts", data).Err(); err != nil {
		return models.Alert{}, err
	}

	// Publish event for SSE
	if err := s.client.Publish(ctx, "alert_events", data).Err(); err != nil {
		// Log error but don't fail the request?
		fmt.Println("Failed to publish event:", err)
	}

	return a, nil
}

func (s *RedisStore) GetAlerts(ctx context.Context) ([]models.Alert, error) {
	// Get all alerts
	val, err := s.client.LRange(ctx, "alerts", 0, -1).Result()
	if err != nil {
		return nil, err
	}

	var alerts []models.Alert
	for _, v := range val {
		var a models.Alert
		if err := json.Unmarshal([]byte(v), &a); err == nil {
			alerts = append(alerts, a)
		}
	}
	return alerts, nil
}

func (s *RedisStore) ClearAlerts(ctx context.Context) error {
	return s.client.Del(ctx, "alerts").Err()
}

func (s *RedisStore) Subscribe(ctx context.Context) *redis.PubSub {
	return s.client.Subscribe(ctx, "alert_events")
}
