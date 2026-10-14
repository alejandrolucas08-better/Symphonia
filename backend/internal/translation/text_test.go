package translation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/oauth2"
)

func TestTranslateTextDeveloperAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("x-goog-api-key") != "test-key" {
			t.Errorf("unexpected request method or API key: %s", r.Method)
		}
		var body struct {
			SystemInstruction struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"systemInstruction"`
			Contents []struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"contents"`
			GenerationConfig map[string]any `json:"generationConfig"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if len(body.Contents) != 1 || len(body.Contents[0].Parts) != 1 || body.Contents[0].Parts[0].Text != "Hello, Ada" {
			t.Errorf("unexpected contents: %+v", body.Contents)
		}
		if len(body.SystemInstruction.Parts) != 1 || !strings.Contains(body.SystemInstruction.Parts[0].Text, "pt-BR") {
			t.Errorf("unexpected translation instruction: %+v", body.SystemInstruction)
		}
		if _, ok := body.GenerationConfig["thinkingConfig"]; ok {
			t.Error("text request must not set unsupported thinkingConfig")
		}
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"Olá, Ada"}]}}]}`))
	}))
	defer server.Close()
	service, err := NewGeminiService(Config{Provider: ProviderGemini, GeminiAPIKey: "test-key", GeminiTextEndpoint: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	if service.textModel != DefaultTextTranslationModel {
		t.Fatalf("text model = %q, want %q", service.textModel, DefaultTextTranslationModel)
	}
	result, err := service.TranslateText(context.Background(), TextRequest{Text: "Hello, Ada", SourceLanguage: LanguageEnglish, TargetLanguage: LanguagePortuguese})
	if err != nil || result != "Olá, Ada" {
		t.Fatalf("TranslateText = %q, %v", result, err)
	}
}

func TestTranslateTextVertexAuthenticationAndFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" || r.Header.Get("x-goog-api-key") != "" {
			t.Errorf("unexpected authentication headers")
		}
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"status":"RESOURCE_EXHAUSTED","message":"quota exceeded"}}`))
	}))
	defer server.Close()
	service := &GeminiTranslationService{
		textEndpoint: server.URL,
		token: func(context.Context) (*oauth2.Token, error) {
			return &oauth2.Token{AccessToken: "test-token"}, nil
		},
	}
	_, err := service.TranslateText(context.Background(), TextRequest{Text: "Olá", SourceLanguage: LanguagePortuguese, TargetLanguage: LanguageEnglish})
	if err == nil || !strings.Contains(err.Error(), "429") || !strings.Contains(err.Error(), "quota exceeded") {
		t.Fatalf("expected upstream status, got %v", err)
	}
}

func TestTextTranslationModelCanBeOverridden(t *testing.T) {
	service, err := NewGeminiService(Config{
		Provider: ProviderGemini, GeminiAPIKey: "test-key", TextTranslationModel: "gemini-3.8-flash",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(service.textEndpoint, "/models/gemini-3.8-flash:generateContent") {
		t.Fatalf("text endpoint = %q", service.textEndpoint)
	}
}

func TestTranslateTextSameLanguageSkipsProvider(t *testing.T) {
	service := &GeminiTranslationService{}
	result, err := service.TranslateText(context.Background(), TextRequest{Text: "Olá", SourceLanguage: LanguagePortuguese, TargetLanguage: LanguagePortuguese})
	if err != nil || result != "Olá" {
		t.Fatalf("TranslateText = %q, %v", result, err)
	}
}
