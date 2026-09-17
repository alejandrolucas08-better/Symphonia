package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

var (
	ErrInvalidToken = errors.New("invalid or expired token")
	ErrMissingToken = errors.New("missing token")
)

const tokenLifetime = 72 * time.Hour

const minSigningKeyBytes = 32

const developmentSigningKey = "symphonia-dev-secret-change-in-production"

type payload struct {
	UserID int64 `json:"uid"`
	Exp    int64 `json:"exp"`
}

func signingKey() []byte {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = developmentSigningKey
	}
	return []byte(secret)
}

// ValidateConfiguration prevents the server from issuing tokens with a known
// or short signing key. It is called during application startup, while the
// fallback above remains available only to keep package-level tests isolated.
func ValidateConfiguration() error {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" || secret == developmentSigningKey || secret == "change-this-in-production" {
		return errors.New("JWT_SECRET must be a unique secret, not a development default")
	}
	if len([]byte(secret)) < minSigningKeyBytes {
		return fmt.Errorf("JWT_SECRET must contain at least %d bytes", minSigningKeyBytes)
	}
	return nil
}

func GenerateToken(userID int64) (string, error) {
	p := payload{
		UserID: userID,
		Exp:    time.Now().Add(tokenLifetime).Unix(),
	}

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))

	body, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}
	bodyEnc := base64.RawURLEncoding.EncodeToString(body)

	signing := hmac.New(sha256.New, signingKey())
	signing.Write([]byte(header + "." + bodyEnc))
	sig := base64.RawURLEncoding.EncodeToString(signing.Sum(nil))

	return header + "." + bodyEnc + "." + sig, nil
}

func ValidateToken(tokenStr string) (int64, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return 0, ErrInvalidToken
	}

	signing := hmac.New(sha256.New, signingKey())
	signing.Write([]byte(parts[0] + "." + parts[1]))
	expected := base64.RawURLEncoding.EncodeToString(signing.Sum(nil))

	if !hmac.Equal([]byte(parts[2]), []byte(expected)) {
		return 0, ErrInvalidToken
	}

	body, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return 0, ErrInvalidToken
	}

	var p payload
	if err := json.Unmarshal(body, &p); err != nil {
		return 0, ErrInvalidToken
	}

	if time.Now().Unix() > p.Exp {
		return 0, ErrInvalidToken
	}

	if p.UserID <= 0 {
		return 0, ErrInvalidToken
	}

	return p.UserID, nil
}
