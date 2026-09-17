package translation

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	coderws "github.com/coder/websocket"
)

func TestMockLiveSessionStreamsEvents(t *testing.T) {
	service := NewMockService()
	session, err := service.OpenLiveSession(context.Background(), OpenRequest{
		ID:             "sess-1",
		SourceLanguage: LanguagePortuguese,
		TargetLanguage: LanguageEnglish,
		EchoTarget:     true,
	})
	if err != nil {
		t.Fatalf("OpenLiveSession: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = session.Close(ctx)
	}()

	input := AudioInput{Format: DefaultAudioFormat, Data: make([]byte, 640)}
	if err := session.SendAudio(context.Background(), input); err != nil {
		t.Fatalf("SendAudio: %v", err)
	}

	wantKinds := []struct {
		kind  EventKind
		check func(*testing.T, LiveEvent)
	}{
		{EventInputTranscription, func(t *testing.T, event LiveEvent) {
			t.Helper()
			if event.SourceText != "Olá, seja bem-vindo ao Symphonia." {
				t.Errorf("SourceText = %q", event.SourceText)
			}
			if event.SourceLanguage != LanguagePortuguese {
				t.Errorf("SourceLanguage = %q", event.SourceLanguage)
			}
		}},
		{EventOutputTranscription, func(t *testing.T, event LiveEvent) {
			t.Helper()
			if event.OutputText != "Hello, welcome to Symphonia." {
				t.Errorf("OutputText = %q", event.OutputText)
			}
			if event.OutputLanguage != LanguageEnglish {
				t.Errorf("OutputLanguage = %q", event.OutputLanguage)
			}
		}},
		{EventTranslatedAudio, func(t *testing.T, event LiveEvent) {
			t.Helper()
			if event.Audio.Format != GeminiOutputFormat {
				t.Errorf("Audio.Format = %+v, want %+v", event.Audio.Format, GeminiOutputFormat)
			}
			if len(event.Audio.Data) == 0 {
				t.Error("Audio.Data is empty")
			}
		}},
		{EventTurnComplete, func(*testing.T, LiveEvent) {}},
	}
	for _, want := range wantKinds {
		select {
		case event := <-session.Events():
			if event.Kind != want.kind {
				t.Fatalf("event kind = %v, want %v", event.Kind, want.kind)
			}
			want.check(t, event)
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for event kind %v", want.kind)
		}
	}
}

func TestMockLiveSessionRejectsEmptyAndClosed(t *testing.T) {
	service := NewMockService()
	session, err := service.OpenLiveSession(context.Background(), OpenRequest{
		ID:             "sess-2",
		TargetLanguage: LanguageSpanish,
	})
	if err != nil {
		t.Fatalf("OpenLiveSession: %v", err)
	}

	if err := session.SendAudio(context.Background(), AudioInput{}); !errors.Is(err, ErrEmptyAudio) {
		t.Fatalf("SendAudio(empty) = %v, want ErrEmptyAudio", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := session.Close(ctx); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := session.SendAudio(ctx, AudioInput{Format: DefaultAudioFormat, Data: make([]byte, 640)}); !errors.Is(err, ErrSessionClosed) {
		t.Fatalf("SendAudio after Close = %v, want ErrSessionClosed", err)
	}
	if _, err := service.OpenLiveSession(context.Background(), OpenRequest{
		ID:             "sess-bad",
		TargetLanguage: "JA-JP",
	}); !errors.Is(err, ErrUnsupportedLanguage) {
		t.Fatalf("OpenLiveSession(bad target) = %v, want ErrUnsupportedLanguage", err)
	}
}

func TestMockLiveSessionValidatesRequest(t *testing.T) {
	service := NewMockService()
	session, err := service.OpenLiveSession(context.Background(), OpenRequest{ID: "ok", TargetLanguage: LanguageEnglish})
	if err != nil {
		t.Fatalf("OpenLiveSession(valid) = %v, want nil", err)
	}
	defer session.Close(context.Background())
	if _, err := service.OpenLiveSession(context.Background(), OpenRequest{}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("OpenLiveSession(empty) = %v, want ErrInvalidRequest", err)
	}
}

func TestGeminiSessionTerminatesOnProviderClose(t *testing.T) {
	for _, beforeSetup := range []bool{true, false} {
		t.Run(fmt.Sprint("beforeSetup=", beforeSetup), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := coderws.Accept(w, r, nil)
				if err != nil {
					return
				}
				defer conn.CloseNow()
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				if _, _, err := conn.Read(ctx); err != nil {
					return
				}
				if !beforeSetup {
					_ = conn.Write(ctx, coderws.MessageText, []byte(`{"setupComplete":{}}`))
				}
				_ = conn.Close(coderws.StatusNormalClosure, "finished")
			}))
			defer server.Close()
			service, err := NewGeminiService(Config{Provider: ProviderGemini, GeminiAPIKey: "test-only-key", GeminiLiveEndpoint: "ws" + strings.TrimPrefix(server.URL, "http")})
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			session, err := service.OpenLiveSession(ctx, OpenRequest{ID: "close-test", TargetLanguage: LanguageEnglish})
			if beforeSetup {
				if !errors.Is(err, ErrSessionClosed) {
					t.Fatalf("setup error = %v, want ErrSessionClosed", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			defer session.Close(ctx)
			select {
			case _, ok := <-session.Events():
				if ok {
					t.Fatal("expected closed events after normal provider close")
				}
			case <-ctx.Done():
				t.Fatal("events channel remained open after provider close")
			}
		})
	}
}

func TestGeminiLanguageCodes(t *testing.T) {
	for _, test := range []struct {
		symphonia Language
		want      string
	}{
		{LanguagePortuguese, "pt-BR"},
		{LanguageEnglish, "en"},
		{LanguageSpanish, "es"},
		{LanguageFrench, "fr"},
	} {
		got, err := geminiLanguageCode(test.symphonia)
		if err != nil || got != test.want {
			t.Errorf("geminiLanguageCode(%q) = %q/%v, want %q", test.symphonia, got, err, test.want)
		}
	}
	if _, err := geminiLanguageCode("DE-DE"); !errors.Is(err, ErrUnsupportedLanguage) {
		t.Errorf("geminiLanguageCode(unsupported) = %v, want ErrUnsupportedLanguage", err)
	}
}

func TestNewGeminiServiceRequiresKey(t *testing.T) {
	if _, err := NewGeminiService(Config{Provider: ProviderGemini}); !errors.Is(err, ErrGeminiNotConfigured) {
		t.Fatalf("NewGeminiService = %v, want ErrGeminiNotConfigured", err)
	}
	_, err := NewGeminiService(Config{Provider: ProviderMock, GeminiAPIKey: "x"})
	if err == nil {
		t.Fatal("NewGeminiService(mock provider) = nil, want error")
	}
}

func TestGeminiSessionAgainstFakeServer(t *testing.T) {
	var (
		mu           sync.Mutex
		gotAPIKey    string
		gotQueryKey  string
		gotModel     string
		gotTarget    string
		gotEcho      bool
		audioPushes  int
		gotStreamEnd bool
		serverErr    error
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := coderws.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()

		mu.Lock()
		gotAPIKey = r.Header.Get("x-goog-api-key")
		gotQueryKey = r.URL.Query().Get("key")
		mu.Unlock()

		readCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for {
			messageType, payload, err := conn.Read(readCtx)
			if err != nil {
				if coderws.CloseStatus(err) != coderws.StatusNormalClosure {
					mu.Lock()
					serverErr = err
					mu.Unlock()
				}
				return
			}
			if messageType != coderws.MessageText {
				continue
			}
			var message struct {
				Setup         map[string]json.RawMessage `json:"setup"`
				RealtimeInput struct {
					Audio struct {
						Data string `json:"data"`
					} `json:"audio"`
					AudioStreamEnd bool `json:"audioStreamEnd"`
				} `json:"realtimeInput"`
			}
			if err := json.Unmarshal(payload, &message); err != nil {
				mu.Lock()
				serverErr = err
				mu.Unlock()
				return
			}
			switch {
			case message.Setup != nil:
				var generationConfig struct {
					TranslationConfig struct {
						TargetLanguageCode string `json:"targetLanguageCode"`
						EchoTargetLanguage bool   `json:"echoTargetLanguage"`
					} `json:"translationConfig"`
				}
				_ = json.Unmarshal(message.Setup["generationConfig"], &generationConfig)
				mu.Lock()
				gotModel = string(message.Setup["model"])
				gotTarget = generationConfig.TranslationConfig.TargetLanguageCode
				gotEcho = generationConfig.TranslationConfig.EchoTargetLanguage
				mu.Unlock()
				writeCtx, writeCancel := context.WithTimeout(context.Background(), 2*time.Second)
				_ = conn.Write(writeCtx, coderws.MessageText, []byte(`{"setupComplete":{}}`))
				writeCancel()
			case message.RealtimeInput.Audio.Data != "":
				mu.Lock()
				audioPushes++
				mu.Unlock()
			case message.RealtimeInput.AudioStreamEnd:
				mu.Lock()
				gotStreamEnd = true
				mu.Unlock()
				writeCtx, writeCancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer writeCancel()
				_ = conn.Write(writeCtx, coderws.MessageText, []byte(
					`{"serverContent":{"inputTranscription":{"text":"Olá, seja bem-vindo.","languageCode":"pt-BR"},"outputTranscription":{"text":"Hello, welcome.","languageCode":"en"}}}`))
				translated := base64.StdEncoding.EncodeToString(make([]byte, 480))
				_ = conn.Write(writeCtx, coderws.MessageText, []byte(
					`{"serverContent":{"modelTurn":{"parts":[{"inlineData":{"mimeType":"audio/pcm;rate=24000","data":"`+translated+`"}}]},"turnComplete":true}}`))
			}
		}
	}))
	defer server.Close()

	endpoint := "ws" + strings.TrimPrefix(server.URL, "http")
	service, err := NewGeminiService(Config{
		Provider:           ProviderGemini,
		GeminiAPIKey:       "test-only-key",
		GeminiLiveEndpoint: endpoint,
	})
	if err != nil {
		t.Fatalf("NewGeminiService: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	request := OpenRequest{
		ID:             "fake-1",
		SourceLanguage: LanguagePortuguese,
		TargetLanguage: LanguageEnglish,
		EchoTarget:     true,
	}
	session, err := service.OpenLiveSession(ctx, request)
	if err != nil {
		t.Fatalf("OpenLiveSession: %v", err)
	}
	defer func() {
		_ = session.Close(context.Background())
	}()

	mu.Lock()
	gotAPIKeyCopy := gotAPIKey
	gotQueryKeyCopy := gotQueryKey
	gotModelCopy := gotModel
	gotTargetCopy := gotTarget
	gotEchoCopy := gotEcho
	mu.Unlock()
	if gotAPIKeyCopy != "test-only-key" {
		t.Errorf("api key header = %q, want test-only-key", gotAPIKeyCopy)
	}
	if gotQueryKeyCopy != "test-only-key" {
		t.Errorf("api key query = %q, want test-only-key", gotQueryKeyCopy)
	}
	if !strings.Contains(gotModelCopy, DefaultTranslationModel) {
		t.Errorf("setup model = %q, want to contain %q", gotModelCopy, DefaultTranslationModel)
	}
	if gotTargetCopy != "en" {
		t.Errorf("target language code = %q, want en", gotTargetCopy)
	}
	if !gotEchoCopy {
		t.Error("echoTargetLanguage = false, want true")
	}

	for range 5 {
		if err := session.SendAudio(ctx, AudioInput{Format: DefaultAudioFormat, Data: make([]byte, 640)}); err != nil {
			t.Fatalf("SendAudio: %v", err)
		}
	}
	if live, ok := session.(*geminiLiveSession); ok {
		if err := live.signalEnd(ctx); err != nil {
			t.Fatalf("signalEnd: %v", err)
		}
	}

	wantEvents := []EventKind{EventInputTranscription, EventOutputTranscription, EventTranslatedAudio, EventTurnComplete}
	for _, want := range wantEvents {
		select {
		case event := <-session.Events():
			if event.Kind != want {
				t.Fatalf("event = %v, want %v", event.Kind, want)
			}
		case <-ctx.Done():
			t.Fatalf("timed out waiting for event %v: %v", want, ctx.Err())
		}
	}

	mu.Lock()
	audioPushesCopy := audioPushes
	gotStreamEndCopy := gotStreamEnd
	serverErrCopy := serverErr
	mu.Unlock()
	if audioPushesCopy != 1 {
		t.Errorf("audio pushes = %d, want 1", audioPushesCopy)
	}
	if !gotStreamEndCopy {
		t.Error("server did not observe audioStreamEnd")
	}
	if serverErrCopy != nil {
		t.Errorf("server error: %v", serverErrCopy)
	}
}

func TestGeminiLiveIntegration(t *testing.T) {
	if os.Getenv("GEMINI_LIVE_INTEGRATION") != "1" {
		t.Skip("set GEMINI_LIVE_INTEGRATION=1 to call the real Gemini Live API")
	}
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Fatal("GEMINI_API_KEY is required for the live integration test")
	}
	service, err := NewGeminiService(Config{
		Provider:         ProviderGemini,
		GeminiAPIKey:     apiKey,
		TranslationModel: os.Getenv("TRANSLATION_MODEL"),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	session, err := service.OpenLiveSession(ctx, OpenRequest{
		ID:             "live-integration",
		SourceLanguage: LanguagePortuguese,
		TargetLanguage: LanguageEnglish,
		EchoTarget:     true,
	})
	if err != nil {
		t.Fatalf("connect to Gemini Live: %v", err)
	}
	if err := session.SendAudio(ctx, AudioInput{Format: DefaultAudioFormat, Data: make([]byte, geminiChunkBytes)}); err != nil {
		t.Fatalf("send audio to Gemini Live: %v", err)
	}
	if err := session.Close(ctx); err != nil {
		t.Fatalf("close Gemini Live session: %v", err)
	}
}

func TestGeminiSessionRejectsUnsupportedAudio(t *testing.T) {
	session := &geminiLiveSession{done: make(chan struct{})}
	for _, audio := range []AudioInput{
		{Format: AudioFormat{Codec: AudioCodecPCM16LE, SampleRate: 44100, Channels: 1, BytesPerSample: 2}, Data: make([]byte, 640)},
		{Format: DefaultAudioFormat, Data: []byte{1}},
	} {
		if err := session.SendAudio(context.Background(), audio); !errors.Is(err, ErrInvalidAudioFormat) {
			t.Errorf("SendAudio(%+v) = %v, want ErrInvalidAudioFormat", audio.Format, err)
		}
	}
}
