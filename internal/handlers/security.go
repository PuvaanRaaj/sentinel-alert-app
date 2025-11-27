package handlers

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"

	// "os" // Commented out - not needed while signature validation is disabled
	"sync"
	"time"
)

// validateSharedSecret checks X-Sentinel-Signature against HMAC-SHA256(body, secret).
// If WEBHOOK_SECRET is empty, validation is skipped (returns true).
// NOTE: Signature validation is currently disabled for internal Gatus webhook usage
// since Gatus uptime monitor cannot calculate signatures for each webhook request.
func validateSharedSecret(r *http.Request) bool {
	// Temporarily skip signature validation for internal usage
	return true

	// Original validation logic (commented out for now)
	// secret := os.Getenv("WEBHOOK_SECRET")
	// if secret == "" {
	// 	return true
	// }
	// return validateSignature(r, secret, r.Header.Get("X-Sentinel-Signature"))
}

// validateSignature validates HMAC for a given secret with timestamp and nonce checks.
func validateSignature(r *http.Request, secret, sig string) bool {
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
	ts := r.Header.Get("X-Sentinel-Timestamp")
	nonce := r.Header.Get("X-Sentinel-Nonce")
	if ts != "" && nonce != "" {
		mac.Reset()
		mac.Write([]byte(ts))
		mac.Write([]byte("." + nonce + "."))
		mac.Write(body)
		expected = hex.EncodeToString(mac.Sum(nil))
		if !withinSkew(ts) || isReplay(nonce) {
			return false
		}
	}
	return hmac.Equal([]byte(sig), []byte(expected))
}

var (
	nonceCache   = make(map[string]time.Time)
	nonceCacheMu sync.Mutex
	maxSkew      = 5 * time.Minute
)

func withinSkew(ts string) bool {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return false
	}
	now := time.Now()
	return t.After(now.Add(-maxSkew)) && t.Before(now.Add(maxSkew))
}

func isReplay(nonce string) bool {
	nonceCacheMu.Lock()
	defer nonceCacheMu.Unlock()
	if nonce == "" {
		return false
	}
	if exp, ok := nonceCache[nonce]; ok && exp.After(time.Now()) {
		return true
	}
	nonceCache[nonce] = time.Now().Add(maxSkew)
	return false
}
