package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCORSOnlyAllowsConfiguredOrigins(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example")
	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, test := range []struct {
		name        string
		origin      string
		status      int
		allowOrigin string
	}{
		{name: "configured origin", origin: "https://app.example", status: http.StatusNoContent, allowOrigin: "https://app.example"},
		{name: "untrusted origin", origin: "https://evil.example", status: http.StatusNoContent},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/session", nil)
			request.Header.Set("Origin", test.origin)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}
			if got := response.Header().Get("Access-Control-Allow-Origin"); got != test.allowOrigin {
				t.Fatalf("Access-Control-Allow-Origin = %q, want %q", got, test.allowOrigin)
			}
		})
	}
}

func TestCORSRejectsUntrustedPreflight(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example")
	handler := CORS(http.NotFoundHandler())
	request := httptest.NewRequest(http.MethodOptions, "/api/session", nil)
	request.Header.Set("Origin", "https://evil.example")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestDecodeJSONRejectsUnknownFieldsAndOversizedBodies(t *testing.T) {
	t.Run("unknown field", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"email":"user@example.com","extra":true}`))
		response := httptest.NewRecorder()
		var destination struct {
			Email string `json:"email"`
		}

		if decodeJSON(response, request, &destination) {
			t.Fatal("decodeJSON accepted an unknown field")
		}
		if response.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
		}
	})

	t.Run("oversized body", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"email":"`+strings.Repeat("a", maxJSONBodyBytes)+`"}`))
		response := httptest.NewRecorder()
		var destination struct {
			Email string `json:"email"`
		}

		if decodeJSON(response, request, &destination) {
			t.Fatal("decodeJSON accepted an oversized body")
		}
		if response.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusRequestEntityTooLarge)
		}
	})
}

func TestSecurityHeaders(t *testing.T) {
	response := httptest.NewRecorder()
	SecurityHeaders(http.NotFoundHandler()).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	for header, want := range map[string]string{
		"Cache-Control":          "no-store",
		"Referrer-Policy":        "no-referrer",
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
	} {
		if got := response.Header().Get(header); got != want {
			t.Fatalf("%s = %q, want %q", header, got, want)
		}
	}
}
