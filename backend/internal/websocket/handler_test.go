package websocket

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	coderws "github.com/coder/websocket"
	"github.com/institucional/symphonia/backend/internal/auth"
	"github.com/institucional/symphonia/backend/internal/call"
)

type fakeCallStore struct {
	value         *call.Call
	err           error
	revalidateErr error
	reads         int
}

func (f *fakeCallStore) GetForActiveParticipant(_ context.Context, _ string, userID int64) (*call.Call, error) {
	f.reads++
	if f.err != nil {
		return nil, f.err
	}
	if f.reads > 1 && f.revalidateErr != nil {
		return nil, f.revalidateErr
	}
	for _, participant := range f.value.Participants {
		if participant.UserID == userID {
			return f.value, nil
		}
	}
	return nil, call.ErrNotParticipant
}

func TestBinaryBeforeJoinIsRejectedAndClosed(t *testing.T) {
	server, callValue, _ := newTestServer(t)
	socket := dialTestSocket(t, server.URL, callValue.Participants[0].UserID)
	defer socket.CloseNow()
	writeBinary(t, socket, testAudioFrame(1, 0, 0))
	event := readEnvelope(t, socket)
	if event.Type != EventError {
		t.Fatalf("event = %q, want error", event.Type)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _, err := socket.Read(ctx)
	if coderws.CloseStatus(err) != coderws.StatusUnsupportedData {
		t.Fatalf("close status = %v, want %v (error: %v)", coderws.CloseStatus(err), coderws.StatusUnsupportedData, err)
	}
}

func TestJoinRevalidatesMembershipAfterUpgrade(t *testing.T) {
	server, callValue, _ := newTestServerWithStore(t, &fakeCallStore{revalidateErr: call.ErrNotParticipant})
	socket := dialTestSocket(t, server.URL, callValue.Participants[0].UserID)
	defer socket.CloseNow()
	writeClient(t, socket, `{"version":1,"type":"join_call","request_id":"join-stale","data":{}}`)
	event := readEnvelope(t, socket)
	if event.Type != EventError || event.RequestID != "join-stale" {
		t.Fatalf("event = %q/%q, want error/join-stale", event.Type, event.RequestID)
	}
	var data struct {
		Code string `json:"code"`
	}
	decodeEventData(t, event, &data)
	if data.Code != "join_rejected" {
		t.Fatalf("error code = %q, want join_rejected", data.Code)
	}
}

func TestAudioRelayBidirectionalWithoutEcho(t *testing.T) {
	server, callValue, _ := newTestServer(t)
	first := connectAndJoin(t, server.URL, callValue.Participants[0].UserID, "join-1")
	defer first.CloseNow()
	second := connectAndJoin(t, server.URL, callValue.Participants[1].UserID, "join-2")
	defer second.CloseNow()
	if event := readEnvelope(t, first); event.Type != EventParticipantJoined {
		t.Fatalf("event = %q, want participant_joined", event.Type)
	}

	fromFirst := testAudioFrame(1, 2, 320)
	fromFirst[AudioHeaderSize] = 0x7f
	writeBinary(t, first, fromFirst)
	assertBinaryFrame(t, second, fromFirst)

	fromSecond := testAudioFrame(2, 9, 640)
	fromSecond[AudioHeaderSize] = 0x42
	writeBinary(t, second, fromSecond)
	assertBinaryFrame(t, first, fromSecond)
	assertNoFrame(t, second)
}

func TestAudioMutedAndMalformedFramesAreDropped(t *testing.T) {
	server, callValue, _ := newTestServer(t)
	first := connectAndJoin(t, server.URL, callValue.Participants[0].UserID, "join-1")
	defer first.CloseNow()
	second := connectAndJoin(t, server.URL, callValue.Participants[1].UserID, "join-2")
	defer second.CloseNow()
	_ = readEnvelope(t, first)

	malformed := testAudioFrame(1, 1, 0)
	malformed[4] = 0x80
	writeBinary(t, first, malformed)
	event := readEnvelope(t, first)
	var errorData struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	decodeEventData(t, event, &errorData)
	if event.Type != EventError || errorData.Code != "invalid_audio_frame" || errorData.Message != "invalid audio frame" {
		t.Fatalf("malformed frame response = %q/%+v", event.Type, errorData)
	}

	writeClient(t, first, `{"version":1,"type":"mute_state","request_id":"mute","data":{"muted":true}}`)
	if event := readEnvelope(t, first); event.Type != EventMuteState {
		t.Fatalf("sender event = %q, want mute_state", event.Type)
	}
	if event := readEnvelope(t, second); event.Type != EventMuteState {
		t.Fatalf("peer event = %q, want mute_state", event.Type)
	}
	writeBinary(t, first, testAudioFrame(1, 2, 320))
	assertNoFrame(t, second)
}

func TestHandlerHandshakeValidation(t *testing.T) {
	server, callValue, _ := newTestServer(t)
	token, _ := auth.GenerateToken(callValue.Participants[0].UserID)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, response, err := coderws.Dial(ctx, wsURL(server.URL), &coderws.DialOptions{
		HTTPHeader:   http.Header{"Origin": []string{server.URL}},
		Subprotocols: []string{"auth." + token},
	})
	if err == nil || response == nil || response.StatusCode != http.StatusBadRequest {
		t.Fatalf("missing version handshake error/status = %v/%v", err, responseStatus(response))
	}

	_, response, err = coderws.Dial(ctx, wsURL(server.URL), &coderws.DialOptions{
		HTTPHeader:   http.Header{"Origin": []string{"https://evil.example"}},
		Subprotocols: []string{Subprotocol, "auth." + token},
	})
	if err == nil || response == nil || response.StatusCode != http.StatusForbidden {
		t.Fatalf("invalid origin handshake error/status = %v/%v", err, responseStatus(response))
	}
}

func TestHubMessageMuteFanoutAndCleanup(t *testing.T) {
	server, callValue, hub := newTestServer(t)
	first := connectAndJoin(t, server.URL, callValue.Participants[0].UserID, "join-1")
	defer first.CloseNow()
	second := connectAndJoin(t, server.URL, callValue.Participants[1].UserID, "join-2")

	joined := readEnvelope(t, first)
	if joined.Type != EventParticipantJoined {
		t.Fatalf("event = %q, want participant_joined", joined.Type)
	}

	writeClient(t, first, `{"version":1,"type":"message","request_id":"m1","data":{"text":"  hello 世界  "}}`)
	for _, socket := range []*coderws.Conn{first, second} {
		event := readEnvelope(t, socket)
		if event.Type != EventMessage {
			t.Fatalf("event = %q, want message", event.Type)
		}
		var data struct {
			UserID int64  `json:"user_id"`
			Text   string `json:"text"`
		}
		decodeEventData(t, event, &data)
		if data.UserID != 1 || data.Text != "hello 世界" {
			t.Fatalf("message = %+v", data)
		}
	}

	writeClient(t, second, `{"version":1,"type":"mute_state","request_id":"mute-1","data":{"muted":true}}`)
	for _, socket := range []*coderws.Conn{first, second} {
		event := readEnvelope(t, socket)
		if event.Type != EventMuteState {
			t.Fatalf("event = %q, want mute_state", event.Type)
		}
	}

	if err := second.Close(coderws.StatusNormalClosure, "bye"); err != nil {
		t.Fatalf("close second socket: %v", err)
	}
	left := readEnvelope(t, first)
	if left.Type != EventParticipantLeft {
		t.Fatalf("event = %q, want participant_left", left.Type)
	}
	var leftData struct {
		UserID int64  `json:"user_id"`
		Reason string `json:"reason"`
	}
	decodeEventData(t, left, &leftData)
	if leftData.UserID != 2 || leftData.Reason != "disconnected" {
		t.Fatalf("participant_left = %+v", leftData)
	}
	hub.mu.Lock()
	connections := len(hub.rooms[callValue.Code].connections)
	_, staleMute := hub.rooms[callValue.Code].muteStates[2]
	hub.mu.Unlock()
	if connections != 1 || staleMute {
		t.Fatalf("room cleanup left %d connections, stale mute = %v", connections, staleMute)
	}
}

func TestHubReplacementDoesNotEmitParticipantLeft(t *testing.T) {
	server, callValue, _ := newTestServer(t)
	observer := connectAndJoin(t, server.URL, callValue.Participants[1].UserID, "observer")
	defer observer.CloseNow()
	original := connectAndJoin(t, server.URL, callValue.Participants[0].UserID, "original")
	defer original.CloseNow()
	if event := readEnvelope(t, observer); event.Type != EventParticipantJoined {
		t.Fatalf("event = %q, want participant_joined", event.Type)
	}

	replacement := connectAndJoin(t, server.URL, callValue.Participants[0].UserID, "replacement")
	defer replacement.CloseNow()
	if event := readEnvelope(t, observer); event.Type != EventParticipantJoined {
		t.Fatalf("replacement event = %q, want participant_joined", event.Type)
	}
	ctx, closeCancel := context.WithTimeout(context.Background(), 2*time.Second)
	_, _, err := original.Read(ctx)
	closeCancel()
	if coderws.CloseStatus(err) != statusConnectionReplaced {
		t.Fatalf("replaced close status = %v, want %v (error: %v)", coderws.CloseStatus(err), statusConnectionReplaced, err)
	}
	frame := testAudioFrame(1, 4, 960)
	writeBinary(t, replacement, frame)
	assertBinaryFrame(t, observer, frame)

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	_, payload, err := observer.Read(ctx)
	if err == nil {
		var event ServerEnvelope
		_ = json.Unmarshal(payload, &event)
		t.Fatalf("unexpected event after replacement: %q", event.Type)
	}
}

func newTestServer(t *testing.T) (*httptest.Server, *call.Call, *Hub) {
	t.Helper()
	return newTestServerWithStore(t, &fakeCallStore{})
}

func newTestServerWithStore(t *testing.T, store *fakeCallStore) (*httptest.Server, *call.Call, *Hub) {
	t.Helper()
	t.Setenv("JWT_SECRET", "websocket-test-secret")
	now := time.Now().UTC()
	value := &call.Call{
		ID: 10, Code: "room1234", HostUserID: 1, Status: call.StatusActive, CreatedAt: now, UpdatedAt: now,
		Participants: []call.Participant{
			{UserID: 1, Name: "Ada", SpokenLanguage: call.LanguageEnglish, HeardLanguage: call.LanguagePortuguese, JoinedAt: now},
			{UserID: 2, Name: "Bia", SpokenLanguage: call.LanguagePortuguese, HeardLanguage: call.LanguageEnglish, JoinedAt: now},
		},
	}
	store.value = value
	hub := NewHub()
	handler := NewHandler(store, hub)
	mux := http.NewServeMux()
	mux.Handle("GET /api/calls/{code}/ws", handler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server, value, hub
}

func connectAndJoin(t *testing.T, serverURL string, userID int64, requestID string) *coderws.Conn {
	t.Helper()
	socket := dialTestSocket(t, serverURL, userID)
	writeClient(t, socket, `{"version":1,"type":"join_call","request_id":"`+requestID+`","data":{}}`)
	event := readEnvelope(t, socket)
	if event.Type != EventJoinCall || event.RequestID != requestID {
		t.Fatalf("join response = %q/%q", event.Type, event.RequestID)
	}
	return socket
}

func dialTestSocket(t *testing.T, serverURL string, userID int64) *coderws.Conn {
	t.Helper()
	token, err := auth.GenerateToken(userID)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	socket, response, err := coderws.Dial(ctx, wsURL(serverURL), &coderws.DialOptions{
		HTTPHeader:   http.Header{"Origin": []string{serverURL}},
		Subprotocols: []string{Subprotocol, "auth." + token},
	})
	if err != nil {
		t.Fatalf("dial websocket (status %v): %v", responseStatus(response), err)
	}
	if socket.Subprotocol() != Subprotocol {
		t.Fatalf("subprotocol = %q, want %q", socket.Subprotocol(), Subprotocol)
	}
	return socket
}

func writeClient(t *testing.T, socket *coderws.Conn, body string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := socket.Write(ctx, coderws.MessageText, []byte(body)); err != nil {
		t.Fatalf("write websocket: %v", err)
	}
}

func writeBinary(t *testing.T, socket *coderws.Conn, payload []byte) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := socket.Write(ctx, coderws.MessageBinary, payload); err != nil {
		t.Fatalf("write binary websocket frame: %v", err)
	}
}

func assertBinaryFrame(t *testing.T, socket *coderws.Conn, expected []byte) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	messageType, payload, err := socket.Read(ctx)
	if err != nil {
		t.Fatalf("read binary websocket frame: %v", err)
	}
	if messageType != coderws.MessageBinary || !bytes.Equal(payload, expected) {
		t.Fatalf("binary frame type/contents did not match: type=%v size=%d", messageType, len(payload))
	}
}

func assertNoFrame(t *testing.T, socket *coderws.Conn) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if messageType, payload, err := socket.Read(ctx); err == nil {
		t.Fatalf("unexpected websocket frame: type=%v size=%d", messageType, len(payload))
	}
}

func readEnvelope(t *testing.T, socket *coderws.Conn) ServerEnvelope {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, payload, err := socket.Read(ctx)
	if err != nil {
		t.Fatalf("read websocket: %v", err)
	}
	var event ServerEnvelope
	if err := json.Unmarshal(payload, &event); err != nil {
		t.Fatalf("decode server envelope: %v", err)
	}
	if event.Version != ProtocolVersion || event.OccurredAt.IsZero() {
		t.Fatalf("invalid server envelope: %+v", event)
	}
	return event
}

func decodeEventData(t *testing.T, event ServerEnvelope, destination any) {
	t.Helper()
	payload, err := json.Marshal(event.Data)
	if err != nil || json.Unmarshal(payload, destination) != nil {
		t.Fatalf("decode event data: %v", err)
	}
}

func wsURL(serverURL string) string {
	return "ws" + strings.TrimPrefix(serverURL, "http") + "/api/calls/room1234/ws"
}

func responseStatus(response *http.Response) any {
	if response == nil {
		return nil
	}
	return response.StatusCode
}
