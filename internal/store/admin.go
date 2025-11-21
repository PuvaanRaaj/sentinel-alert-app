package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"incident-viewer-go/internal/models"
)

// User methods

func (s *RedisStore) CreateUser(ctx context.Context, username, password, role string) (models.User, error) {
	// Check if username exists
	existing, _ := s.GetUserByUsername(ctx, username)
	if existing.ID != 0 {
		return models.User{}, errors.New("username already exists")
	}

	id, err := s.client.Incr(ctx, "user:next_id").Result()
	if err != nil {
		return models.User{}, err
	}

	passwordHash, err := models.HashPassword(password)
	if err != nil {
		return models.User{}, err
	}

	user := models.User{
		ID:           int(id),
		Username:     username,
		PasswordHash: passwordHash,
		Role:         role,
		CreatedAt:    time.Now().UTC(),
	}

	data, err := json.Marshal(user)
	if err != nil {
		return models.User{}, err
	}

	pipe := s.client.Pipeline()
	pipe.Set(ctx, fmt.Sprintf("user:%d", user.ID), data, 0) // No TTL
	pipe.Set(ctx, fmt.Sprintf("user:username:%s", username), user.ID, 0)
	pipe.SAdd(ctx, "users", user.ID)
	_, err = pipe.Exec(ctx)

	return user, err
}

func (s *RedisStore) GetUser(ctx context.Context, id int) (models.User, error) {
	val, err := s.client.Get(ctx, fmt.Sprintf("user:%d", id)).Result()
	if err != nil {
		return models.User{}, err
	}

	var user models.User
	if err := json.Unmarshal([]byte(val), &user); err != nil {
		return models.User{}, err
	}

	return user, nil
}

func (s *RedisStore) GetUserByUsername(ctx context.Context, username string) (models.User, error) {
	idVal, err := s.client.Get(ctx, fmt.Sprintf("user:username:%s", username)).Result()
	if err != nil {
		return models.User{}, err
	}

	var id int
	fmt.Sscanf(idVal, "%d", &id)
	return s.GetUser(ctx, id)
}

func (s *RedisStore) GetUsers(ctx context.Context) ([]models.User, error) {
	ids, err := s.client.SMembers(ctx, "users").Result()
	if err != nil {
		return nil, err
	}

	var users []models.User
	for _, idStr := range ids {
		var id int
		fmt.Sscanf(idStr, "%d", &id)
		if user, err := s.GetUser(ctx, id); err == nil {
			users = append(users, user)
		}
	}
	return users, nil
}

func (s *RedisStore) UpdateUser(ctx context.Context, id int, username, role string) error {
	user, err := s.GetUser(ctx, id)
	if err != nil {
		return err
	}

	// Update username mapping if changed
	if user.Username != username {
		s.client.Del(ctx, fmt.Sprintf("user:username:%s", user.Username))
		s.client.Set(ctx, fmt.Sprintf("user:username:%s", username), id, 0)
		user.Username = username
	}

	user.Role = role

	data, err := json.Marshal(user)
	if err != nil {
		return err
	}

	return s.client.Set(ctx, fmt.Sprintf("user:%d", id), data, 0).Err()
}

func (s *RedisStore) DeleteUser(ctx context.Context, id int) error {
	user, err := s.GetUser(ctx, id)
	if err != nil {
		return err
	}

	pipe := s.client.Pipeline()
	pipe.Del(ctx, fmt.Sprintf("user:%d", id))
	pipe.Del(ctx, fmt.Sprintf("user:username:%s", user.Username))
	pipe.SRem(ctx, "users", id)
	_, err = pipe.Exec(ctx)

	return err
}

// Bot methods

func (s *RedisStore) CreateBot(ctx context.Context, name string, createdBy int) (models.Bot, error) {
	id, err := s.client.Incr(ctx, "bot:next_id").Result()
	if err != nil {
		return models.Bot{}, err
	}

	token, err := models.GenerateToken()
	if err != nil {
		return models.Bot{}, err
	}

	bot := models.Bot{
		ID:        int(id),
		Token:     token,
		Name:      name,
		CreatedAt: time.Now().UTC(),
		CreatedBy: createdBy,
	}

	data, err := json.Marshal(bot)
	if err != nil {
		return models.Bot{}, err
	}

	pipe := s.client.Pipeline()
	pipe.Set(ctx, fmt.Sprintf("bot:%d", bot.ID), data, 0)
	pipe.Set(ctx, fmt.Sprintf("bot:token:%s", token), bot.ID, 0)
	pipe.SAdd(ctx, "bots", bot.ID)
	_, err = pipe.Exec(ctx)

	return bot, err
}

func (s *RedisStore) GetBot(ctx context.Context, id int) (models.Bot, error) {
	val, err := s.client.Get(ctx, fmt.Sprintf("bot:%d", id)).Result()
	if err != nil {
		return models.Bot{}, err
	}

	var bot models.Bot
	if err := json.Unmarshal([]byte(val), &bot); err != nil {
		return models.Bot{}, err
	}

	return bot, nil
}

func (s *RedisStore) GetBotByToken(ctx context.Context, token string) (models.Bot, error) {
	idVal, err := s.client.Get(ctx, fmt.Sprintf("bot:token:%s", token)).Result()
	if err != nil {
		return models.Bot{}, err
	}

	var id int
	fmt.Sscanf(idVal, "%d", &id)
	return s.GetBot(ctx, id)
}

func (s *RedisStore) GetBots(ctx context.Context) ([]models.Bot, error) {
	ids, err := s.client.SMembers(ctx, "bots").Result()
	if err != nil {
		return nil, err
	}

	var bots []models.Bot
	for _, idStr := range ids {
		var id int
		fmt.Sscanf(idStr, "%d", &id)
		if bot, err := s.GetBot(ctx, id); err == nil {
			bots = append(bots, bot)
		}
	}
	return bots, nil
}

func (s *RedisStore) DeleteBot(ctx context.Context, id int) error {
	bot, err := s.GetBot(ctx, id)
	if err != nil {
		return err
	}

	pipe := s.client.Pipeline()
	pipe.Del(ctx, fmt.Sprintf("bot:%d", id))
	pipe.Del(ctx, fmt.Sprintf("bot:token:%s", bot.Token))
	pipe.SRem(ctx, "bots", id)
	_, err = pipe.Exec(ctx)

	return err
}

// Chat methods

func (s *RedisStore) CreateChat(ctx context.Context, chatID, name string, botID int) (models.Chat, error) {
	id, err := s.client.Incr(ctx, "chat:next_id").Result()
	if err != nil {
		return models.Chat{}, err
	}

	chat := models.Chat{
		ID:        int(id),
		ChatID:    chatID,
		Name:      name,
		BotID:     botID,
		CreatedAt: time.Now().UTC(),
	}

	data, err := json.Marshal(chat)
	if err != nil {
		return models.Chat{}, err
	}

	pipe := s.client.Pipeline()
	pipe.Set(ctx, fmt.Sprintf("chat:%d", chat.ID), data, 0)
	pipe.SAdd(ctx, "chats", chat.ID)
	_, err = pipe.Exec(ctx)

	return chat, err
}

func (s *RedisStore) GetChat(ctx context.Context, id int) (models.Chat, error) {
	val, err := s.client.Get(ctx, fmt.Sprintf("chat:%d", id)).Result()
	if err != nil {
		return models.Chat{}, err
	}

	var chat models.Chat
	if err := json.Unmarshal([]byte(val), &chat); err != nil {
		return models.Chat{}, err
	}

	return chat, nil
}

func (s *RedisStore) GetChats(ctx context.Context) ([]models.Chat, error) {
	ids, err := s.client.SMembers(ctx, "chats").Result()
	if err != nil {
		return nil, err
	}

	var chats []models.Chat
	for _, idStr := range ids {
		var id int
		fmt.Sscanf(idStr, "%d", &id)
		if chat, err := s.GetChat(ctx, id); err == nil {
			chats = append(chats, chat)
		}
	}
	return chats, nil
}

func (s *RedisStore) DeleteChat(ctx context.Context, id int) error {
	pipe := s.client.Pipeline()
	pipe.Del(ctx, fmt.Sprintf("chat:%d", id))
	pipe.SRem(ctx, "chats", id)
	_, err := pipe.Exec(ctx)

	return err
}
