package websocket

import (
	"encoding/binary"
	"errors"
	"strings"
	"testing"
)

func TestValidateAudioFrame(t *testing.T) {
	valid := testAudioFrame(7, 11, 13)
	if !ValidateAudioFrame(valid) {
		t.Fatal("valid audio frame was rejected")
	}
	valid[4] = AudioFlagDiscontinuity
	if !ValidateAudioFrame(valid) {
		t.Fatal("discontinuity flag was rejected")
	}
	tests := []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"size", func(frame []byte) []byte { return frame[:len(frame)-1] }},
		{"magic", func(frame []byte) []byte { frame[0] = 'X'; return frame }},
		{"version", func(frame []byte) []byte { frame[2] = 2; return frame }},
		{"kind", func(frame []byte) []byte { frame[3] = 2; return frame }},
		{"flags", func(frame []byte) []byte { frame[4] = 2; return frame }},
		{"codec", func(frame []byte) []byte { frame[5] = 2; return frame }},
		{"channels", func(frame []byte) []byte { frame[6] = 2; return frame }},
		{"header size", func(frame []byte) []byte { frame[7] = 23; return frame }},
		{"stream ID", func(frame []byte) []byte { binary.BigEndian.PutUint32(frame[8:12], 0); return frame }},
		{"sample rate", func(frame []byte) []byte { binary.BigEndian.PutUint16(frame[20:22], 8000); return frame }},
		{"sample count", func(frame []byte) []byte { binary.BigEndian.PutUint16(frame[22:24], 160); return frame }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			frame := append([]byte(nil), valid...)
			if ValidateAudioFrame(test.mutate(frame)) {
				t.Fatal("malformed audio frame was accepted")
			}
		})
	}
}

func testAudioFrame(streamID, sequence, timestamp uint32) []byte {
	frame := make([]byte, AudioFrameSize)
	copy(frame[0:2], audioMagic[:])
	frame[2] = AudioVersion
	frame[3] = AudioKindPCM
	frame[5] = AudioCodecPCM16LE
	frame[6] = AudioChannels
	frame[7] = AudioHeaderSize
	binary.BigEndian.PutUint32(frame[8:12], streamID)
	binary.BigEndian.PutUint32(frame[12:16], sequence)
	binary.BigEndian.PutUint32(frame[16:20], timestamp)
	binary.BigEndian.PutUint16(frame[20:22], AudioSampleRate)
	binary.BigEndian.PutUint16(frame[22:24], AudioSampleCount)
	for i := AudioHeaderSize; i < len(frame); i++ {
		frame[i] = byte(i)
	}
	return frame
}

func TestDecodeClientValidation(t *testing.T) {
	tests := []struct {
		name string
		body string
		code string
	}{
		{"valid", `{"version":1,"type":"join_call","request_id":"join-1","data":{}}`, ""},
		{"malformed", `{`, "invalid_envelope"},
		{"version", `{"version":2,"type":"join_call","request_id":"join-1","data":{}}`, "unsupported_version"},
		{"missing request ID", `{"version":1,"type":"join_call","request_id":"","data":{}}`, "invalid_request_id"},
		{"long request ID", `{"version":1,"type":"join_call","request_id":"` + strings.Repeat("a", 65) + `","data":{}}`, "invalid_request_id"},
		{"non-object data", `{"version":1,"type":"message","request_id":"m1","data":null}`, "invalid_envelope"},
		{"unknown", `{"version":1,"type":"other","request_id":"m1","data":{}}`, "unknown_event"},
		{"server only", `{"version":1,"type":"call_ended","request_id":"m1","data":{}}`, "server_only_event"},
		{"unknown envelope field", `{"version":1,"type":"join_call","request_id":"join-1","data":{},"extra":true}`, "invalid_envelope"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := DecodeClient([]byte(test.body))
			if test.code == "" {
				if err != nil {
					t.Fatalf("DecodeClient returned error: %v", err)
				}
				return
			}
			var protocolErr *ProtocolError
			if !errors.As(err, &protocolErr) || protocolErr.Code != test.code {
				t.Fatalf("error = %#v, want protocol code %q", err, test.code)
			}
		})
	}
}

func TestAuthProtocol(t *testing.T) {
	token, ok := authProtocol([]string{"symphonia.v1, auth.header.payload"})
	if !ok || token != "header.payload" {
		t.Fatalf("authProtocol = %q, %v", token, ok)
	}
	for _, protocols := range [][]string{{"symphonia.v1"}, {"auth.token"}, {"symphonia.v1, auth.one, auth.two"}} {
		if _, ok := authProtocol(protocols); ok {
			t.Fatalf("authProtocol accepted %v", protocols)
		}
	}
}
