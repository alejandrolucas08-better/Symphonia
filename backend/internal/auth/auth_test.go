package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

func TestGenerateAndValidateToken(t *testing.T) {
	token, err := GenerateToken(42)
	if err != nil {
		t.Fatalf("GenerateToken returned error: %v", err)
	}
	if token == "" {
		t.Fatal("GenerateToken returned empty token")
	}

	userID, err := ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken returned error: %v", err)
	}
	if userID != 42 {
		t.Fatalf("ValidateToken returned wrong user ID: got %d, want 42", userID)
	}
}

func TestValidateTokenRejectsTampered(t *testing.T) {
	token, err := GenerateToken(1)
	if err != nil {
		t.Fatalf("GenerateToken returned error: %v", err)
	}

	tampered := token[:len(token)-2] + "xx"

	if _, err := ValidateToken(tampered); err == nil {
		t.Fatal("ValidateToken accepted tampered token")
	}
}

func TestValidateTokenRejectsGarbage(t *testing.T) {
	if _, err := ValidateToken("not.a.token"); err == nil {
		t.Fatal("ValidateToken accepted garbage input")
	}
	if _, err := ValidateToken(""); err == nil {
		t.Fatal("ValidateToken accepted empty input")
	}
}

func TestValidateTokenRejectsExpired(t *testing.T) {
	p := payload{
		UserID: 7,
		Exp:    time.Now().Add(-time.Hour).Unix(),
	}

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	body, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	bodyEnc := base64.RawURLEncoding.EncodeToString(body)

	signing := hmac.New(sha256.New, signingKey())
	signing.Write([]byte(header + "." + bodyEnc))
	sig := base64.RawURLEncoding.EncodeToString(signing.Sum(nil))

	token := header + "." + bodyEnc + "." + sig

	if _, err := ValidateToken(token); err == nil {
		t.Fatal("ValidateToken accepted expired token")
	}
}

func TestValidateConfiguration(t *testing.T) {
	for _, test := range []struct {
		name    string
		secret  string
		wantErr bool
	}{
		{name: "missing", wantErr: true},
		{name: "development default", secret: developmentSigningKey, wantErr: true},
		{name: "short", secret: "too-short", wantErr: true},
		{name: "secure", secret: "0123456789abcdef0123456789abcdef"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("JWT_SECRET", test.secret)
			err := ValidateConfiguration()
			if (err != nil) != test.wantErr {
				t.Fatalf("ValidateConfiguration() error = %v, wantErr %t", err, test.wantErr)
			}
		})
	}
}
