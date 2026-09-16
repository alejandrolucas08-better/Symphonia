package websocket

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/institucional/symphonia/backend/internal/call"
)

const (
	writeTimeout      = 5 * time.Second
	audioWriteTimeout = time.Second
)

const statusConnectionReplaced websocket.StatusCode = 4001

var errConnectionReplaced = errors.New("connection was replaced or removed")

type Hub struct {
	mu    sync.Mutex
	rooms map[string]*room
}

type room struct {
	connections map[int64]*connection
	muteStates  map[int64]bool
}

type connection struct {
	socket      *websocket.Conn
	code        string
	userID      int64
	participant call.Participant
	ready       bool
}

type muteState struct {
	UserID int64 `json:"user_id"`
	Muted  bool  `json:"muted"`
}

func NewHub() *Hub {
	return &Hub{rooms: make(map[string]*room)}
}

func (h *Hub) add(code string, participant call.Participant, socket *websocket.Conn, requestID string, currentCall *call.Call) (*connection, error) {
	client := &connection{socket: socket, code: code, userID: participant.UserID, participant: participant}
	h.mu.Lock()
	r := h.rooms[code]
	if r == nil {
		r = &room{connections: make(map[int64]*connection), muteStates: make(map[int64]bool)}
		h.rooms[code] = r
	}
	old := r.connections[participant.UserID]
	r.connections[participant.UserID] = client
	mutes := make([]muteState, 0, len(r.muteStates))
	for userID, muted := range r.muteStates {
		mutes = append(mutes, muteState{UserID: userID, Muted: muted})
	}
	h.mu.Unlock()

	if err := client.send(EventJoinCall, requestID, struct {
		Call       *call.Call  `json:"call"`
		MuteStates []muteState `json:"mute_states"`
	}{Call: currentCall, MuteStates: mutes}); err != nil {
		h.mu.Lock()
		if h.rooms[code] == r && r.connections[participant.UserID] == client {
			if old != nil {
				r.connections[participant.UserID] = old
			} else {
				delete(r.connections, participant.UserID)
				if len(r.connections) == 0 {
					delete(h.rooms, code)
				}
			}
		}
		h.mu.Unlock()
		return nil, err
	}

	h.mu.Lock()
	active := h.rooms[code] == r && r.connections[participant.UserID] == client
	if active {
		client.ready = true
	}
	h.mu.Unlock()
	if !active {
		client.socket.CloseNow()
		return nil, errConnectionReplaced
	}
	h.broadcast(code, participant.UserID, EventParticipantJoined, struct {
		Participant call.Participant `json:"participant"`
	}{Participant: participant})
	if old != nil {
		go func() {
			_ = old.socket.Close(statusConnectionReplaced, "connection replaced")
		}()
	}
	return client, nil
}

func (h *Hub) remove(client *connection, notify bool) {
	h.mu.Lock()
	r := h.rooms[client.code]
	if r == nil || r.connections[client.userID] != client {
		h.mu.Unlock()
		return
	}
	delete(r.connections, client.userID)
	delete(r.muteStates, client.userID)
	if len(r.connections) == 0 {
		delete(h.rooms, client.code)
	}
	h.mu.Unlock()
	if notify {
		h.broadcast(client.code, client.userID, EventParticipantLeft, struct {
			UserID int64  `json:"user_id"`
			Reason string `json:"reason"`
		}{UserID: client.userID, Reason: "disconnected"})
	}
}

func (h *Hub) message(client *connection, text string) {
	h.mu.Lock()
	r := h.rooms[client.code]
	if r == nil || r.connections[client.userID] != client {
		h.mu.Unlock()
		return
	}
	participant := client.participant
	h.mu.Unlock()
	h.broadcast(client.code, 0, EventMessage, struct {
		ID       string        `json:"id"`
		UserID   int64         `json:"user_id"`
		Name     string        `json:"name"`
		Language call.Language `json:"language"`
		Text     string        `json:"text"`
		SentAt   time.Time     `json:"sent_at"`
	}{ID: messageID(), UserID: client.userID, Name: participant.Name, Language: participant.SpokenLanguage, Text: text, SentAt: time.Now().UTC()})
}

func (h *Hub) mute(client *connection, muted bool) {
	h.mu.Lock()
	r := h.rooms[client.code]
	if r == nil || r.connections[client.userID] != client {
		h.mu.Unlock()
		return
	}
	r.muteStates[client.userID] = muted
	h.mu.Unlock()
	h.broadcast(client.code, 0, EventMuteState, muteState{UserID: client.userID, Muted: muted})
}

func (h *Hub) audio(sender *connection, payload []byte) {
	h.mu.Lock()
	r := h.rooms[sender.code]
	if r == nil || r.connections[sender.userID] != sender || !sender.ready || r.muteStates[sender.userID] {
		h.mu.Unlock()
		return
	}
	var peer *connection
	for userID, client := range r.connections {
		if userID != sender.userID && client.ready {
			peer = client
			break
		}
	}
	if peer == nil {
		h.mu.Unlock()
		return
	}
	h.mu.Unlock()

	if err := peer.write(websocket.MessageBinary, payload, audioWriteTimeout); err == nil {
		return
	}

	h.mu.Lock()
	r = h.rooms[sender.code]
	if r == nil {
		h.mu.Unlock()
		return
	}
	if r.connections[peer.userID] == peer {
		delete(r.connections, peer.userID)
		delete(r.muteStates, peer.userID)
	} else {
		h.mu.Unlock()
		return
	}
	h.mu.Unlock()
	peer.socket.CloseNow()
	h.broadcast(sender.code, peer.userID, EventParticipantLeft, struct {
		UserID int64  `json:"user_id"`
		Reason string `json:"reason"`
	}{UserID: peer.userID, Reason: "disconnected"})
}

func (h *Hub) broadcast(code string, exceptUserID int64, eventType string, data any) {
	h.mu.Lock()
	var clients []*connection
	if r := h.rooms[code]; r != nil {
		for userID, client := range r.connections {
			if client.ready && userID != exceptUserID {
				clients = append(clients, client)
			}
		}
	}
	h.mu.Unlock()
	for _, client := range clients {
		_ = client.send(eventType, "", data)
	}
}

func (h *Hub) ParticipantJoined(code string, participant call.Participant) {
	h.mu.Lock()
	if r := h.rooms[code]; r != nil {
		if client := r.connections[participant.UserID]; client != nil {
			client.participant = participant
		}
	}
	h.mu.Unlock()
	h.broadcast(code, participant.UserID, EventParticipantJoined, struct {
		Participant call.Participant `json:"participant"`
	}{Participant: participant})
}

func (h *Hub) LanguageChanged(code string, participant call.Participant) {
	h.mu.Lock()
	if r := h.rooms[code]; r != nil {
		if client := r.connections[participant.UserID]; client != nil {
			client.participant = participant
		}
	}
	h.mu.Unlock()
	h.broadcast(code, 0, EventLanguageChanged, struct {
		Participant call.Participant `json:"participant"`
	}{Participant: participant})
}

func (h *Hub) ParticipantLeft(code string, userID int64) {
	h.mu.Lock()
	var leaving *connection
	if r := h.rooms[code]; r != nil {
		leaving = r.connections[userID]
		delete(r.connections, userID)
		delete(r.muteStates, userID)
		if len(r.connections) == 0 {
			delete(h.rooms, code)
		}
	}
	h.mu.Unlock()
	h.broadcast(code, userID, EventParticipantLeft, struct {
		UserID int64  `json:"user_id"`
		Reason string `json:"reason"`
	}{UserID: userID, Reason: "left"})
	if leaving != nil {
		go func() {
			_ = leaving.socket.Close(websocket.StatusNormalClosure, "participant left")
		}()
	}
}

func (h *Hub) CallEnded(code string, endedByUserID int64) {
	h.mu.Lock()
	var clients []*connection
	if r := h.rooms[code]; r != nil {
		for _, client := range r.connections {
			clients = append(clients, client)
		}
		delete(h.rooms, code)
	}
	h.mu.Unlock()
	data := struct {
		EndedByUserID int64 `json:"ended_by_user_id"`
	}{EndedByUserID: endedByUserID}
	for _, client := range clients {
		_ = client.send(EventCallEnded, "", data)
		go func(client *connection) {
			_ = client.socket.Close(websocket.StatusNormalClosure, "call ended")
		}(client)
	}
}

func (c *connection) send(eventType, requestID string, data any) error {
	payload, err := json.Marshal(ServerEnvelope{
		Version: ProtocolVersion, Type: eventType, RequestID: requestID, Data: data, OccurredAt: time.Now().UTC(),
	})
	if err != nil {
		return err
	}
	return c.write(websocket.MessageText, payload, writeTimeout)
}

func (c *connection) write(messageType websocket.MessageType, payload []byte, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return c.socket.Write(ctx, messageType, payload)
}

func messageID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return time.Now().UTC().Format("20060102T150405.000000000")
	}
	return hex.EncodeToString(value[:])
}
