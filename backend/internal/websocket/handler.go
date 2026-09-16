package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strings"
	"unicode/utf8"

	coderws "github.com/coder/websocket"
	"github.com/institucional/symphonia/backend/internal/auth"
	"github.com/institucional/symphonia/backend/internal/call"
)

type callStore interface {
	GetForActiveParticipant(context.Context, string, int64) (*call.Call, error)
}

type Handler struct {
	calls          callStore
	hub            *Hub
	allowedOrigins map[string]struct{}
}

func NewHandler(calls callStore, hub *Hub) *Handler {
	origins := os.Getenv("WS_ALLOWED_ORIGINS")
	if strings.TrimSpace(origins) == "" {
		origins = "http://localhost:5173,http://127.0.0.1:5173"
	}
	allowed := make(map[string]struct{})
	for _, origin := range strings.Split(origins, ",") {
		if origin = strings.TrimSpace(origin); origin != "" {
			allowed[origin] = struct{}{}
		}
	}
	return &Handler{calls: calls, hub: hub, allowedOrigins: allowed}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.validOrigin(r) {
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return
	}
	token, ok := authProtocol(r.Header.Values("Sec-WebSocket-Protocol"))
	if !ok {
		http.Error(w, "required WebSocket protocols are missing", http.StatusBadRequest)
		return
	}
	userID, err := auth.ValidateToken(token)
	if err != nil {
		http.Error(w, "invalid or expired token", http.StatusUnauthorized)
		return
	}
	currentCall, err := h.calls.GetForActiveParticipant(r.Context(), r.PathValue("code"), userID)
	if err != nil {
		switch {
		case errors.Is(err, call.ErrCallNotFound):
			http.Error(w, "call not found", http.StatusNotFound)
		case errors.Is(err, call.ErrCallEnded):
			http.Error(w, "call has ended", http.StatusConflict)
		case errors.Is(err, call.ErrNotParticipant):
			http.Error(w, "not an active participant", http.StatusForbidden)
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}
	participantValue, ok := participant(currentCall, userID)
	if !ok {
		http.Error(w, "not an active participant", http.StatusForbidden)
		return
	}

	socket, err := coderws.Accept(w, r, &coderws.AcceptOptions{
		Subprotocols:       []string{Subprotocol},
		InsecureSkipVerify: true,
	})
	if err != nil {
		return
	}
	defer socket.CloseNow()
	socket.SetReadLimit(MaxFrameBytes)
	lifetimeCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	messageType, payload, err := socket.Read(lifetimeCtx)
	if err != nil {
		return
	}
	if messageType != coderws.MessageText {
		sendSocketError(socket, "", "invalid_envelope", "first frame must be text join_call")
		_ = socket.Close(coderws.StatusUnsupportedData, "text frames required")
		return
	}
	envelope, err := DecodeClient(payload)
	if err != nil {
		sendProtocolError(socket, err)
		_ = socket.Close(coderws.StatusPolicyViolation, "join_call required")
		return
	}
	if envelope.Type != EventJoinCall || !emptyObject(envelope.Data) {
		sendSocketError(socket, envelope.RequestID, "join_required", "first event must be join_call with empty data")
		_ = socket.Close(coderws.StatusPolicyViolation, "join_call required")
		return
	}
	currentCall, err = h.calls.GetForActiveParticipant(lifetimeCtx, currentCall.Code, userID)
	if err != nil {
		sendSocketError(socket, envelope.RequestID, "join_rejected", "call membership is no longer active")
		_ = socket.Close(coderws.StatusPolicyViolation, "join_call rejected")
		return
	}
	participantValue, ok = participant(currentCall, userID)
	if !ok {
		sendSocketError(socket, envelope.RequestID, "join_rejected", "call membership is no longer active")
		_ = socket.Close(coderws.StatusPolicyViolation, "join_call rejected")
		return
	}
	client, err := h.hub.add(currentCall.Code, participantValue, socket, envelope.RequestID, currentCall)
	if err != nil {
		return
	}
	defer h.hub.remove(client, true)
	h.readLoop(lifetimeCtx, client)
}

func (h *Handler) readLoop(ctx context.Context, client *connection) {
	for {
		messageType, payload, err := client.socket.Read(ctx)
		if err != nil {
			return
		}
		if messageType == coderws.MessageBinary {
			if !ValidateAudioFrame(payload) {
				client.sendError("", "invalid_audio_frame", "invalid audio frame")
				continue
			}
			h.hub.audio(client, payload)
			continue
		}
		if messageType != coderws.MessageText {
			client.sendError("", "invalid_envelope", "unsupported frame type")
			continue
		}
		envelope, err := DecodeClient(payload)
		if err != nil {
			client.sendProtocolError(err)
			continue
		}
		switch envelope.Type {
		case EventJoinCall:
			client.sendError(envelope.RequestID, "already_joined", "join_call is only valid as the first event")
		case EventMessage:
			var data struct {
				Text string `json:"text"`
			}
			if decodeData(envelope.Data, &data) != nil {
				client.sendError(envelope.RequestID, "invalid_data", "message data must contain only text")
				continue
			}
			data.Text = strings.TrimSpace(data.Text)
			if count := utf8.RuneCountInString(data.Text); count < 1 || count > 1000 {
				client.sendError(envelope.RequestID, "invalid_data", "text must contain 1 to 1000 Unicode codepoints")
				continue
			}
			h.hub.message(client, data.Text)
		case EventMuteState:
			var data struct {
				Muted *bool `json:"muted"`
			}
			if decodeData(envelope.Data, &data) != nil || data.Muted == nil {
				client.sendError(envelope.RequestID, "invalid_data", "mute_state data must contain only muted")
				continue
			}
			h.hub.mute(client, *data.Muted)
		}
	}
}

func authProtocol(headers []string) (string, bool) {
	foundVersion := false
	token := ""
	for _, header := range headers {
		for _, protocol := range strings.Split(header, ",") {
			protocol = strings.TrimSpace(protocol)
			switch {
			case protocol == Subprotocol:
				foundVersion = true
			case strings.HasPrefix(protocol, "auth."):
				if token != "" || len(protocol) == len("auth.") {
					return "", false
				}
				token = strings.TrimPrefix(protocol, "auth.")
			}
		}
	}
	return token, foundVersion && token != ""
}

func (h *Handler) validOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil {
		return false
	}
	expectedScheme := "http"
	if r.TLS != nil {
		expectedScheme = "https"
	}
	if parsed.Scheme == expectedScheme && parsed.Host == r.Host {
		return true
	}
	_, ok := h.allowedOrigins[origin]
	return ok
}

func participant(currentCall *call.Call, userID int64) (call.Participant, bool) {
	for _, value := range currentCall.Participants {
		if value.UserID == userID {
			return value, true
		}
	}
	return call.Participant{}, false
}

func emptyObject(data json.RawMessage) bool {
	var value map[string]json.RawMessage
	return json.Unmarshal(data, &value) == nil && len(value) == 0
}

func sendProtocolError(socket *coderws.Conn, err error) {
	var protocolErr *ProtocolError
	if errors.As(err, &protocolErr) {
		sendSocketError(socket, protocolErr.RequestID, protocolErr.Code, protocolErr.Message)
	}
}

func sendSocketError(socket *coderws.Conn, requestID, code, message string) {
	client := &connection{socket: socket}
	client.sendError(requestID, code, message)
}

func (c *connection) sendProtocolError(err error) {
	var protocolErr *ProtocolError
	if errors.As(err, &protocolErr) {
		c.sendError(protocolErr.RequestID, protocolErr.Code, protocolErr.Message)
	}
}

func (c *connection) sendError(requestID, code, message string) {
	_ = c.send(EventError, requestID, struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}{Code: code, Message: message})
}
