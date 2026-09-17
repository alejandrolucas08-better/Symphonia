package websocket

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	coderws "github.com/coder/websocket"
	"github.com/institucional/symphonia/backend/internal/call"
	"github.com/institucional/symphonia/backend/internal/translation"
)

func TestHubTranslatesAudioAndRelaysRawWhenNotNeeded(t *testing.T) {
	t.Setenv("JWT_SECRET", "websocket-test-secret")
	now := time.Now().UTC()
	value := &call.Call{
		ID: 10, Code: "room1234", HostUserID: 1, Status: call.StatusActive, CreatedAt: now, UpdatedAt: now,
		Participants: []call.Participant{
			{UserID: 1, Name: "Ada", SpokenLanguage: call.LanguageEnglish, HeardLanguage: call.LanguagePortuguese, JoinedAt: now},
			{UserID: 2, Name: "Bia", SpokenLanguage: call.LanguagePortuguese, HeardLanguage: call.LanguageSpanish, JoinedAt: now},
		},
	}
	store := &fakeCallStore{}
	store.value = value
	hub := NewHub()
	handler := NewHandler(store, hub, translation.NewMockService())
	mux := http.NewServeMux()
	mux.Handle("GET /api/calls/{code}/ws", handler)
	server := httptest.NewServer(mux)
	defer server.Close()

	ada := connectAndJoin(t, server.URL, 1, "join-ada")
	defer ada.CloseNow()
	bia := connectAndJoin(t, server.URL, 2, "join-bia")
	defer bia.CloseNow()
	_ = readEnvelope(t, ada) // participant_joined for bia reaching ada

	// Bia speaks Portuguese; Ada hears Portuguese -> raw relay, no session.
	fromBia := testAudioFrame(2, 1, 320)
	writeBinary(t, bia, fromBia)
	assertBinaryFrame(t, ada, fromBia)

	// Ada speaks English; Bia hears Spanish -> must be translated.
	frame := testAudioFrame(1, 2, 320)
	writeBinary(t, ada, frame)

	assertTranscription := func(event ServerEnvelope, wantType string, wantLanguage translation.Language) {
		t.Helper()
		if event.Type != wantType {
			t.Fatalf("event = %q, want %q", event.Type, wantType)
		}
		var data transcriptionEvent
		decodeEventData(t, event, &data)
		if data.UserID != 1 {
			t.Errorf("user_id = %d, want 1", data.UserID)
		}
		if data.Language != wantLanguage {
			t.Errorf("language = %q, want %q", data.Language, wantLanguage)
		}
	}

	input := readEnvelope(t, bia)
	assertTranscription(input, EventInputTranscription, translation.LanguageEnglish)
	var inputText transcriptionEvent
	decodeEventData(t, input, &inputText)
	if inputText.Text == "" {
		t.Error("input transcription text is empty")
	}
	output := readEnvelope(t, bia)
	assertTranscription(output, EventOutputTranscription, translation.LanguageSpanish)
	var outputText transcriptionEvent
	decodeEventData(t, output, &outputText)
	if outputText.Text == "" {
		t.Error("output transcription text is empty")
	}

	audio := readTranslatedFrame(t, bia)
	_ = readTranslatedFrame(t, bia) // padded remainder of the mock's 640 samples at 24 kHz
	if bytes.Equal(audio, frame) {
		t.Fatal("original audio relayed instead of translation")
	}

	// The raw binary frame must never be relayed when translation is active.
	assertNoFrame(t, bia)
}

func TestHubReopensSessionWhenHeardLanguageChanges(t *testing.T) {
	t.Setenv("JWT_SECRET", "websocket-test-secret")
	now := time.Now().UTC()
	value := &call.Call{
		ID: 10, Code: "room1234", HostUserID: 1, Status: call.StatusActive, CreatedAt: now, UpdatedAt: now,
		Participants: []call.Participant{
			{UserID: 1, Name: "Ada", SpokenLanguage: call.LanguageEnglish, HeardLanguage: call.LanguagePortuguese, JoinedAt: now},
			{UserID: 2, Name: "Bia", SpokenLanguage: call.LanguagePortuguese, HeardLanguage: call.LanguageSpanish, JoinedAt: now},
		},
	}
	store := &fakeCallStore{}
	store.value = value
	hub := NewHub()
	handler := NewHandler(store, hub, translation.NewMockService())
	mux := http.NewServeMux()
	mux.Handle("GET /api/calls/{code}/ws", handler)
	server := httptest.NewServer(mux)
	defer server.Close()

	ada := connectAndJoin(t, server.URL, 1, "join-ada")
	defer ada.CloseNow()
	bia := connectAndJoin(t, server.URL, 2, "join-bia")
	defer bia.CloseNow()
	_ = readEnvelope(t, ada) // participant_joined for bia reaching ada

	writeBinary(t, ada, testAudioFrame(1, 1, 320))
	// First generation: Ada's English is translated into Spanish.
	_ = readEnvelope(t, bia) // input transcription
	output := readEnvelope(t, bia)
	var first transcriptionEvent
	decodeEventData(t, output, &first)
	if first.Language != translation.LanguageSpanish {
		t.Fatalf("first output language = %q, want %q", first.Language, translation.LanguageSpanish)
	}
	_ = readTranslatedFrame(t, bia)
	_ = readTranslatedFrame(t, bia)

	// Bia switches her heard language to French; sessions must be recreated.
	hub.LanguageChanged(value.Code, call.Participant{
		UserID: 2, Name: "Bia", SpokenLanguage: call.LanguagePortuguese, HeardLanguage: call.LanguageFrench, JoinedAt: now,
	})
	langEvent := readEnvelope(t, bia)
	if langEvent.Type != EventLanguageChanged {
		t.Fatalf("event = %q, want %q", langEvent.Type, EventLanguageChanged)
	}

	writeBinary(t, ada, testAudioFrame(1, 2, 320))
	_ = readEnvelope(t, bia) // input transcription
	secondOutput := readEnvelope(t, bia)
	var second transcriptionEvent
	decodeEventData(t, secondOutput, &second)
	if second.Language != translation.LanguageFrench {
		t.Fatalf("second output language = %q, want %q", second.Language, translation.LanguageFrench)
	}
}

func TestHubClosesSessionsWhenRoomTeardown(t *testing.T) {
	t.Setenv("JWT_SECRET", "websocket-test-secret")
	now := time.Now().UTC()
	value := &call.Call{
		ID: 10, Code: "room1234", HostUserID: 1, Status: call.StatusActive, CreatedAt: now, UpdatedAt: now,
		Participants: []call.Participant{
			{UserID: 1, Name: "Ada", SpokenLanguage: call.LanguageEnglish, HeardLanguage: call.LanguagePortuguese, JoinedAt: now},
			{UserID: 2, Name: "Bia", SpokenLanguage: call.LanguagePortuguese, HeardLanguage: call.LanguageSpanish, JoinedAt: now},
		},
	}
	store := &fakeCallStore{}
	store.value = value
	hub := NewHub()
	handler := NewHandler(store, hub, translation.NewMockService())
	mux := http.NewServeMux()
	mux.Handle("GET /api/calls/{code}/ws", handler)
	server := httptest.NewServer(mux)
	defer server.Close()

	ada := connectAndJoin(t, server.URL, 1, "join-ada")
	defer ada.CloseNow()
	bia := connectAndJoin(t, server.URL, 2, "join-bia")
	defer bia.CloseNow()
	_ = readEnvelope(t, ada) // participant_joined for bia reaching ada

	writeBinary(t, ada, testAudioFrame(1, 1, 320))
	_ = readEnvelope(t, bia) // input transcription
	_ = readEnvelope(t, bia) // output transcription
	_ = readTranslatedFrame(t, bia)
	_ = readTranslatedFrame(t, bia)

	if err := bia.Close(coderws.StatusNormalClosure, "bye"); err != nil {
		t.Fatalf("close bia socket: %v", err)
	}
	_ = readEnvelope(t, ada) // participant_left

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = ada.Close(coderws.StatusNormalClosure, "bye")
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		hub.mu.Lock()
		room := hub.rooms[value.Code]
		hub.mu.Unlock()
		if room == nil {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("room was never removed after all clients left")
		case <-time.After(10 * time.Millisecond):
		}
	}
	t.Fatal("room was never removed after all clients left")
}

func readTranslatedFrame(t *testing.T, socket *coderws.Conn) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	kind, data, err := socket.Read(ctx)
	if err != nil || kind != coderws.MessageBinary || !ValidateAudioFrame(data) {
		t.Fatalf("expected translated PCM16 16 kHz binary frame, kind=%v err=%v", kind, err)
	}
	return data
}
