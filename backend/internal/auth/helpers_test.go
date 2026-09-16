package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func splitTokenParts(token string) ([]string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("token must have three parts")
	}
	return parts, nil
}

func TestTokenGenerationProducesJWTStructure(t *testing.T) {
	token, err := GenerateToken(3)
	if err != nil {
		t.Fatalf("GenerateToken returned error: %v", err)
	}

	parts, err := splitTokenParts(token)
	if err != nil {
		t.Fatalf("token does not have JWT structure: %v", err)
	}

	decoded, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("could not decode payload: %v", err)
	}

	var p payload
	if err := json.Unmarshal(decoded, &p); err != nil {
		t.Fatalf("could not unmarshal payload: %v", err)
	}

	if p.UserID != 3 {
		t.Fatalf("payload user ID = %d, want 3", p.UserID)
	}
	if p.Exp <= time.Now().Unix() {
		t.Fatal("token expiration should be in the future")
	}
}