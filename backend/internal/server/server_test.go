package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStaticRoutes(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "assets"), 0700); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"index.html":    "<!doctype html><title>Symphonia</title>",
		"assets/app.js": "console.log('asset')", "audio-capture-worklet.js": "registerProcessor('audio', class {})",
		".env": "must-not-be-served",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	handler, err := StaticHandler(dir)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("GET /", handler)
	for _, tc := range []struct {
		method, url string
		code        int
		content     string
	}{
		{"GET", "/", 200, "Symphonia"}, {"GET", "/call/abcd1234/setup", 200, "Symphonia"},
		{"GET", "/assets/app.js", 200, "console.log"}, {"GET", "/audio-capture-worklet.js", 200, "registerProcessor"},
		{"HEAD", "/home", 200, ""}, {"GET", "/api/missing", 404, ""}, {"GET", "/api", 404, ""},
		{"GET", "/assets", 404, ""}, {"GET", "/assets/missing", 404, ""},
		{"GET", "/missing.js", 404, ""}, {"GET", "/.env", 404, ""}, {"POST", "/home", 405, ""},
	} {
		t.Run(tc.method+tc.url, func(t *testing.T) {
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, httptest.NewRequest(tc.method, tc.url, nil))
			if w.Code != tc.code || !strings.Contains(w.Body.String(), tc.content) {
				t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
			}
			if tc.code != 200 && strings.Contains(w.Body.String(), "Symphonia") {
				t.Fatal("HTML fallback swallowed an error")
			}
		})
	}
}

func TestMissingBuildFails(t *testing.T) {
	if _, err := StaticHandler(t.TempDir()); err == nil {
		t.Fatal("missing build must fail startup")
	}
}

func TestHealthReadiness(t *testing.T) {
	for _, failed := range []bool{false, true} {
		handler := HealthHandler(func(ctx context.Context) error {
			if _, ok := ctx.Deadline(); !ok {
				t.Fatal("database ping must be bounded")
			}
			if failed {
				return errors.New("private database details")
			}
			return nil
		})
		w := httptest.NewRecorder()
		handler(w, httptest.NewRequest("GET", "/healthz", nil))
		want := 200
		if failed {
			want = 503
		}
		if w.Code != want || strings.Contains(w.Body.String(), "private") {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	}
}
