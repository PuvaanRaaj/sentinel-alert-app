package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"incident-viewer-go/internal/models"

	"github.com/redis/go-redis/v9"
)

const (
	alertTTL = 30 * 24 * time.Hour // 30 days
)

// AlertStore handles alert operations (Redis)
type AlertStore interface {
	AddAlert(ctx context.Context, source, level, title, message string) (models.Alert, error)
	GetAlerts(ctx context.Context) ([]models.Alert, error)
	SearchAlerts(ctx context.Context, query, level, source string) ([]models.Alert, error)
	ClearAlerts(ctx context.Context) error
	PurgeAllAlerts(ctx context.Context) error
	Subscribe(ctx context.Context) *redis.PubSub
}

// AdminStore handles admin operations (PostgreSQL)
type AdminStore interface {
	// User methods
	CreateUser(ctx context.Context, username, password, role string) (models.User, error)
	GetUser(ctx context.Context, id int) (models.User, error)
	GetUserByUsername(ctx context.Context, username string) (models.User, error)
	GetUsers(ctx context.Context) ([]models.User, error)
	UpdateUser(ctx context.Context, id int, username, role string) error
	DeleteUser(ctx context.Context, id int) error

	// Bot methods
	CreateBot(ctx context.Context, name string, createdBy int) (models.Bot, error)
	GetBot(ctx context.Context, id int) (models.Bot, error)
	GetBotByToken(ctx context.Context, token string) (models.Bot, error)
	GetBots(ctx context.Context) ([]models.Bot, error)
	DeleteBot(ctx context.Context, id int) error

	// Chat methods
	CreateChat(ctx context.Context, chatID, name string, botID int) (models.Chat, error)
	GetChat(ctx context.Context, id int) (models.Chat, error)
	GetChats(ctx context.Context) ([]models.Chat, error)
	DeleteChat(ctx context.Context, id int) error
}

type RedisStore struct {
	client *redis.Client
}

func NewRedisStore(opts *redis.Options) *RedisStore {
	rdb := redis.NewClient(opts)
	return &RedisStore{client: rdb}
}

func (s *RedisStore) AddAlert(ctx context.Context, source, level, title, message string) (models.Alert, error) {
	// Generate ID
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

	key := fmt.Sprintf("alert:%d", a.ID)

	// Store alert as hash with TTL
	pipe := s.client.Pipeline()
	pipe.Set(ctx, key, data, alertTTL)

	// Add to timeline sorted set (score = timestamp)
	pipe.ZAdd(ctx, "alerts:timeline", redis.Z{
		Score:  float64(a.CreatedAt.Unix()),
		Member: key,
	})

	// Add to search indices
	if level != "" {
		pipe.SAdd(ctx, fmt.Sprintf("alerts:level:%s", strings.ToLower(level)), key)
		pipe.Expire(ctx, fmt.Sprintf("alerts:level:%s", strings.ToLower(level)), alertTTL)
	}
	if source != "" {
		pipe.SAdd(ctx, fmt.Sprintf("alerts:source:%s", strings.ToLower(source)), key)
		pipe.Expire(ctx, fmt.Sprintf("alerts:source:%s", strings.ToLower(source)), alertTTL)
	}

	_, err = pipe.Exec(ctx)
	if err != nil {
		return models.Alert{}, err
	}

	// Publish event for SSE
	if err := s.client.Publish(ctx, "alert_events", data).Err(); err != nil {
		fmt.Println("Failed to publish event:", err)
	}

	return a, nil
}

func (s *RedisStore) GetAlerts(ctx context.Context) ([]models.Alert, error) {
	// Get alert keys from sorted set (newest first)
	keys, err := s.client.ZRevRange(ctx, "alerts:timeline", 0, -1).Result()
	if err != nil {
		return nil, err
	}

	var alerts []models.Alert
	for _, key := range keys {
		val, err := s.client.Get(ctx, key).Result()
		if err == redis.Nil {
			// Alert expired, remove from sorted set
			s.client.ZRem(ctx, "alerts:timeline", key)
			continue
		} else if err != nil {
			continue
		}

		var a models.Alert
		if err := json.Unmarshal([]byte(val), &a); err == nil {
			alerts = append(alerts, a)
		}
	}
	return alerts, nil
}

func (s *RedisStore) SearchAlerts(ctx context.Context, query, level, source string) ([]models.Alert, error) {
	var keys []string

	// Build intersection of search criteria
	var setKeys []string
	if level != "" {
		setKeys = append(setKeys, fmt.Sprintf("alerts:level:%s", strings.ToLower(level)))
	}
	if source != "" {
		setKeys = append(setKeys, fmt.Sprintf("alerts:source:%s", strings.ToLower(source)))
	}

	if len(setKeys) > 0 {
		// Intersect sets if multiple criteria
		if len(setKeys) == 1 {
			members, err := s.client.SMembers(ctx, setKeys[0]).Result()
			if err != nil {
				return nil, err
			}
			keys = members
		} else {
			members, err := s.client.SInter(ctx, setKeys...).Result()
			if err != nil {
				return nil, err
			}
			keys = members
		}
	} else {
		// No filters, get all from timeline
		allKeys, err := s.client.ZRevRange(ctx, "alerts:timeline", 0, -1).Result()
		if err != nil {
			return nil, err
		}
		keys = allKeys
	}

	// Fetch and filter by query text
	var alerts []models.Alert
	query = strings.ToLower(query)

	for _, key := range keys {
		val, err := s.client.Get(ctx, key).Result()
		if err == redis.Nil {
			continue
		} else if err != nil {
			continue
		}

		var a models.Alert
		if err := json.Unmarshal([]byte(val), &a); err != nil {
			continue
		}

		// Text search in title and message
		if query != "" {
			searchText := strings.ToLower(a.Title + " " + a.Message + " " + a.Source)
			if !strings.Contains(searchText, query) {
				continue
			}
		}

		alerts = append(alerts, a)
	}

	return alerts, nil
}

func (s *RedisStore) ClearAlerts(ctx context.Context) error {
	return s.client.Del(ctx, "alerts").Err()
}

func (s *RedisStore) PurgeAllAlerts(ctx context.Context) error {
	// Delete all keys matching alert:*
	iter := s.client.Scan(ctx, 0, "alert:*", 0).Iterator()
	keys := []string{}

	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}

	if err := iter.Err(); err != nil {
		return err
	}

	if len(keys) > 0 {
		s.client.Del(ctx, keys...)
	}

	// Clear timeline
	s.client.Del(ctx, "alerts:timeline")

	// Clear index sets (use SCAN to find them)
	iter = s.client.Scan(ctx, 0, "alerts:level:*", 0).Iterator()
	indexKeys := []string{}
	for iter.Next(ctx) {
		indexKeys = append(indexKeys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return err
	}
	if len(indexKeys) > 0 {
		s.client.Del(ctx, indexKeys...)
	}

	iter = s.client.Scan(ctx, 0, "alerts:source:*", 0).Iterator()
	sourceKeys := []string{}
	for iter.Next(ctx) {
		sourceKeys = append(sourceKeys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return err
	}
	if len(sourceKeys) > 0 {
		s.client.Del(ctx, sourceKeys...)
	}

	return nil
}

func (s *RedisStore) Subscribe(ctx context.Context) *redis.PubSub {
	return s.client.Subscribe(ctx, "alert_events")
}
