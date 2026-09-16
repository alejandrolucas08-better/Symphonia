package translation

import (
	"context"
	"math"
	"sync"
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

// OpenLiveSession implements SessionOpener with an in-memory session, so the
// streaming WebSocket pipeline can be exercised without any external provider.
func (s *MockService) OpenLiveSession(_ context.Context, request OpenRequest) (LiveSession, error) {
	if err := ValidateOpenRequest(request); err != nil {
		return nil, err
	}
	session := &mockLiveSession{
		svc:     s,
		request: request,
		in:      make(chan AudioInput, 64),
		out:     make(chan LiveEvent, 64),
		done:    make(chan struct{}),
	}
	go session.run()
	return session, nil
}

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

// mockLiveSession is the mock's streaming Session implementation. It
// deterministically processes each audio chunk and emits source transcription,
// translated transcript, a short translated audio tone, and a turn-complete
// signal, mirroring the Gemini model's output cadence.
type mockLiveSession struct {
	svc     *MockService
	request OpenRequest
	in      chan AudioInput
	out     chan LiveEvent
	done    chan struct{}
	once    sync.Once
}

func (m *mockLiveSession) SendAudio(ctx context.Context, audio AudioInput) error {
	select {
	case <-m.done:
		return ErrSessionClosed
	default:
	}
	if len(audio.Data) == 0 {
		return ErrEmptyAudio
	}
	select {
	case <-m.done:
		return ErrSessionClosed
	case <-ctx.Done():
		return ctx.Err()
	case m.in <- audio:
		return nil
	}
}

func (m *mockLiveSession) Events() <-chan LiveEvent { return m.out }

func (m *mockLiveSession) Close(ctx context.Context) error {
	m.once.Do(func() { close(m.done) })
	return nil
}

func (m *mockLiveSession) run() {
	defer close(m.out)
	for {
		select {
		case <-m.done:
			return
		case audio, ok := <-m.in:
			if !ok {
				return
			}
			m.process(audio)
		}
	}
}

func (m *mockLiveSession) process(audio AudioInput) {
	sourceText := m.svc.phrases[m.request.SourceLanguage]
	outputText := m.svc.phrases[m.request.TargetLanguage]

	dataLen := len(audio.Data)
	if dataLen == 0 {
		dataLen = 640
	}
	tone := mockTone(GeminiOutputFormat, toneSamples(dataLen))

	m.emit(LiveEvent{
		Kind:           EventInputTranscription,
		SourceText:     sourceText,
		SourceLanguage: m.request.SourceLanguage,
	})
	m.emit(LiveEvent{
		Kind:           EventOutputTranscription,
		OutputText:     outputText,
		OutputLanguage: m.request.TargetLanguage,
	})
	m.emit(LiveEvent{
		Kind:           EventTranslatedAudio,
		OutputLanguage: m.request.TargetLanguage,
		Audio:          AudioOutput{Format: GeminiOutputFormat, Data: tone},
	})
	m.emit(LiveEvent{Kind: EventTurnComplete})
}

func (m *mockLiveSession) emit(event LiveEvent) {
	select {
	case <-m.done:
	case m.out <- event:
	}
}

// toneSamples derives a deterministic, non-zero sample count from the input
// byte count so the test can assert that audio is actually present.
func toneSamples(inputBytes int) int {
	const minSamples = 640
	samples := inputBytes / 2 // two bytes per sample
	if samples < minSamples {
		return minSamples
	}
	return samples
}
