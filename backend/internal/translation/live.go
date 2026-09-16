package translation

import (
	"context"
	"errors"
	"fmt"
)

// EventKind classifies a LiveEvent emitted by a streaming session.
type EventKind int

const (
	// EventInputTranscription carries the recognized text of the source audio.
	EventInputTranscription EventKind = iota
	// EventOutputTranscription carries the translated text of the output audio.
	EventOutputTranscription
	// EventTranslatedAudio carries a chunk of translated audio.
	EventTranslatedAudio
	// EventTurnComplete signals that the server finished the current turn.
	EventTurnComplete
	// EventInterrupted signals that the server interrupted generation.
	EventInterrupted
	// EventSessionError carries a terminal session failure.
	EventSessionError
)

// LiveEvent is a single item produced by a LiveSession.
type LiveEvent struct {
	Kind           EventKind
	SourceText     string
	SourceLanguage Language
	OutputText     string
	OutputLanguage Language
	Audio          AudioOutput
	Err            error
}

// OpenRequest configures a new streaming translation session.
type OpenRequest struct {
	// ID is a client-provided identifier (1 to 64 bytes).
	ID string
	// SourceLanguage is the language of the incoming audio. Empty means
	// automatic detection.
	SourceLanguage Language
	// TargetLanguage is the language the audio must be translated into.
	TargetLanguage Language
	// EchoTarget controls whether input already in the target language is
	// echoed or kept silent.
	EchoTarget bool
}

// LiveSession is a bidirectional, provider-backed streaming translation
// channel. Audio pushed through SendAudio is translated by the provider and
// surfaced, together with transcripts, through Events.
type LiveSession interface {
	// SendAudio queues one audio chunk (raw little-endian 16-bit PCM) for
	// translation. It blocks if the provider's backpressure is full.
	SendAudio(ctx context.Context, audio AudioInput) error
	// Events delivers input/output transcripts and translated audio chunks
	// until the session ends, at which point the channel is closed.
	Events() <-chan LiveEvent
	// Close ends the session gracefully and releases the underlying provider
	// connection.
	Close(ctx context.Context) error
}

// SessionOpener is implemented by providers that expose streaming sessions.
type SessionOpener interface {
	// OpenLiveSession starts a persistent streaming translation session and
	// returns it only after the provider handshake completed.
	OpenLiveSession(ctx context.Context, request OpenRequest) (LiveSession, error)
}

var (
	// ErrSessionClosed is returned when the session is already closed.
	ErrSessionClosed = errors.New("translation session is closed")
	// ErrStreamTimeout is returned when a streaming operation times out.
	ErrStreamTimeout = errors.New("translation stream timed out")
	// ErrSessionFailed is returned when the provider fails mid-session.
	ErrSessionFailed = errors.New("translation session failed")
)

// ValidateOpenRequest returns the canonical validation error for a session
// request, or nil.
func ValidateOpenRequest(request OpenRequest) error {
	switch {
	case request.ID == "" || len(request.ID) > 64:
		return fmt.Errorf("%w: id must contain 1 to 64 bytes", ErrInvalidRequest)
	case !request.TargetLanguage.Valid():
		return fmt.Errorf("%w: %q", ErrUnsupportedLanguage, request.TargetLanguage)
	case request.SourceLanguage != "" && !request.SourceLanguage.Valid():
		return fmt.Errorf("%w: %q", ErrUnsupportedLanguage, request.SourceLanguage)
	default:
		return nil
	}
}
