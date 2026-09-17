package websocket

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/institucional/symphonia/backend/internal/translation"
)

func TestResamplingPreservesDurationAndChunkBoundaries(t *testing.T) {
	pcm := make([]byte, 48000)
	for i := 0; i < len(pcm); i += 2 {
		binary.LittleEndian.PutUint16(pcm[i:], uint16(i%20000))
	}
	format := translation.AudioFormat{Codec: translation.AudioCodecPCM16LE, SampleRate: 24000, Channels: 1, BytesPerSample: 2}
	var whole, split translatedPCM
	if err := whole.append(translation.AudioOutput{Format: format, Data: pcm}); err != nil {
		t.Fatal(err)
	}
	for start := 0; start < len(pcm); start += 137 {
		end := min(start+137, len(pcm))
		if err := split.append(translation.AudioOutput{Format: format, Data: pcm[start:end]}); err != nil {
			t.Fatal(err)
		}
	}
	if len(whole.pcm) != 32000 || !bytes.Equal(whole.pcm, split.pcm) {
		t.Fatal("resampling changed duration or depends on provider chunk boundaries")
	}
}
