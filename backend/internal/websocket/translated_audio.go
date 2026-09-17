package websocket

import (
	"encoding/binary"
	"errors"
	"time"

	"github.com/coder/websocket"
	"github.com/institucional/symphonia/backend/internal/translation"
)

// translatedPCM preserves partial samples and frame boundaries between provider
// chunks. Area averaging maps every three 24 kHz samples to two 16 kHz samples.
// This is a low-cost speech resampler, not a studio-quality anti-aliasing filter.
type translatedPCM struct {
	pending  []byte
	pcm      []byte
	sequence uint32
	next     time.Time
}

func (p *translatedPCM) append(audio translation.AudioOutput) error {
	f := audio.Format
	if f.Codec != translation.AudioCodecPCM16LE || f.Channels != 1 || f.BytesPerSample != 2 || (f.SampleRate != 24000 && f.SampleRate != 16000) {
		return errors.New("unsupported translated audio format")
	}
	if f.SampleRate == 16000 {
		p.pcm = append(p.pcm, audio.Data...)
		return nil
	}
	p.pending = append(p.pending, audio.Data...)
	for len(p.pending) >= 6 {
		a := int32(int16(binary.LittleEndian.Uint16(p.pending)))
		b := int32(int16(binary.LittleEndian.Uint16(p.pending[2:])))
		c := int32(int16(binary.LittleEndian.Uint16(p.pending[4:])))
		p.pcm = binary.LittleEndian.AppendUint16(p.pcm, uint16(int16((2*a+b)/3)))
		p.pcm = binary.LittleEndian.AppendUint16(p.pcm, uint16(int16((b+2*c)/3)))
		p.pending = p.pending[6:]
	}
	return nil
}

func (h *Hub) sendTranslatedFrames(session *translationSession, flush bool) {
	p := &session.audio
	if flush && len(p.pcm) > 0 && len(p.pcm)%AudioPayloadSize != 0 {
		p.pcm = append(p.pcm, make([]byte, AudioPayloadSize-len(p.pcm)%AudioPayloadSize)...)
	}
	for len(p.pcm) >= AudioPayloadSize {
		if delay := time.Until(p.next); delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-session.done:
				timer.Stop()
				return
			case <-timer.C:
			}
		}
		h.mu.Lock()
		r := h.rooms[session.code]
		var peer *connection
		if r != nil && r.sessions[session.userID] == session {
			for id, client := range r.connections {
				if id != session.userID && client.ready {
					peer = client
					break
				}
			}
		}
		h.mu.Unlock()
		if peer == nil {
			return
		}
		frame := make([]byte, AudioFrameSize)
		copy(frame, []byte{'S', 'A', AudioVersion, AudioKindPCM, 0, AudioCodecPCM16LE, AudioChannels, AudioHeaderSize})
		if p.sequence == 0 {
			frame[4] = AudioFlagDiscontinuity
		}
		binary.BigEndian.PutUint32(frame[8:], session.streamID)
		binary.BigEndian.PutUint32(frame[12:], p.sequence)
		binary.BigEndian.PutUint32(frame[16:], p.sequence*AudioSampleCount)
		binary.BigEndian.PutUint16(frame[20:], AudioSampleRate)
		binary.BigEndian.PutUint16(frame[22:], AudioSampleCount)
		copy(frame[AudioHeaderSize:], p.pcm[:AudioPayloadSize])
		p.pcm = p.pcm[AudioPayloadSize:]
		p.sequence++
		if err := peer.write(websocket.MessageBinary, frame, audioWriteTimeout); err != nil {
			h.dropPeer(session.code, peer)
			return
		}
		p.next = time.Now().Add(20 * time.Millisecond)
	}
}
