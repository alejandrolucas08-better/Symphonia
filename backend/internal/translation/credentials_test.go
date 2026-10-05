package translation

import (
	"strings"
	"testing"
)

func TestHostedCredentialsValidation(t *testing.T) {
	for _, credentials := range []string{"private-secret-invalid-json", `{"type":"authorized_user","refresh_token":"private-secret"}`} {
		_, err := NewGeminiService(Config{Provider: ProviderGemini, GoogleCloudProject: "test", GoogleCredentialsJSON: credentials})
		if err == nil || strings.Contains(err.Error(), "private-secret") {
			t.Fatalf("expected a sanitized credentials error, got %v", err)
		}
	}
	t.Setenv("GOOGLE_CREDENTIALS_JSON", `{"type":"service_account","client_email":"test@example.com","private_key":"test-only","token_uri":"https://oauth2.googleapis.com/token"}`)
	t.Setenv("GOOGLE_CLOUD_PROJECT", "test")
	t.Setenv("TRANSLATION_PROVIDER", "gemini")
	service, err := NewGeminiService(LoadConfig())
	if err != nil || service.token == nil {
		t.Fatalf("hosted credentials not configured: %v", err)
	}
}
