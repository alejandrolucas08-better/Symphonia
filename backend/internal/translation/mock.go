package translation

import (
	"context"
	"math"
	"time"
)

// MockService implements Service deterministically so the translation flow can
// be validated end to end without any external provider.
type MockService struct {
	phrases map[Language]string
}

const mockConfidence = 0.92

// NewMockService returns a MockService with a small built-in phrase set.
func NewMockService() *MockService {
	return &MockService{phrases: map[Language]string{
		LanguagePortuguese: "Olá, seja bem-vindo ao Symphonia.",
		LanguageEnglish:    "Hello, welcome to Symphonia.",
		LanguageSpanish:    "Hola, bienvenido a Symphonia.",
		LanguageFrench:     "Bonjour, bienvenue sur Symphonia.",
	}}
}

// Name identifies the mock provider.
func (s *MockService) Name() string { return string(ProviderMock) }

// Translate returns the original transcription and a deterministic translation
// for the given audio input.
func (s *MockService) Translate(_ context.Context, request Request) (*Result, error) {
	if err := ValidateRequest(request); err != nil {
		return nil, err
	}
	return &Result{
		ID:             request.ID,
		SourceLanguage: request.SourceLanguage,
		TargetLanguage: request.TargetLanguage,
		Transcription: Transcription{
			Text:       s.phrases[request.SourceLanguage],
			Language:   request.SourceLanguage,
			Confidence: mockConfidence,
		},
		TranslatedText: s.phrases[request.TargetLanguage],
		TranslatedAudio: AudioOutput{
			Format: request.Audio.Format,
			Data:   mockTone(request.Audio.Format, durationSamples(request.Audio)),
		},
		CreatedAt: time.Now().UTC(),
	}, nil
}

// durationSamples computes the number of samples carried by the audio buffer.
func durationSamples(audio AudioInput) int {
	frame := audio.Format.Channels * audio.Format.BytesPerSample
	if frame == 0 {
		return 0
	}
	return len(audio.Data) / frame
}

// mockTone renders a soft 440 Hz PCM tone so the translated audio is a valid,
// non-empty buffer in the same format as the input.
func mockTone(format AudioFormat, samples int) []byte {
	if samples == 0 || format.SampleRate == 0 {
		return []byte{}
	}
	data := make([]byte, samples*format.Channels*format.BytesPerSample)
	for i := 0; i < samples; i++ {
		value := int16(0.12 * 32767 * math.Sin(2*math.Pi*440*float64(i)/float64(format.SampleRate)))
		offset := i * format.Channels * format.BytesPerSample
		for channel := 0; channel < format.Channels; channel++ {
			sampleOffset := offset + channel*format.BytesPerSample
			data[sampleOffset] = byte(value)
			data[sampleOffset+1] = byte(value >> 8)
		}
	}
	return data
}