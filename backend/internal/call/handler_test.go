package call

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/institucional/symphonia/backend/internal/auth"
)

type fakeRepository struct {
	createUserID int64
	result       *Call
	err          error
}

func (f *fakeRepository) Create(_ context.Context, userID int64, _, _ Language) (*Call, error) {
	f.createUserID = userID
	return f.result, f.err
}

func (f *fakeRepository) GetByCode(context.Context, string) (*Call, error) {
	return f.result, f.err
}

func (f *fakeRepository) Join(context.Context, string, int64, Language, Language) (*Call, error) {
	return f.result, f.err
}

func (f *fakeRepository) UpdateLanguage(context.Context, string, int64, Language, Language) (*Call, error) {
	return f.result, f.err
}

func (f *fakeRepository) Leave(context.Context, string, int64) (*Call, error) {
	return f.result, f.err
}

func (f *fakeRepository) End(context.Context, string, int64) (*Call, error) {
	return f.result, f.err
}

func TestValidLanguage(t *testing.T) {
	for _, language := range []Language{LanguagePortuguese, LanguageEnglish, LanguageSpanish, LanguageFrench} {
		if !ValidLanguage(language) {
			t.Errorf("ValidLanguage(%q) = false", language)
		}
	}
	for _, language := range []Language{"", "en-US", "DE-DE", "EN"} {
		if ValidLanguage(language) {
			t.Errorf("ValidLanguage(%q) = true", language)
		}
	}
}

func TestCreateValidatesLanguages(t *testing.T) {
	repository := &fakeRepository{}
	handler := NewHandler(repository)
	request := httptest.NewRequest(http.MethodPost, "/api/calls", strings.NewReader(
		`{"spoken_language":"EN-US","heard_language":"DE-DE"}`))
	response := httptest.NewRecorder()

	handler.Create(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if repository.createUserID != 0 {
		t.Fatal("repository was called for an invalid language")
	}
	assertError(t, response, "spoken_language and heard_language must be supported languages")
}

func TestCreateUsesAuthenticatedUser(t *testing.T) {
	repository := &fakeRepository{result: &Call{ID: 9, Code: "abcd1234", Status: StatusWaiting, Participants: []Participant{}}}
	handler := NewHandler(repository)
	token, err := auth.GenerateToken(42)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/calls", strings.NewReader(
		`{"spoken_language":"EN-US","heard_language":"PT-BR"}`))
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()

	auth.Middleware(handler.Create)(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusCreated, response.Body.String())
	}
	if repository.createUserID != 42 {
		t.Fatalf("create user ID = %d, want 42", repository.createUserID)
	}
}

func TestHandlerErrorStatuses(t *testing.T) {
	tests := []struct {
		err     error
		status  int
		message string
	}{
		{ErrCallNotFound, http.StatusNotFound, "call not found"},
		{ErrNotParticipant, http.StatusForbidden, "you are not an active participant"},
		{ErrNotHost, http.StatusForbidden, "only the host can end this call"},
		{ErrCallFull, http.StatusConflict, "call is full"},
		{ErrCallEnded, http.StatusConflict, "call has ended"},
		{errors.New("database unavailable"), http.StatusInternalServerError, "internal server error"},
	}
	for _, test := range tests {
		t.Run(test.message, func(t *testing.T) {
			handler := NewHandler(&fakeRepository{err: test.err})
			request := httptest.NewRequest(http.MethodGet, "/api/calls/missing", nil)
			response := httptest.NewRecorder()
			handler.Get(response, request)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}
			assertError(t, response, test.message)
		})
	}
}

func assertError(t *testing.T, response *httptest.ResponseRecorder, expected string) {
	t.Helper()
	var body map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["error"] != expected {
		t.Fatalf("error = %q, want %q", body["error"], expected)
	}
}
