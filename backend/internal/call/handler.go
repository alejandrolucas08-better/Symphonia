package call

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/institucional/symphonia/backend/internal/auth"
)

type callRepository interface {
	Create(context.Context, int64, string, Language, Language) (*Call, error)
	GetByCode(context.Context, string) (*Call, error)
	Join(context.Context, string, int64, Language, Language) (*Call, error)
	UpdateLanguage(context.Context, string, int64, Language, Language) (*Call, error)
	Leave(context.Context, string, int64) (*Call, error)
	End(context.Context, string, int64) (*Call, error)
}

type Handler struct {
	calls    callRepository
	notifier Notifier
}

type Notifier interface {
	ParticipantJoined(code string, participant Participant)
	LanguageChanged(code string, participant Participant)
	ParticipantLeft(code string, userID int64)
	CallEnded(code string, endedByUserID int64)
}

type languageRequest struct {
	SpokenLanguage Language `json:"spoken_language"`
	HeardLanguage  Language `json:"heard_language"`
}

type createRequest struct {
	Name string `json:"name"`
	languageRequest
}

const defaultCallName = "Meeting"

func NewHandler(calls callRepository, notifiers ...Notifier) *Handler {
	var notifier Notifier
	if len(notifiers) > 0 {
		notifier = notifiers[0]
	}
	return &Handler{calls: calls, notifier: notifier}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var request createRequest
	if !decodeRequest(w, r, &request) {
		return
	}
	request.Name = strings.TrimSpace(request.Name)
	if request.Name == "" {
		request.Name = defaultCallName
	}
	if utf8.RuneCountInString(request.Name) > 80 || strings.ContainsFunc(request.Name, unicode.IsControl) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name must contain 1 to 80 characters"})
		return
	}
	if !validLanguages(request.languageRequest) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "spoken_language and heard_language must be supported languages"})
		return
	}
	userID, _ := auth.UserIDFromContext(r.Context())
	result, err := h.calls.Create(r.Context(), userID, request.Name, request.SpokenLanguage, request.HeardLanguage)
	h.respond(w, result, err, http.StatusCreated)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	result, err := h.calls.GetByCode(r.Context(), r.PathValue("code"))
	h.respond(w, result, err, http.StatusOK)
}

func (h *Handler) Join(w http.ResponseWriter, r *http.Request) {
	request, ok := decodeLanguages(w, r)
	if !ok {
		return
	}
	userID, _ := auth.UserIDFromContext(r.Context())
	result, err := h.calls.Join(r.Context(), r.PathValue("code"), userID, request.SpokenLanguage, request.HeardLanguage)
	if err == nil && h.notifier != nil {
		if participant, ok := participantForUser(result, userID); ok {
			h.notifier.ParticipantJoined(result.Code, participant)
		}
	}
	h.respond(w, result, err, http.StatusOK)
}

func (h *Handler) UpdateLanguage(w http.ResponseWriter, r *http.Request) {
	request, ok := decodeLanguages(w, r)
	if !ok {
		return
	}
	userID, _ := auth.UserIDFromContext(r.Context())
	result, err := h.calls.UpdateLanguage(r.Context(), r.PathValue("code"), userID, request.SpokenLanguage, request.HeardLanguage)
	if err == nil && h.notifier != nil {
		if participant, ok := participantForUser(result, userID); ok {
			h.notifier.LanguageChanged(result.Code, participant)
		}
	}
	h.respond(w, result, err, http.StatusOK)
}

func (h *Handler) Leave(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	result, err := h.calls.Leave(r.Context(), r.PathValue("code"), userID)
	if err == nil && result != nil && h.notifier != nil {
		h.notifier.ParticipantLeft(result.Code, userID)
	}
	h.respond(w, result, err, http.StatusOK)
}

func (h *Handler) End(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	result, err := h.calls.End(r.Context(), r.PathValue("code"), userID)
	if err == nil && result != nil && h.notifier != nil {
		h.notifier.CallEnded(result.Code, userID)
	}
	h.respond(w, result, err, http.StatusOK)
}

func participantForUser(result *Call, userID int64) (Participant, bool) {
	if result == nil {
		return Participant{}, false
	}
	for _, participant := range result.Participants {
		if participant.UserID == userID {
			return participant, true
		}
	}
	return Participant{}, false
}

func decodeLanguages(w http.ResponseWriter, r *http.Request) (languageRequest, bool) {
	var request languageRequest
	if !decodeRequest(w, r, &request) {
		return request, false
	}
	if !validLanguages(request) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "spoken_language and heard_language must be supported languages"})
		return request, false
	}
	return request, true
}

func decodeRequest(w http.ResponseWriter, r *http.Request, request any) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return false
	}
	return true
}

func validLanguages(request languageRequest) bool {
	return ValidLanguage(request.SpokenLanguage) && ValidLanguage(request.HeardLanguage)
}

func (h *Handler) respond(w http.ResponseWriter, result *Call, err error, successStatus int) {
	if err == nil {
		writeJSON(w, successStatus, result)
		return
	}
	switch {
	case errors.Is(err, ErrCallNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "call not found"})
	case errors.Is(err, ErrNotParticipant):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "you are not an active participant"})
	case errors.Is(err, ErrNotHost):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "only the host can end this call"})
	case errors.Is(err, ErrCallFull):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "call is full"})
	case errors.Is(err, ErrCallEnded):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "call has ended"})
	default:
		log.Printf("call handler error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write call response: %v", err)
	}
}
