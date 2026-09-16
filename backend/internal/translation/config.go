package translation

import (
	"fmt"
	"os"
	"strings"
)

// Provider names a concrete implementation of Service.
type Provider string

const (
	ProviderMock   Provider = "mock"
	ProviderGemini Provider = "gemini"
)

// Config holds the settings required to build a Service.
type Config struct {
	// Provider selects the Service implementation.
	Provider Provider
	// GeminiAPIKey is the key required by the Gemini implementation.
	GeminiAPIKey string
	// TranslationModel overrides the default Live Translate model.
	TranslationModel string
	// GeminiLiveEndpoint overrides the default BidiGenerateContent endpoint.
	// Intended for proxies and local harnesses in tests.
	GeminiLiveEndpoint string
}

// LoadConfig builds Config from environment variables.
//
//   - TRANSLATION_PROVIDER selects the Service implementation (default "mock").
//   - GEMINI_API_KEY is required by the Gemini implementation.
//   - TRANSLATION_MODEL overrides the default Live Translate model.
//   - GEMINI_LIVE_ENDPOINT overrides the default BidiGenerateContent endpoint.
func LoadConfig() Config {
	return Config{
		Provider:           providerFromEnv(os.Getenv("TRANSLATION_PROVIDER")),
		GeminiAPIKey:       os.Getenv("GEMINI_API_KEY"),
		TranslationModel:   os.Getenv("TRANSLATION_MODEL"),
		GeminiLiveEndpoint: os.Getenv("GEMINI_LIVE_ENDPOINT"),
	}
}

// New builds the Service selected by the config. Callers must depend on
// Service, never on a concrete provider. The Gemini value set indicates the
// returned service also satisfies SessionOpener.
func New(config Config) (Service, error) {
	switch config.Provider {
	case ProviderMock, "":
		return NewMockService(), nil
	case ProviderGemini:
		return NewGeminiService(config)
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknownProvider, config.Provider)
	}
}

func providerFromEnv(value string) Provider {
	switch Provider(strings.ToLower(strings.TrimSpace(value))) {
	case ProviderGemini:
		return ProviderGemini
	default:
		return ProviderMock
	}
}
