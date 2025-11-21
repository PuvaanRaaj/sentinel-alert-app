package handlers

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
)

// validateSharedSecret checks X-Sentinel-Signature against HMAC-SHA256(body, secret).
// If WEBHOOK_SECRET is empty, validation is skipped (returns true).
func validateSharedSecret(r *http.Request) bool {
	secret := os.Getenv("WEBHOOK_SECRET")
	if secret == "" {
		return true
	}
	sig := r.Header.Get("X-Sentinel-Signature")
	if sig == "" {
		return false
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return false
	}
	r.Body = io.NopCloser(bytes.NewBuffer(body)) // restore for downstream handlers

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(sig), []byte(expected))
}
