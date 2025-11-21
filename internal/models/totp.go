package models

import (
	"bytes"
	"encoding/base64"
	"image/png"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// GenerateTOTPSecret generates a new TOTP secret for a user
func GenerateTOTPSecret(username, issuer string) (*otp.Key, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: username,
	})
	return key, err
}

// GenerateQRCode generates a base64-encoded PNG QR code from the TOTP key
func GenerateQRCode(key *otp.Key) (string, error) {
	var buf bytes.Buffer
	img, err := key.Image(200, 200)
	if err != nil {
		return "", err
	}

	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// VerifyTOTPCode verifies a TOTP code against a secret
func VerifyTOTPCode(secret, code string) bool {
	return totp.Validate(code, secret)
}
