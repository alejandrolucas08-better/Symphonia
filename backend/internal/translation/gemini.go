package translation

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	coderws "github.com/coder/websocket"
)

// Model and endpoint constants follow the official Gemini Live API documenta-
// tion for gemini-3.5-live-translate-preview:
//
//	https://ai.google.dev/gemini-api/docs/live-api/live-translate
const (
	// DefaultTranslationModel is the preview speech-to-speech translation model.
	DefaultTranslationModel = "gemini-3.5-live-translate-preview"
	// DefaultGeminiLiveEndpoint is the BidiGenerateContent WebSocket endpoint
	// used by the Live API (v1beta).
	DefaultGeminiLiveEndpoint = "wss://generativelanguage.googleapis.com/ws/google.ai.generativelanguage.v1beta.GenerativeService.BidiGenerateContent"

	// GeminiInputSampleRate is the raw PCM sample rate the model expects.
	GeminiInputSampleRate = 16000
	// GeminiOutputSampleRate is the raw PCM sample rate of the translated audio.
	GeminiOutputSampleRate = 24000

	geminiAudioMimeRate    = "audio/pcm;rate=16000"
	geminiOutputAudioMime  = "audio/pcm;rate=24000"
	geminiChunkBytes       = 3200 // 100 ms of 16 kHz mono 16-bit PCM
	geminiMessageReadLimit = 1 << 20
	geminiSetupTimeout     = 10 * time.Second
	geminiMaxSessionWait   = 30 * time.Second
)

// GeminiOutputFormat is the raw PCM format of the translated audio.
var GeminiOutputFormat = AudioFormat{
	Codec:          AudioCodecPCM16LE,
	SampleRate:     GeminiOutputSampleRate,
	Channels:       1,
	BytesPerSample: 2,
}

// Errors specific to the Gemini provider. They wrap the shared sentinel
// errors so callers can classify failures without depending on the provider.
var (
	ErrGeminiNotConfigured = errors.New("gemini translation is not configured")
	ErrGeminiSetup         = errors.New("gemini session setup failed")
	ErrGeminiStream        = errors.New("gemini translation stream failed")
)

// GeminiTranslationService implements Service and SessionOpener against the
// Gemini Live Translate model. It is fully isolated behind the translation
// package API; the rest of the application never imports provider internals.
type GeminiTranslationService struct {
	apiKey   string
	model    string
	endpoint string
}

// NewGeminiService validates the configuration and returns a Gemini-backed
// translation service.
func NewGeminiService(config Config) (*GeminiTranslationService, error) {
	if config.Provider != ProviderGemini {
		return nil, fmt.Errorf("%w: provider %q", ErrUnknownProvider, config.Provider)
	}
	if config.GeminiAPIKey == "" {
		return nil, fmt.Errorf("%w: GEMINI_API_KEY is required", ErrGeminiNotConfigured)
	}
	service := &GeminiTranslationService{
		apiKey:   config.GeminiAPIKey,
		model:    DefaultTranslationModel,
		endpoint: DefaultGeminiLiveEndpoint,
	}
	if config.TranslationModel != "" {
		service.model = config.TranslationModel
	}
	if config.GeminiLiveEndpoint != "" {
		service.endpoint = config.GeminiLiveEndpoint
	}
	return service, nil
}

// Name identifies the Gemini provider.
func (g *GeminiTranslationService) Name() string { return string(ProviderGemini) }

// Translate runs a one-shot translation: it opens a session, streams the whole
// audio input, waits for the translated turn, and closes the session.
func (g *GeminiTranslationService) Translate(ctx context.Context, request Request) (*Result, error) {
	if err := ValidateRequest(request); err != nil {
		return nil, err
	}
	session, err := g.OpenLiveSession(ctx, OpenRequest{
		ID:             request.ID,
		SourceLanguage: request.SourceLanguage,
		TargetLanguage: request.TargetLanguage,
		EchoTarget:     true,
	})
	if err != nil {
		return nil, err
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = session.Close(closeCtx)
	}()

	sendCtx, cancel := context.WithTimeout(ctx, geminiMaxSessionWait)
	defer cancel()
	for start := 0; start < len(request.Audio.Data); start += geminiChunkBytes {
		end := start + geminiChunkBytes
		if end > len(request.Audio.Data) {
			end = len(request.Audio.Data)
		}
		if err := session.SendAudio(sendCtx, AudioInput{
			Format: request.Audio.Format,
			Data:   request.Audio.Data[start:end],
		}); err != nil {
			return nil, err
		}
	}
	if live, ok := session.(*geminiLiveSession); ok {
		if err := live.signalEnd(sendCtx); err != nil {
			return nil, fmt.Errorf("%w: signal end of stream: %v", ErrGeminiStream, err)
		}
	}

	result := &Result{
		ID:             request.ID,
		SourceLanguage: request.SourceLanguage,
		TargetLanguage: request.TargetLanguage,
		CreatedAt:      time.Now().UTC(),
		Transcription: Transcription{
			Language: request.SourceLanguage,
		},
	}
	streamCtx, stop := context.WithTimeout(ctx, geminiMaxSessionWait)
	defer stop()
	for {
		select {
		case <-streamCtx.Done():
			return nil, fmt.Errorf("%w: waiting for translation: %v", ErrStreamTimeout, streamCtx.Err())
		case event, ok := <-session.Events():
			if !ok {
				return nil, fmt.Errorf("%w: session ended before translation completed", ErrSessionClosed)
			}
			switch event.Kind {
			case EventInputTranscription:
				result.Transcription.Text += event.SourceText
			case EventOutputTranscription:
				result.TranslatedText += event.OutputText
			case EventTranslatedAudio:
				result.TranslatedAudio.Data = append(result.TranslatedAudio.Data, event.Audio.Data...)
				result.TranslatedAudio.Format = event.Audio.Format
			case EventSessionError:
				return nil, event.Err
			case EventTurnComplete:
				if len(result.TranslatedAudio.Data) > 0 {
					return result, nil
				}
			}
		}
	}
}

// OpenLiveSession establishes a Gemini live session and returns it once the
// provider acknowledged the setup message with setupComplete.
func (g *GeminiTranslationService) OpenLiveSession(ctx context.Context, request OpenRequest) (LiveSession, error) {
	if err := ValidateOpenRequest(request); err != nil {
		return nil, err
	}
	if g.apiKey == "" {
		return nil, fmt.Errorf("%w: GEMINI_API_KEY is required", ErrGeminiNotConfigured)
	}
	target, err := geminiLanguageCode(request.TargetLanguage)
	if err != nil {
		return nil, err
	}

	dialCtx, cancel := context.WithTimeout(ctx, geminiSetupTimeout)
	defer cancel()
	conn, response, err := coderws.Dial(dialCtx, g.endpoint, &coderws.DialOptions{
		HTTPHeader: http.Header{"x-goog-api-key": []string{g.apiKey}},
	})
	if err != nil {
		return nil, fmt.Errorf("%w: connect: %v", ErrGeminiSetup, err)
	}
	if response != nil && response.StatusCode != http.StatusSwitchingProtocols {
		_ = conn.Close(coderws.StatusPolicyViolation, "unexpected handshake response")
		return nil, fmt.Errorf("%w: HTTP %d", ErrGeminiSetup, response.StatusCode)
	}
	conn.SetReadLimit(geminiMessageReadLimit)

	session := &geminiLiveSession{
		service: g,
		conn:    conn,
		request: request,
		out:     make(chan LiveEvent, 64),
		done:    make(chan struct{}),
		ready:   make(chan error, 1),
	}

	if err := session.writeJSON(dialCtx, g.setupMessage(request, target)); err != nil {
		_ = conn.CloseNow()
		return nil, fmt.Errorf("%w: send setup: %v", ErrGeminiSetup, err)
	}
	go session.readLoop()

	if err := session.waitForSetup(dialCtx); err != nil {
		session.Close(dialCtx)
		return nil, err
	}
	return session, nil
}

// setupMessage matches the official Live Translate WebSocket shape, where the
// transcription toggles and translationConfig live inside generationConfig.
func (g *GeminiTranslationService) setupMessage(request OpenRequest, target string) map[string]any {
	return map[string]any{
		"setup": map[string]any{
			"model": "models/" + g.model,
			"generationConfig": map[string]any{
				"responseModalities":       []string{"AUDIO"},
				"inputAudioTranscription":  map[string]any{},
				"outputAudioTranscription": map[string]any{},
				"translationConfig": map[string]any{
					"targetLanguageCode": target,
					"echoTargetLanguage": request.EchoTarget,
				},
			},
		},
	}
}

// geminiLanguageCode maps Symphonia languages to the BCP-47 codes supported by
// the Live Translate model.
func geminiLanguageCode(language Language) (string, error) {
	switch language {
	case LanguagePortuguese:
		return "pt-BR", nil
	case LanguageEnglish:
		return "en", nil
	case LanguageSpanish:
		return "es", nil
	case LanguageFrench:
		return "fr", nil
	default:
		return "", fmt.Errorf("%w: %q", ErrUnsupportedLanguage, language)
	}
}

// languageFromGeminiCode best-effort maps a detected provider language code
// back to a Symphonia language. It returns "" when the code is not mapped.
func languageFromGeminiCode(code string) Language {
	switch strings.ToLower(code) {
	case "pt", "pt-br", "pt-pt":
		return LanguagePortuguese
	case "en", "en-us", "en-gb":
		return LanguageEnglish
	case "es", "es-es":
		return LanguageSpanish
	case "fr", "fr-fr":
		return LanguageFrench
	default:
		return ""
	}
}

// liveMessage mirrors the BidiGenerateContent server messages we consume.
type liveMessage struct {
	SetupComplete map[string]any     `json:"setupComplete"`
	ServerContent *liveServerContent `json:"serverContent"`
	Error         *liveProtocolError `json:"error"`
	GoAway        *liveGoAway        `json:"goAway"`
}

type liveServerContent struct {
	InputTranscription        *liveTranscription `json:"inputTranscription"`
	InterimInputTranscription *liveTranscription `json:"interimInputTranscription"`
	OutputTranscription       *liveTranscription `json:"outputTranscription"`
	ModelTurn                 *liveModelTurn     `json:"modelTurn"`
	Interrupted               bool               `json:"interrupted"`
	GenerationComplete        bool               `json:"generationComplete"`
	TurnComplete              bool               `json:"turnComplete"`
}

type liveTranscription struct {
	Text         string `json:"text"`
	LanguageCode string `json:"languageCode"`
}

type liveModelTurn struct {
	Parts []livePart `json:"parts"`
}

type livePart struct {
	InlineData *liveInlineData `json:"inlineData"`
}

type liveInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type liveProtocolError struct {
	Code    json.RawMessage `json:"code"`
	Message string          `json:"message"`
}

type liveGoAway struct {
	TimeLeft string `json:"timeLeft"`
}

// geminiLiveSession is the provider-backed LiveSession implementation.
type geminiLiveSession struct {
	service *GeminiTranslationService
	conn    *coderws.Conn
	request OpenRequest

	out  chan LiveEvent
	done chan struct{}
	// ready receives nil once setup completes, or the failure reason.
	ready chan error

	writeMu   sync.Mutex
	readyOnce sync.Once
	closeOnce sync.Once
}

func (s *geminiLiveSession) Events() <-chan LiveEvent { return s.out }

func (s *geminiLiveSession) SendAudio(ctx context.Context, audio AudioInput) error {
	select {
	case <-s.done:
		return ErrSessionClosed
	default:
	}
	if len(audio.Data) == 0 {
		return ErrEmptyAudio
	}
	message := map[string]any{
		"realtimeInput": map[string]any{
			"audio": map[string]string{
				"data":     base64.StdEncoding.EncodeToString(audio.Data),
				"mimeType": geminiAudioMimeRate,
			},
		},
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if err := s.writeJSON(ctx, message); err != nil {
		return fmt.Errorf("%w: send audio: %v", ErrGeminiStream, err)
	}
	return nil
}

// signalEnd tells the provider the audio stream finished so the pending turn
// can be finalized before the connection is closed.
func (s *geminiLiveSession) signalEnd(ctx context.Context) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.writeJSON(ctx, map[string]any{"realtimeInput": map[string]any{"audioStreamEnd": true}})
}

func (s *geminiLiveSession) Close(ctx context.Context) error {
	s.closeOnce.Do(func() {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		_ = s.writeJSON(ctx, map[string]any{"realtimeInput": map[string]any{"audioStreamEnd": true}})
		close(s.done)
		_ = s.conn.Close(coderws.StatusNormalClosure, "session closed")
	})
	return nil
}

func (s *geminiLiveSession) writeJSON(ctx context.Context, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err := s.conn.Write(ctx, coderws.MessageText, payload); err != nil {
		return err
	}
	return nil
}

// waitForSetup blocks until the provider acknowledges the session.
func (s *geminiLiveSession) waitForSetup(ctx context.Context) error {
	select {
	case err := <-s.ready:
		return err
	case <-ctx.Done():
		return fmt.Errorf("%w: %v", ErrGeminiSetup, ctx.Err())
	}
}

func (s *geminiLiveSession) setReady(err error) {
	s.readyOnce.Do(func() {
		s.ready <- err
	})
}

func (s *geminiLiveSession) emit(event LiveEvent) bool {
	select {
	case <-s.done:
		return false
	case s.out <- event:
		return true
	}
}

func (s *geminiLiveSession) readLoop() {
	defer close(s.out)
	defer s.conn.CloseNow()
	for {
		if err := s.readMessage(); err != nil {
			s.setReady(err)
			if !s.isDone() && !errors.Is(err, ErrSessionClosed) {
				s.emit(LiveEvent{Kind: EventSessionError, Err: err})
			}
			return
		}
	}
}

func (s *geminiLiveSession) isDone() bool {
	select {
	case <-s.done:
		return true
	default:
		return false
	}
}

func (s *geminiLiveSession) readMessage() error {
	ctx, cancel := context.WithTimeout(context.Background(), geminiMaxSessionWait)
	defer cancel()
	messageType, payload, err := s.conn.Read(ctx)
	if err != nil {
		if s.isDone() || isNormalClose(err) {
			return ErrSessionClosed
		}
		return fmt.Errorf("%w: read: %v", ErrSessionFailed, err)
	}
	if messageType != coderws.MessageText {
		return nil
	}

	var message liveMessage
	if err := json.Unmarshal(payload, &message); err != nil {
		return fmt.Errorf("%w: decode server message: %v", ErrSessionFailed, err)
	}

	switch {
	case message.SetupComplete != nil:
		s.setReady(nil)
		return nil
	case message.Error != nil:
		err := fmt.Errorf("%w: provider error %s: %s", ErrSessionFailed,
			string(message.Error.Code), message.Error.Message)
		s.setReady(err)
		return err
	case message.GoAway != nil:
		s.emit(LiveEvent{Kind: EventSessionError,
			Err: fmt.Errorf("%w: server go_away (time left %q)", ErrSessionFailed, message.GoAway.TimeLeft)})
		return nil
	}

	if message.ServerContent == nil {
		return nil
	}
	content := message.ServerContent

	if content.InputTranscription != nil && content.InputTranscription.Text != "" {
		s.emit(LiveEvent{
			Kind:           EventInputTranscription,
			SourceText:     content.InputTranscription.Text,
			SourceLanguage: s.request.SourceLanguage,
		})
	}
	if content.OutputTranscription != nil && content.OutputTranscription.Text != "" {
		s.emit(LiveEvent{
			Kind:           EventOutputTranscription,
			OutputText:     content.OutputTranscription.Text,
			OutputLanguage: s.request.TargetLanguage,
		})
	}
	if content.ModelTurn != nil {
		for _, part := range content.ModelTurn.Parts {
			if part.InlineData == nil {
				continue
			}
			data, err := base64.StdEncoding.DecodeString(part.InlineData.Data)
			if err != nil {
				return fmt.Errorf("%w: decode translated audio: %v", ErrSessionFailed, err)
			}
			s.emit(LiveEvent{
				Kind:           EventTranslatedAudio,
				OutputLanguage: s.request.TargetLanguage,
				Audio: AudioOutput{
					Format: GeminiOutputFormat,
					Data:   data,
				},
			})
		}
	}
	if content.Interrupted {
		s.emit(LiveEvent{Kind: EventInterrupted})
	}
	if content.TurnComplete {
		s.emit(LiveEvent{Kind: EventTurnComplete})
	}
	return nil
}

func isNormalClose(err error) bool {
	if coderws.CloseStatus(err) == coderws.StatusNormalClosure ||
		coderws.CloseStatus(err) == coderws.StatusGoingAway {
		return true
	}
	return false
}
