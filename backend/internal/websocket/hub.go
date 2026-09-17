package websocket

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/institucional/symphonia/backend/internal/call"
	"github.com/institucional/symphonia/backend/internal/translation"
)

const (
	writeTimeout      = 5 * time.Second
	audioWriteTimeout = time.Second
)

const statusConnectionReplaced websocket.StatusCode = 4001

var errConnectionReplaced = errors.New("connection was replaced or removed")

type Hub struct {
	mu           sync.Mutex
	rooms        map[string]*room
	translations translation.SessionOpener
	closed       bool
}

type room struct {
	connections map[int64]*connection
	muteStates  map[int64]bool
	sessions    map[int64]*translationSession
}

type connection struct {
	socket                 *websocket.Conn
	code                   string
	userID                 int64
	participant            call.Participant
	ready                  bool
	ctx                    context.Context
	cancel                 context.CancelFunc
	nextTranslationAttempt time.Time // owned by this connection's audio worker
}

// translationSession wraps one streaming translation session owned by a room.
// It translates the voice of session.userID into session.target (the heard
// language of the receiving peer).
type translationSession struct {
	live      translation.LiveSession
	source    translation.Language
	target    translation.Language
	userID    int64
	code      string
	closeOnce sync.Once
	done      chan struct{}
	streamID  uint32
	audio     translatedPCM
}

func (s *translationSession) close() {
	s.closeOnce.Do(func() {
		if s.done != nil {
			close(s.done)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = s.live.Close(ctx)
	})
}

type muteState struct {
	UserID int64 `json:"user_id"`
	Muted  bool  `json:"muted"`
}

func NewHub() *Hub {
	return &Hub{rooms: make(map[string]*room)}
}

// setTranslations enables streaming translation for the hub. It must be called
// before any audio flows; with a nil service the hub only relays raw audio.
func (h *Hub) setTranslations(service translation.SessionOpener) {
	h.translations = service
}

func (h *Hub) add(code string, participant call.Participant, socket *websocket.Conn, requestID string, currentCall *call.Call) (*connection, error) {
	client := &connection{socket: socket, code: code, userID: participant.UserID, participant: participant}
	client.ctx, client.cancel = context.WithCancel(context.Background())
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		client.cancel()
		return nil, errConnectionReplaced
	}
	r := h.rooms[code]
	if r == nil {
		r = &room{
			connections: make(map[int64]*connection),
			muteStates:  make(map[int64]bool),
			sessions:    make(map[int64]*translationSession),
		}
		h.rooms[code] = r
	}
	old := r.connections[participant.UserID]
	var stale []*translationSession
	if old != nil {
		stale = roomSessions(r)
		clear(r.sessions)
	}
	r.connections[participant.UserID] = client
	client.ready = true
	mutes := make([]muteState, 0, len(r.muteStates))
	for userID, muted := range r.muteStates {
		mutes = append(mutes, muteState{UserID: userID, Muted: muted})
	}
	// Snapshot the peers that must be told about this participant at the moment
	// of registration, so joins are consistent even if another client joins
	// while the join_call reply is being written.
	var peers []*connection
	for userID, other := range r.connections {
		if userID != participant.UserID && other.ready {
			peers = append(peers, other)
		}
	}
	h.mu.Unlock()

	for _, session := range stale {
		session.close()
	}
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
		client.cancel()
		return nil, err
	}

	for _, peer := range peers {
		_ = peer.send(EventParticipantJoined, "", struct {
			Participant call.Participant `json:"participant"`
		}{Participant: participant})
	}

	h.mu.Lock()
	active := h.rooms[code] == r && r.connections[participant.UserID] == client
	h.mu.Unlock()
	if !active {
		client.cancel()
		_ = client.socket.Close(statusConnectionReplaced, "connection replaced")
		return nil, errConnectionReplaced
	}
	if old != nil {
		old.cancel()
		go func() {
			_ = old.socket.Close(statusConnectionReplaced, "connection replaced")
		}()
	}
	return client, nil
}

func (h *Hub) remove(client *connection, notify bool) {
	if client.cancel != nil {
		client.cancel()
	}
	h.mu.Lock()
	r := h.rooms[client.code]
	if r == nil || r.connections[client.userID] != client {
		h.mu.Unlock()
		return
	}
	delete(r.connections, client.userID)
	delete(r.muteStates, client.userID)
	sessions := roomSessions(r)
	clear(r.sessions)
	if len(r.connections) == 0 {
		delete(h.rooms, client.code)
	}
	h.mu.Unlock()
	for _, session := range sessions {
		session.close()
	}
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
	var session *translationSession
	if muted {
		session = r.sessions[client.userID]
		delete(r.sessions, client.userID)
	}
	h.mu.Unlock()
	if session != nil {
		session.close()
	}
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
	speaker := sender.participant
	var listener call.Participant
	if peer != nil {
		listener = peer.participant
	}
	h.mu.Unlock()
	if peer == nil {
		return
	}

	relay := func() {
		if err := peer.write(websocket.MessageBinary, payload, audioWriteTimeout); err != nil {
			h.dropPeer(sender.code, peer)
		}
	}

	if h.translations == nil || !translationNeeded(speaker, listener) {
		relay()
		return
	}

	target := translation.Language(listener.HeardLanguage)
	session := h.sessionFor(sender.code, sender.userID, target)
	if session == nil {
		if time.Now().Before(sender.nextTranslationAttempt) {
			return
		}
		opened, err := h.openSession(sender, peer, speaker, listener, target)
		if err != nil {
			sender.nextTranslationAttempt = time.Now().Add(5 * time.Second)
			log.Printf("websocket: open translation session failed (user %d)", sender.userID)
			sender.sendError("", "translation_unavailable", "Translation unavailable; check backend configuration and reconnect.")
			return
		}
		session = opened
	}
	parent := sender.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, writeTimeout)
	defer cancel()
	if err := session.live.SendAudio(ctx, frameToAudioInput(payload)); err != nil {
		log.Printf("websocket: translation audio send failed (user %d)", sender.userID)
		h.dropSession(session)
		sender.sendError("", "translation_unavailable", "Translation interrupted; reconnect to retry.")
		return
	}
}

// dropPeer removes a slow or failed peer and notifies the remaining client.
func (h *Hub) dropPeer(code string, peer *connection) {
	h.mu.Lock()
	r := h.rooms[code]
	if r == nil {
		h.mu.Unlock()
		return
	}
	if r.connections[peer.userID] != peer {
		h.mu.Unlock()
		return
	}
	delete(r.connections, peer.userID)
	delete(r.muteStates, peer.userID)
	sessions := roomSessions(r)
	clear(r.sessions)
	h.mu.Unlock()
	for _, session := range sessions {
		session.close()
	}
	peer.socket.CloseNow()
	h.broadcast(code, peer.userID, EventParticipantLeft, struct {
		UserID int64  `json:"user_id"`
		Reason string `json:"reason"`
	}{UserID: peer.userID, Reason: "disconnected"})
}

// translationNeeded reports whether the speaker's voice must be translated to
// reach the peer, i.e. the peer does not hear the speaker's own language.
func translationNeeded(speaker, peer call.Participant) bool {
	return speaker.SpokenLanguage != peer.HeardLanguage
}

// sessionFor returns the live session translating the given user's speech into
// the requested target language, or nil.
func (h *Hub) sessionFor(code string, userID int64, target translation.Language) *translationSession {
	h.mu.Lock()
	defer h.mu.Unlock()
	r := h.rooms[code]
	if r == nil {
		return nil
	}
	session := r.sessions[userID]
	if session != nil && session.target == target {
		return session
	}
	return nil
}

// openSession opens a streaming session for a speaker, stores it on the room,
// and starts dispatching its output to the call.
func (h *Hub) openSession(sender, peer *connection, speaker, listener call.Participant, target translation.Language) (*translationSession, error) {
	code := sender.code
	ctx := sender.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	session, err := h.translations.OpenLiveSession(ctx, translation.OpenRequest{
		ID:             fmt.Sprintf("%s:%d", code, speaker.UserID),
		SourceLanguage: translation.Language(speaker.SpokenLanguage),
		TargetLanguage: target,
		EchoTarget:     true,
	})
	if err != nil {
		return nil, err
	}
	active := &translationSession{live: session, source: translation.Language(speaker.SpokenLanguage), target: target, userID: speaker.UserID, code: code}
	active.done = make(chan struct{})
	var id [4]byte
	_, _ = rand.Read(id[:])
	active.streamID = binary.BigEndian.Uint32(id[:]) | 1

	h.mu.Lock()
	r := h.rooms[code]
	if r == nil || r.connections[speaker.UserID] != sender || r.connections[listener.UserID] != peer ||
		r.muteStates[speaker.UserID] ||
		sender.participant.SpokenLanguage != speaker.SpokenLanguage || peer.participant.HeardLanguage != listener.HeardLanguage {
		h.mu.Unlock()
		active.close()
		return nil, errConnectionReplaced
	}
	if stale := r.sessions[speaker.UserID]; stale != nil && stale != active {
		go stale.close()
	}
	r.sessions[speaker.UserID] = active
	h.mu.Unlock()

	go h.pumpSession(active)
	return active, nil
}

// pumpSession forwards a session's output until the session ends, then removes
// it from the room.
func (h *Hub) pumpSession(session *translationSession) {
	defer session.close()
	for event := range session.live.Events() {
		h.dispatch(session, event)
	}
	h.mu.Lock()
	if r := h.rooms[session.code]; r != nil && r.sessions[session.userID] == session {
		delete(r.sessions, session.userID)
	}
	h.mu.Unlock()
}

func (h *Hub) dispatch(session *translationSession, event translation.LiveEvent) {
	h.mu.Lock()
	r := h.rooms[session.code]
	active := r != nil && r.sessions[session.userID] == session
	h.mu.Unlock()
	if !active {
		return
	}
	switch event.Kind {
	case translation.EventInputTranscription:
		h.broadcast(session.code, 0, EventInputTranscription, transcriptionEvent{
			UserID:   session.userID,
			Text:     event.SourceText,
			Language: event.SourceLanguage,
		})
	case translation.EventOutputTranscription:
		h.broadcast(session.code, 0, EventOutputTranscription, transcriptionEvent{
			UserID:   session.userID,
			Text:     event.OutputText,
			Language: event.OutputLanguage,
		})
	case translation.EventTranslatedAudio:
		h.deliverTranslatedAudio(session, event)
	case translation.EventInterrupted:
		session.audio = translatedPCM{}
		h.deliverTranslationInterrupted(session)
	case translation.EventSessionError:
		log.Printf("websocket: translation session failed (%s/%d)", session.code, session.userID)
		h.broadcast(session.code, 0, EventError, map[string]string{"code": "translation_unavailable", "message": "Translation interrupted; retrying on subsequent audio."})
		h.dropSession(session)
	case translation.EventTurnComplete:
		h.sendTranslatedFrames(session, true)
	}
}

func (h *Hub) deliverTranslationInterrupted(session *translationSession) {
	h.mu.Lock()
	r := h.rooms[session.code]
	var peer *connection
	if r != nil && r.sessions[session.userID] == session {
		for userID, client := range r.connections {
			if userID != session.userID && client.ready {
				peer = client
				break
			}
		}
	}
	h.mu.Unlock()
	if peer != nil {
		_ = peer.send(EventTranslationInterrupted, "", struct {
			UserID int64 `json:"user_id"`
		}{UserID: session.userID})
	}
}

// deliverTranslatedAudio sends a translated audio chunk to the peer listening
// to this session's speaker.
func (h *Hub) deliverTranslatedAudio(session *translationSession, event translation.LiveEvent) {
	if err := session.audio.append(event.Audio); err != nil {
		h.dropSession(session)
		return
	}
	h.sendTranslatedFrames(session, false)
}

// dropSession removes and closes a translation session.
func (h *Hub) dropSession(session *translationSession) {
	h.mu.Lock()
	if r := h.rooms[session.code]; r != nil && r.sessions[session.userID] == session {
		delete(r.sessions, session.userID)
	}
	h.mu.Unlock()
	session.close()
}

// roomSessions returns a copy of a room's sessions. Callers must hold h.mu.
func roomSessions(r *room) []*translationSession {
	sessions := make([]*translationSession, 0, len(r.sessions))
	for _, session := range r.sessions {
		sessions = append(sessions, session)
	}
	return sessions
}

// transcriptionEvent is broadcast for source transcripts and translated text.
type transcriptionEvent struct {
	UserID   int64                `json:"user_id"`
	Text     string               `json:"text"`
	Language translation.Language `json:"language"`
}

// frameToAudioInput extracts a translation audio chunk from a valid protocol
// audio frame.
func frameToAudioInput(payload []byte) translation.AudioInput {
	return translation.AudioInput{
		Format:    translation.DefaultAudioFormat,
		Data:      append([]byte(nil), payload[AudioHeaderSize:]...),
		StreamID:  binary.BigEndian.Uint32(payload[8:12]),
		Sequence:  binary.BigEndian.Uint32(payload[12:16]),
		Timestamp: binary.BigEndian.Uint32(payload[16:20]),
	}
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
	var stale []*translationSession
	if r := h.rooms[code]; r != nil {
		if client := r.connections[participant.UserID]; client != nil {
			client.participant = participant
		}
		// Sessions translate each speaker into the changing participant's
		// heard language; when that target changes, close and reopen lazily.
		heard := translation.Language(participant.HeardLanguage)
		for userID, session := range r.sessions {
			if (userID != participant.UserID && session.target != heard) ||
				(userID == participant.UserID && session.source != translation.Language(participant.SpokenLanguage)) {
				delete(r.sessions, userID)
				stale = append(stale, session)
			}
		}
	}
	h.mu.Unlock()
	for _, session := range stale {
		session.close()
	}
	h.broadcast(code, 0, EventLanguageChanged, struct {
		Participant call.Participant `json:"participant"`
	}{Participant: participant})
}

func (h *Hub) ParticipantLeft(code string, userID int64) {
	h.mu.Lock()
	var leaving *connection
	var sessions []*translationSession
	if r := h.rooms[code]; r != nil {
		leaving = r.connections[userID]
		delete(r.connections, userID)
		delete(r.muteStates, userID)
		sessions = roomSessions(r)
		clear(r.sessions)
		if len(r.connections) == 0 {
			delete(h.rooms, code)
		}
	}
	h.mu.Unlock()
	for _, session := range sessions {
		session.close()
	}
	h.broadcast(code, userID, EventParticipantLeft, struct {
		UserID int64  `json:"user_id"`
		Reason string `json:"reason"`
	}{UserID: userID, Reason: "left"})
	if leaving != nil {
		if leaving.cancel != nil {
			leaving.cancel()
		}
		go func() {
			_ = leaving.socket.Close(websocket.StatusNormalClosure, "participant left")
		}()
	}
}

func (h *Hub) CallEnded(code string, endedByUserID int64) {
	h.mu.Lock()
	var clients []*connection
	var sessions []*translationSession
	if r := h.rooms[code]; r != nil {
		for _, client := range r.connections {
			clients = append(clients, client)
		}
		sessions = roomSessions(r)
		delete(h.rooms, code)
	}
	h.mu.Unlock()
	for _, session := range sessions {
		session.close()
	}
	data := struct {
		EndedByUserID int64 `json:"ended_by_user_id"`
	}{EndedByUserID: endedByUserID}
	for _, client := range clients {
		if client.cancel != nil {
			client.cancel()
		}
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

// Close releases hijacked WebSockets and provider sessions during shutdown.
// net/http.Server.Shutdown alone does not close hijacked connections.
func (h *Hub) Close() {
	h.mu.Lock()
	h.closed = true
	var clients []*connection
	var sessions []*translationSession
	for _, r := range h.rooms {
		for _, client := range r.connections {
			clients = append(clients, client)
		}
		sessions = append(sessions, roomSessions(r)...)
	}
	clear(h.rooms)
	h.mu.Unlock()
	for _, client := range clients {
		if client.cancel != nil {
			client.cancel()
		}
		_ = client.socket.CloseNow()
	}
	var wg sync.WaitGroup
	for _, session := range sessions {
		wg.Add(1)
		go func() { defer wg.Done(); session.close() }()
	}
	wg.Wait()
}
