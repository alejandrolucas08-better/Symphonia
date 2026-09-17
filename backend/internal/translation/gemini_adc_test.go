package translation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"golang.org/x/oauth2"
)

func TestVertexAuthSetupAndCancellation(t *testing.T) {
	for _, target := range []Language{LanguagePortuguese, LanguageEnglish} {
		t.Run(string(target), func(t *testing.T) {
			closed := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer close(closed)
				if r.Header.Get("Authorization") != "Bearer test-token" || r.URL.RawQuery != "" || r.Header.Get("x-goog-api-key") != "" {
					t.Error("invalid ADC authentication")
				}
				conn, err := websocket.Accept(w, r, nil)
				if err != nil {
					return
				}
				defer conn.CloseNow()
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				_, payload, err := conn.Read(ctx)
				if err != nil {
					t.Error(err)
					return
				}
				var msg struct {
					Setup map[string]json.RawMessage `json:"setup"`
				}
				if err := json.Unmarshal(payload, &msg); err != nil {
					t.Error(err)
					return
				}
				if string(msg.Setup["model"]) != `"projects/test-project/locations/global/publishers/google/models/`+DefaultTranslationModel+`"` {
					t.Error("invalid Vertex model resource")
				}
				if string(msg.Setup["inputAudioTranscription"]) != "{}" || string(msg.Setup["outputAudioTranscription"]) != "{}" {
					t.Error("transcription must be in setup")
				}
				var generation map[string]json.RawMessage
				_ = json.Unmarshal(msg.Setup["generationConfig"], &generation)
				language, _ := geminiLanguageCode(target)
				if !strings.Contains(string(generation["translationConfig"]), `"targetLanguageCode":"`+language+`"`) {
					t.Error("wrong target language")
				}
				_ = conn.Write(ctx, websocket.MessageText, []byte(`{"setupComplete":{}}`))
				_, _, _ = conn.Read(ctx)
			}))
			defer server.Close()
			service, err := NewGeminiService(Config{Provider: ProviderGemini, GoogleCloudProject: "test-project", GeminiAPIKey: "must-not-be-used", GeminiLiveEndpoint: "ws" + strings.TrimPrefix(server.URL, "http")})
			if err != nil {
				t.Fatal(err)
			}
			service.token = func(context.Context) (*oauth2.Token, error) { return &oauth2.Token{AccessToken: "test-token"}, nil }
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			session, err := service.OpenLiveSession(ctx, OpenRequest{ID: "adc-test", TargetLanguage: target})
			if err != nil {
				t.Fatal(err)
			}
			cancel()
			select {
			case <-closed:
			case <-time.After(4 * time.Second):
				t.Fatal("cancellation leaked provider connection")
			}
			for range session.Events() {
			}
		})
	}
}
