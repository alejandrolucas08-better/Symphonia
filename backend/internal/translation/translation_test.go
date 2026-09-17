package translation

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestSupportedLanguages(t *testing.T) {
	for _, language := range SupportedLanguages {
		if !language.Valid() {
			t.Errorf("%q reported invalid", language)
		}
	}
	for _, language := range []Language{"", "en-US", "DE-DE", "JA-JP"} {
		if language.Valid() {
			t.Errorf("Valid(%q) = true, want false", language)
		}
	}
}

func TestValidateRequest(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Request)
		wantErr error
	}{
		{"valid request", func(r *Request) {}, nil},
		{"missing id", func(r *Request) { r.ID = "" }, ErrInvalidRequest},
		{"too long id", func(r *Request) { r.ID = strings.Repeat("x", 65) }, ErrInvalidRequest},
		{"missing source", func(r *Request) { r.SourceLanguage = "" }, ErrInvalidRequest},
		{"unsupported source", func(r *Request) { r.SourceLanguage = "DE-DE" }, ErrUnsupportedLanguage},
		{"unsupported target", func(r *Request) { r.TargetLanguage = "JA-JP" }, ErrUnsupportedLanguage},
		{"empty audio", func(r *Request) { r.Audio.Data = nil }, ErrEmptyAudio},
		{"unsupported codec", func(r *Request) { r.Audio.Format.Codec = "MP3" }, ErrInvalidAudioFormat},
		{"zero sample rate", func(r *Request) { r.Audio.Format.SampleRate = 0 }, ErrInvalidAudioFormat},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := validRequest()
			test.mutate(&request)
			err := ValidateRequest(request)
			switch {
			case test.wantErr == nil && err != nil:
				t.Fatalf("ValidateRequest = %v, want nil", err)
			case test.wantErr != nil && err == nil:
				t.Fatal("ValidateRequest = nil, want an error")
			case test.wantErr != nil && !errors.Is(err, test.wantErr):
				t.Fatalf("ValidateRequest = %v, want %v", err, test.wantErr)
			}
		})
	}
}

func TestMockServiceTranslateHappyPath(t *testing.T) {
	service := NewMockService()
	if service.Name() != string(ProviderMock) {
		t.Fatalf("Name() = %q, want %q", service.Name(), ProviderMock)
	}

	request := validRequest()
	result, err := service.Translate(context.Background(), request)
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	if result.ID != request.ID {
		t.Errorf("ID = %q, want %q", result.ID, request.ID)
	}
	if result.SourceLanguage != LanguagePortuguese {
		t.Errorf("SourceLanguage = %q, want %q", result.SourceLanguage, LanguagePortuguese)
	}
	if result.TargetLanguage != LanguageEnglish {
		t.Errorf("TargetLanguage = %q, want %q", result.TargetLanguage, LanguageEnglish)
	}
	if result.Transcription.Text != "Olá, seja bem-vindo ao Symphonia." {
		t.Errorf("Transcription.Text = %q", result.Transcription.Text)
	}
	if result.Transcription.Language != LanguagePortuguese {
		t.Errorf("Transcription.Language = %q, want %q", result.Transcription.Language, LanguagePortuguese)
	}
	if result.Transcription.Confidence != mockConfidence {
		t.Errorf("Transcription.Confidence = %v, want %v", result.Transcription.Confidence, mockConfidence)
	}
	if result.TranslatedText != "Hello, welcome to Symphonia." {
		t.Errorf("TranslatedText = %q", result.TranslatedText)
	}
	if result.TranslatedAudio.Format != request.Audio.Format {
		t.Errorf("TranslatedAudio.Format = %+v, want %+v", result.TranslatedAudio.Format, request.Audio.Format)
	}
	expectedSamples := len(request.Audio.Data) / (request.Audio.Format.Channels * request.Audio.Format.BytesPerSample)
	if want := expectedSamples * 2; len(result.TranslatedAudio.Data) != want {
		t.Errorf("TranslatedAudio.Data length = %d, want %d", len(result.TranslatedAudio.Data), want)
	}
	if result.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}
}

func TestMockServiceValidatesBeforeWork(t *testing.T) {
	service := NewMockService()
	_, err := service.Translate(context.Background(), Request{ID: "req-1", TargetLanguage: LanguageSpanish})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Translate = %v, want ErrInvalidRequest", err)
	}
}

func TestNewSelectsProvider(t *testing.T) {
	service, err := New(Config{Provider: ProviderMock})
	if err != nil {
		t.Fatalf("New(mock) = %v, want nil", err)
	}
	if service.Name() != string(ProviderMock) {
		t.Fatalf("Name() = %q, want %q", service.Name(), ProviderMock)
	}

	if service, err := New(Config{}); err != nil || service == nil {
		t.Fatalf("New(empty config) = %v/%v, want mock service", service, err)
	}

	if _, err := New(Config{Provider: ProviderGemini}); !errors.Is(err, ErrGeminiNotConfigured) {
		t.Fatalf("New(gemini) = %v, want ErrGeminiNotConfigured", err)
	}

	if _, err := New(Config{Provider: "deep-seek"}); !errors.Is(err, ErrUnknownProvider) {
		t.Fatalf("New(unknown) = %v, want ErrUnknownProvider", err)
	}
}

func TestLoadConfig(t *testing.T) {
	t.Setenv("TRANSLATION_PROVIDER", "gemini")
	t.Setenv("GEMINI_API_KEY", "test-only-key")
	config := LoadConfig()
	if config.Provider != ProviderGemini {
		t.Errorf("Provider = %q, want %q", config.Provider, ProviderGemini)
	}
	if config.GeminiAPIKey != "test-only-key" {
		t.Errorf("GeminiAPIKey = %q, want %q", config.GeminiAPIKey, "test-only-key")
	}
}

func TestLoadConfigDefaultsToMock(t *testing.T) {
	t.Setenv("TRANSLATION_PROVIDER", "")
	t.Setenv("GEMINI_API_KEY", "")
	if config := LoadConfig(); config.Provider != ProviderMock {
		t.Errorf("Provider = %q, want %q", config.Provider, ProviderMock)
	}

	t.Setenv("TRANSLATION_PROVIDER", "MOCK")
	if config := LoadConfig(); config.Provider != ProviderMock {
		t.Errorf("Provider = %q, want %q", config.Provider, ProviderMock)
	}

	t.Setenv("TRANSLATION_PROVIDER", "typo")
	if config := LoadConfig(); config.Provider != "typo" {
		t.Errorf("Provider = %q, want typo", config.Provider)
	}
}

func validRequest() Request {
	return Request{
		ID:             "req-1",
		SourceLanguage: LanguagePortuguese,
		TargetLanguage: LanguageEnglish,
		Audio: AudioInput{
			Format: DefaultAudioFormat,
			Data:   make([]byte, 640),
		},
	}
}
