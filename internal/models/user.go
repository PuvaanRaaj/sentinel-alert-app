package models

import (
	"time"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID                 int       `json:"id"`
	Username           string    `json:"username"`
	PasswordHash       string    `json:"-"`
	Role               string    `json:"role"` // "admin" or "user"
	TOTPSecret         string    `json:"-"`
	TOTPEnabled        bool      `json:"totp_enabled"`
	LastPasswordChange time.Time `json:"last_password_change,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
}

// HashPassword generates bcrypt hash of the password
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPassword compares password with hash
func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
	return err == nil
}
