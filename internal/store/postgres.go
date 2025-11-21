package store

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"

	"incident-viewer-go/internal/models"

	_ "github.com/lib/pq"
)

//go:embed schema.sql
var schemaSQL string

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(databaseURL string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &PostgresStore{db: db}, nil
}

// RunMigrations creates tables if they don't exist and applies schema updates
func (s *PostgresStore) RunMigrations(ctx context.Context) error {
	// Create tables
	if _, err := s.db.ExecContext(ctx, schemaSQL); err != nil {
		return err
	}

	// Apply migrations for existing tables
	migrations := []string{
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS totp_secret VARCHAR(255);`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS totp_enabled BOOLEAN DEFAULT FALSE;`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS last_password_change TIMESTAMP WITH TIME ZONE DEFAULT NOW();`,
	}

	for _, migration := range migrations {
		if _, err := s.db.ExecContext(ctx, migration); err != nil {
			// Log error but continue? Or fail?
			// For now, let's return error if migration fails, as it's critical.
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	return nil
}

// User methods

func (s *PostgresStore) CreateUser(ctx context.Context, username, password, role string) (models.User, error) {
	passwordHash, err := models.HashPassword(password)
	if err != nil {
		return models.User{}, err
	}

	var user models.User
	err = s.db.QueryRowContext(ctx,
		`INSERT INTO users (username, password_hash, role, created_at) 
		 VALUES ($1, $2, $3, NOW()) 
		 RETURNING id, username, password_hash, role, created_at`,
		username, passwordHash, role,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.CreatedAt)

	if err != nil {
		return models.User{}, err
	}

	return user, nil
}

func (s *PostgresStore) GetUser(ctx context.Context, id int) (models.User, error) {
	var user models.User
	var totpSecret sql.NullString
	var lastPasswordChange sql.NullTime

	err := s.db.QueryRowContext(ctx,
		`SELECT id, username, password_hash, role, totp_secret, totp_enabled, last_password_change, created_at FROM users WHERE id = $1`,
		id,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &totpSecret, &user.TOTPEnabled, &lastPasswordChange, &user.CreatedAt)

	if err == sql.ErrNoRows {
		return models.User{}, errors.New("user not found")
	}
	if err != nil {
		return models.User{}, err
	}

	if totpSecret.Valid {
		user.TOTPSecret = totpSecret.String
	}
	if lastPasswordChange.Valid {
		user.LastPasswordChange = lastPasswordChange.Time
	}

	return user, nil
}

func (s *PostgresStore) GetUserByUsername(ctx context.Context, username string) (models.User, error) {
	var user models.User
	var totpSecret sql.NullString
	var lastPasswordChange sql.NullTime

	err := s.db.QueryRowContext(ctx,
		`SELECT id, username, password_hash, role, totp_secret, totp_enabled, last_password_change, created_at FROM users WHERE username = $1`,
		username,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &totpSecret, &user.TOTPEnabled, &lastPasswordChange, &user.CreatedAt)

	if err == sql.ErrNoRows {
		return models.User{}, errors.New("user not found")
	}
	if err != nil {
		return models.User{}, err
	}

	if totpSecret.Valid {
		user.TOTPSecret = totpSecret.String
	}
	if lastPasswordChange.Valid {
		user.LastPasswordChange = lastPasswordChange.Time
	}

	return user, nil
}

func (s *PostgresStore) GetUsers(ctx context.Context) ([]models.User, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, username, password_hash, role, totp_secret, totp_enabled, last_password_change, created_at FROM users ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var user models.User
		var totpSecret sql.NullString
		var lastPasswordChange sql.NullTime

		if err := rows.Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &totpSecret, &user.TOTPEnabled, &lastPasswordChange, &user.CreatedAt); err != nil {
			continue
		}

		if totpSecret.Valid {
			user.TOTPSecret = totpSecret.String
		}
		if lastPasswordChange.Valid {
			user.LastPasswordChange = lastPasswordChange.Time
		}

		users = append(users, user)
	}

	return users, nil
}

func (s *PostgresStore) UpdateUser(ctx context.Context, id int, username, role string) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE users SET username = $1, role = $2 WHERE id = $3`,
		username, role, id,
	)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return errors.New("user not found")
	}

	return nil
}

func (s *PostgresStore) DeleteUser(ctx context.Context, id int) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, id)
	return err
}

// User profile & password management

func (s *PostgresStore) UpdateUserPassword(ctx context.Context, userID int, newPasswordHash string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET password_hash = $1, last_password_change = NOW() WHERE id = $2`,
		newPasswordHash, userID,
	)
	return err
}

func (s *PostgresStore) UpdateUserProfile(ctx context.Context, userID int, username string) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE users SET username = $1 WHERE id = $2`,
		username, userID,
	)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return errors.New("user not found")
	}

	return nil
}

// 2FA methods

func (s *PostgresStore) UpdateUser2FA(ctx context.Context, userID int, totpSecret string, enabled bool) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET totp_secret = $1, totp_enabled = $2 WHERE id = $3`,
		totpSecret, enabled, userID,
	)
	return err
}

func (s *PostgresStore) Disable2FA(ctx context.Context, userID int) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET totp_secret = NULL, totp_enabled = FALSE WHERE id = $1`,
		userID,
	)
	return err
}

// Bot methods

func (s *PostgresStore) CreateBot(ctx context.Context, name string, createdBy int) (models.Bot, error) {
	token, err := models.GenerateToken()
	if err != nil {
		return models.Bot{}, err
	}

	var bot models.Bot
	err = s.db.QueryRowContext(ctx,
		`INSERT INTO bots (token, name, created_by, created_at) 
		 VALUES ($1, $2, $3, NOW()) 
		 RETURNING id, token, name, created_by, created_at`,
		token, name, createdBy,
	).Scan(&bot.ID, &bot.Token, &bot.Name, &bot.CreatedBy, &bot.CreatedAt)

	return bot, err
}

func (s *PostgresStore) GetBot(ctx context.Context, id int) (models.Bot, error) {
	var bot models.Bot
	err := s.db.QueryRowContext(ctx,
		`SELECT id, token, name, created_by, created_at FROM bots WHERE id = $1`,
		id,
	).Scan(&bot.ID, &bot.Token, &bot.Name, &bot.CreatedBy, &bot.CreatedAt)

	if err == sql.ErrNoRows {
		return models.Bot{}, errors.New("bot not found")
	}
	return bot, err
}

func (s *PostgresStore) GetBotByToken(ctx context.Context, token string) (models.Bot, error) {
	var bot models.Bot
	err := s.db.QueryRowContext(ctx,
		`SELECT id, token, name, created_by, created_at FROM bots WHERE token = $1`,
		token,
	).Scan(&bot.ID, &bot.Token, &bot.Name, &bot.CreatedBy, &bot.CreatedAt)

	if err == sql.ErrNoRows {
		return models.Bot{}, errors.New("bot not found")
	}
	return bot, err
}

func (s *PostgresStore) GetBots(ctx context.Context) ([]models.Bot, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, token, name, created_by, created_at FROM bots ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bots []models.Bot
	for rows.Next() {
		var bot models.Bot
		if err := rows.Scan(&bot.ID, &bot.Token, &bot.Name, &bot.CreatedBy, &bot.CreatedAt); err != nil {
			continue
		}
		bots = append(bots, bot)
	}

	return bots, nil
}

func (s *PostgresStore) DeleteBot(ctx context.Context, id int) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM bots WHERE id = $1`, id)
	return err
}

// Chat methods

func (s *PostgresStore) CreateChat(ctx context.Context, chatID, name string, botID int) (models.Chat, error) {
	var chat models.Chat
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO chats (chat_id, name, bot_id, created_at) 
		 VALUES ($1, $2, $3, NOW()) 
		 RETURNING id, chat_id, name, bot_id, created_at`,
		chatID, name, botID,
	).Scan(&chat.ID, &chat.ChatID, &chat.Name, &chat.BotID, &chat.CreatedAt)

	return chat, err
}

func (s *PostgresStore) GetChat(ctx context.Context, id int) (models.Chat, error) {
	var chat models.Chat
	err := s.db.QueryRowContext(ctx,
		`SELECT id, chat_id, name, bot_id, created_at FROM chats WHERE id = $1`,
		id,
	).Scan(&chat.ID, &chat.ChatID, &chat.Name, &chat.BotID, &chat.CreatedAt)

	if err == sql.ErrNoRows {
		return models.Chat{}, errors.New("chat not found")
	}
	return chat, err
}

func (s *PostgresStore) GetChats(ctx context.Context) ([]models.Chat, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, chat_id, name, bot_id, created_at FROM chats ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chats []models.Chat
	for rows.Next() {
		var chat models.Chat
		if err := rows.Scan(&chat.ID, &chat.ChatID, &chat.Name, &chat.BotID, &chat.CreatedAt); err != nil {
			continue
		}
		chats = append(chats, chat)
	}

	return chats, nil
}

func (s *PostgresStore) DeleteChat(ctx context.Context, id int) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM chats WHERE id = $1`, id)
	return err
}

// User-Chat Permission methods

func (s *PostgresStore) AssignChatToUser(ctx context.Context, userID, chatID int) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO user_chat_permissions (user_id, chat_id, created_at)
		 VALUES ($1, $2, NOW())
		 ON CONFLICT (user_id, chat_id) DO NOTHING`,
		userID, chatID,
	)
	return err
}

func (s *PostgresStore) RemoveChatFromUser(ctx context.Context, userID, chatID int) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM user_chat_permissions WHERE user_id = $1 AND chat_id = $2`,
		userID, chatID,
	)
	return err
}

func (s *PostgresStore) GetUserChats(ctx context.Context, userID int) ([]models.Chat, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT c.id, c.chat_id, c.name, c.bot_id, c.created_at 
		 FROM chats c
		 INNER JOIN user_chat_permissions ucp ON c.id = ucp.chat_id
		 WHERE ucp.user_id = $1
		 ORDER BY c.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chats []models.Chat
	for rows.Next() {
		var chat models.Chat
		if err := rows.Scan(&chat.ID, &chat.ChatID, &chat.Name, &chat.BotID, &chat.CreatedAt); err != nil {
			continue
		}
		chats = append(chats, chat)
	}

	return chats, nil
}

func (s *PostgresStore) GetChatUsers(ctx context.Context, chatID int) ([]models.User, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT u.id, u.username, u.password_hash, u.role, u.created_at
		 FROM users u
		 INNER JOIN user_chat_permissions ucp ON u.id = ucp.user_id
		 WHERE ucp.chat_id = $1
		 ORDER BY u.username ASC`,
		chatID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var user models.User
		if err := rows.Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.CreatedAt); err != nil {
			continue
		}
		users = append(users, user)
	}

	return users, nil
}
