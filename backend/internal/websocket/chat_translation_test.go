package websocket

import (
	"context"
	"errors"
	"testing"

	"github.com/institucional/symphonia/backend/internal/call"
	"github.com/institucional/symphonia/backend/internal/translation"
)

type fakeTextTranslator struct {
	requests chan translation.TextRequest
	result   string
	err      error
}

func (f *fakeTextTranslator) TranslateText(_ context.Context, request translation.TextRequest) (string, error) {
	f.requests <- request
	return f.result, f.err
}

func TestChatTranslationDeliveredOnlyToListener(t *testing.T) {
	server, callValue, hub := newTestServer(t)
	callValue.Participants[1].HeardLanguage = call.LanguagePortuguese
	translator := &fakeTextTranslator{requests: make(chan translation.TextRequest, 1), result: "Olá"}
	hub.textTranslator = translator
	sender := connectAndJoin(t, server.URL, 1, "join-1")
	defer sender.CloseNow()
	peer := connectAndJoin(t, server.URL, 2, "join-2")
	defer peer.CloseNow()
	_ = readEnvelope(t, sender)

	writeClient(t, sender, `{"version":1,"type":"message","request_id":"msg","data":{"text":"Hello"}}`)
	var sent, received struct {
		ID                string `json:"id"`
		Text              string `json:"text"`
		TranslationStatus string `json:"translation_status"`
	}
	sentEvent := readEnvelope(t, sender)
	receivedEvent := readEnvelope(t, peer)
	if sentEvent.Type != EventMessage || receivedEvent.Type != EventMessage {
		t.Fatalf("message events = %s, %s", sentEvent.Type, receivedEvent.Type)
	}
	decodeEventData(t, sentEvent, &sent)
	decodeEventData(t, receivedEvent, &received)
	if sent.ID == "" || sent.ID != received.ID || sent.Text != "Hello" || received.Text != "Hello" || sent.TranslationStatus != "none" || received.TranslationStatus != "pending" {
		t.Fatalf("message states = %+v, %+v", sent, received)
	}
	request := <-translator.requests
	if request.Text != "Hello" || request.SourceLanguage != translation.LanguageEnglish || request.TargetLanguage != translation.LanguagePortuguese {
		t.Fatalf("translation request = %+v", request)
	}
	update := readEnvelope(t, peer)
	if update.Type != EventMessageTranslation {
		t.Fatalf("update event = %s", update.Type)
	}
	var data struct {
		MessageID string `json:"message_id"`
		Text      string `json:"text"`
		Language  string `json:"language"`
		Status    string `json:"status"`
	}
	decodeEventData(t, update, &data)
	if data.MessageID != sent.ID || data.Text != "Olá" || data.Language != "PT-BR" || data.Status != "ready" {
		t.Fatalf("translation update = %+v", data)
	}
	assertNoFrame(t, sender)
}

func TestChatTranslationFailurePreservesOriginal(t *testing.T) {
	server, callValue, hub := newTestServer(t)
	callValue.Participants[1].HeardLanguage = call.LanguagePortuguese
	hub.textTranslator = &fakeTextTranslator{requests: make(chan translation.TextRequest, 1), err: errors.New("provider failed")}
	sender := connectAndJoin(t, server.URL, 1, "join-1")
	defer sender.CloseNow()
	peer := connectAndJoin(t, server.URL, 2, "join-2")
	defer peer.CloseNow()
	_ = readEnvelope(t, sender)
	writeClient(t, sender, `{"version":1,"type":"message","request_id":"msg","data":{"text":"Hello"}}`)
	_ = readEnvelope(t, sender)
	message := readEnvelope(t, peer)
	var original struct {
		Text string `json:"text"`
	}
	decodeEventData(t, message, &original)
	update := readEnvelope(t, peer)
	var result struct {
		Text   string `json:"text"`
		Status string `json:"status"`
	}
	decodeEventData(t, update, &result)
	if original.Text != "Hello" || result.Text != "" || result.Status != "unavailable" {
		t.Fatalf("original/update = %+v, %+v", original, result)
	}
}
