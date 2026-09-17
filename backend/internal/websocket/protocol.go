package websocket

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
)

const (
	ProtocolVersion = 1
	Subprotocol     = "symphonia.v1"
	MaxFrameBytes   = 16 * 1024

	AudioFrameSize         = 664
	AudioHeaderSize        = 24
	AudioPayloadSize       = 640
	AudioVersion           = 1
	AudioKindPCM           = 1
	AudioFlagDiscontinuity = 1 << 0
	AudioCodecPCM16LE      = 1
	AudioChannels          = 1
	AudioSampleRate        = 16000
	AudioSampleCount       = 320

	EventJoinCall               = "join_call"
	EventParticipantJoined      = "participant_joined"
	EventParticipantLeft        = "participant_left"
	EventMessage                = "message"
	EventMuteState              = "mute_state"
	EventLanguageChanged        = "language_changed"
	EventCallEnded              = "call_ended"
	EventError                  = "error"
	EventInputTranscription     = "input_transcription"
	EventOutputTranscription    = "output_transcription"
	EventTranslatedAudio        = "translated_audio"
	EventTranslationInterrupted = "translation_interrupted"
)

var audioMagic = [2]byte{'S', 'A'}

type ClientEnvelope struct {
	Version   int             `json:"version"`
	Type      string          `json:"type"`
	RequestID string          `json:"request_id"`
	Data      json.RawMessage `json:"data"`
}

type ServerEnvelope struct {
	Version    int       `json:"version"`
	Type       string    `json:"type"`
	RequestID  string    `json:"request_id,omitempty"`
	Data       any       `json:"data"`
	OccurredAt time.Time `json:"occurred_at"`
}

type ProtocolError struct {
	Code      string
	Message   string
	RequestID string
}

func (e *ProtocolError) Error() string { return e.Message }

func DecodeClient(payload []byte) (ClientEnvelope, error) {
	var envelope ClientEnvelope
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil {
		return envelope, protocolError("invalid_envelope", "invalid client envelope", "")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return envelope, protocolError("invalid_envelope", "invalid client envelope", envelope.RequestID)
	}
	if envelope.Version != ProtocolVersion {
		return envelope, protocolError("unsupported_version", "unsupported protocol version", envelope.RequestID)
	}
	if envelope.RequestID == "" || len(envelope.RequestID) > 64 {
		return envelope, protocolError("invalid_request_id", "request_id must contain 1 to 64 bytes", "")
	}
	if envelope.Type == "" || !isJSONObject(envelope.Data) {
		return envelope, protocolError("invalid_envelope", "type and object data are required", envelope.RequestID)
	}
	switch envelope.Type {
	case EventJoinCall, EventMessage, EventMuteState:
		return envelope, nil
	case EventParticipantJoined, EventParticipantLeft, EventLanguageChanged, EventCallEnded, EventError,
		EventInputTranscription, EventOutputTranscription, EventTranslatedAudio, EventTranslationInterrupted:
		return envelope, protocolError("server_only_event", "event type is server-only", envelope.RequestID)
	default:
		return envelope, protocolError("unknown_event", "unknown event type", envelope.RequestID)
	}
}

func ValidateAudioFrame(payload []byte) bool {
	return len(payload) == AudioFrameSize &&
		payload[0] == audioMagic[0] && payload[1] == audioMagic[1] &&
		payload[2] == AudioVersion &&
		payload[3] == AudioKindPCM &&
		payload[4]&^byte(AudioFlagDiscontinuity) == 0 &&
		payload[5] == AudioCodecPCM16LE &&
		payload[6] == AudioChannels &&
		payload[7] == AudioHeaderSize &&
		binary.BigEndian.Uint32(payload[8:12]) != 0 &&
		binary.BigEndian.Uint16(payload[20:22]) == AudioSampleRate &&
		binary.BigEndian.Uint16(payload[22:24]) == AudioSampleCount &&
		len(payload[AudioHeaderSize:]) == AudioPayloadSize
}

func decodeData(data json.RawMessage, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("extra JSON value")
	}
	return nil
}

func isJSONObject(data []byte) bool {
	trimmed := strings.TrimSpace(string(data))
	return len(trimmed) >= 2 && trimmed[0] == '{' && trimmed[len(trimmed)-1] == '}'
}

func protocolError(code, message, requestID string) *ProtocolError {
	return &ProtocolError{Code: code, Message: message, RequestID: requestID}
}
