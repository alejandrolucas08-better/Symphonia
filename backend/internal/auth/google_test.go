package auth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/institucional/symphonia/backend/internal/database"
	"github.com/institucional/symphonia/backend/internal/user"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"
)

func TestGoogleIDTokenValidation(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	encoded := func(value any) string { b, _ := json.Marshal(value); return base64.RawURLEncoding.EncodeToString(b) }
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]string{
			"kid": "test-key", "kty": "RSA", "alg": "RS256",
			"n": base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString([]byte{1, 0, 1}),
		}}})
	}))
	defer server.Close()
	g := NewGoogleOAuth(nil, "http://localhost:5173")
	g.config.ClientID = "test-client"
	g.jwksURL = server.URL
	g.client = server.Client()
	identity := googleIdentity{Issuer: "https://accounts.google.com", Audience: "test-client", Subject: "google-subject", Email: "person@example.com", EmailVerified: true, Name: "Person", Nonce: "nonce-1", Expires: time.Now().Add(time.Hour).Unix()}
	sign := func(claims googleIdentity) string {
		unsigned := encoded(map[string]string{"alg": "RS256", "kid": "test-key"}) + "." + encoded(claims)
		digest := sha256.Sum256([]byte(unsigned))
		signature, signErr := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
		if signErr != nil {
			t.Fatal(signErr)
		}
		return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature)
	}
	if got, err := g.verifyIDToken(context.Background(), sign(identity), "nonce-1"); err != nil || got.Subject != identity.Subject {
		t.Fatalf("valid token: %+v, %v", got, err)
	}
	for name, change := range map[string]func(*googleIdentity){
		"wrong audience":   func(v *googleIdentity) { v.Audience = "other" },
		"unverified email": func(v *googleIdentity) { v.EmailVerified = false },
		"expired":          func(v *googleIdentity) { v.Expires = time.Now().Add(-time.Minute).Unix() },
		"wrong issuer":     func(v *googleIdentity) { v.Issuer = "https://example.com" },
	} {
		t.Run(name, func(t *testing.T) {
			changed := identity
			change(&changed)
			if _, err := g.verifyIDToken(context.Background(), sign(changed), "nonce-1"); err == nil {
				t.Fatal("accepted invalid identity")
			}
		})
	}
	if _, err := g.verifyIDToken(context.Background(), sign(identity), "other-nonce"); err == nil {
		t.Fatal("accepted wrong nonce")
	}
	raw := sign(identity)
	if _, err := g.verifyIDToken(context.Background(), raw[:len(raw)-2]+"ab", "nonce-1"); err == nil {
		t.Fatal("accepted bad signature")
	}
}

func TestOAuthStateRejectsTamperingAndExpiration(t *testing.T) {
	state := oauthState{State: "state", Verifier: "verifier", Nonce: "nonce", Expires: time.Now().Add(time.Minute).Unix()}
	raw, err := signState(state)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parseState(raw); err != nil {
		t.Fatal(err)
	}
	if _, err := parseState(raw + "x"); err == nil {
		t.Fatal("accepted tampered state")
	}
	state.Expires = time.Now().Add(-time.Minute).Unix()
	expired, _ := signState(state)
	if _, err := parseState(expired); err == nil {
		t.Fatal("accepted expired state")
	}
}

func TestGoogleOAuthRoundTripIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := database.RunMigrations(ctx, pool); err != nil {
		t.Fatal(err)
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	address := fmt.Sprintf("oauth-%d@example.com", time.Now().UnixNano())
	defer pool.Exec(ctx, `DELETE FROM users WHERE email=$1`, address)
	var nonce string
	encoded := func(value any) string { b, _ := json.Marshal(value); return base64.RawURLEncoding.EncodeToString(b) }
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/keys":
			_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]string{
				"kid": "test-key", "kty": "RSA", "alg": "RS256",
				"n": base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
				"e": base64.RawURLEncoding.EncodeToString([]byte{1, 0, 1}),
			}}})
		case "/token":
			w.Header().Set("Content-Type", "application/json")
			claims := googleIdentity{Issuer: "https://accounts.google.com", Audience: "oauth-test-client", Subject: "oauth-test-subject", Email: address, EmailVerified: true, Name: "OAuth Test", Nonce: nonce, Expires: time.Now().Add(time.Hour).Unix()}
			unsigned := encoded(map[string]string{"alg": "RS256", "kid": "test-key"}) + "." + encoded(claims)
			digest := sha256.Sum256([]byte(unsigned))
			sig, signErr := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
			if signErr != nil {
				t.Error(signErr)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "test-access", "token_type": "Bearer", "expires_in": 3600, "id_token": unsigned + "." + base64.RawURLEncoding.EncodeToString(sig)})
		default:
			http.NotFound(w, r)
		}
	}))
	defer provider.Close()
	store := NewStore(pool)
	g := NewGoogleOAuth(store, "http://localhost:5173")
	g.config.ClientID = "oauth-test-client"
	g.config.ClientSecret = "oauth-test-secret"
	g.config.Endpoint = oauth2.Endpoint{AuthURL: provider.URL + "/authorize", TokenURL: provider.URL + "/token"}
	g.jwksURL = provider.URL + "/keys"
	g.client = provider.Client()
	start := httptest.NewRecorder()
	g.Start(start, httptest.NewRequest(http.MethodGet, "/api/auth/google/start?return_to=%2Fhome", nil))
	if start.Code != http.StatusFound {
		t.Fatalf("start: %d %s", start.Code, start.Body.String())
	}
	redirect, err := url.Parse(start.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if redirect.Query().Get("code_challenge_method") != "S256" || redirect.Query().Get("nonce") == "" {
		t.Fatal("missing PKCE or nonce")
	}
	stateCookie := start.Result().Cookies()[0]
	state, err := parseState(stateCookie.Value)
	if err != nil {
		t.Fatal(err)
	}
	nonce = state.Nonce
	callbackRequest := httptest.NewRequest(http.MethodGet, "/api/auth/google/callback?state="+url.QueryEscape(state.State)+"&code=test-code", nil)
	callbackRequest.AddCookie(stateCookie)
	callback := httptest.NewRecorder()
	g.Callback(callback, callbackRequest)
	if callback.Code != http.StatusFound {
		t.Fatalf("callback: %d %s", callback.Code, callback.Body.String())
	}
	target, err := url.Parse(callback.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if target.Path != "/auth/google/complete" {
		t.Fatalf("unexpected callback target: %s", target.Path)
	}
	ticket := target.Query().Get("ticket")
	if ticket == "" {
		t.Fatal("missing one-time ticket")
	}
	h := NewHandler(user.NewRepository(pool), store, &testMailer{}, "http://localhost:5173")
	finishBody := strings.NewReader(fmt.Sprintf(`{"token":%q}`, ticket))
	finish := httptest.NewRecorder()
	h.FinishGoogle(finish, httptest.NewRequest(http.MethodPost, "/api/auth/google/finish", finishBody))
	if finish.Code != http.StatusOK || finish.Header().Get("Authorization") == "" {
		t.Fatalf("finish: %d %s", finish.Code, finish.Body.String())
	}
	replay := httptest.NewRecorder()
	h.FinishGoogle(replay, httptest.NewRequest(http.MethodPost, "/api/auth/google/finish", strings.NewReader(fmt.Sprintf(`{"token":%q}`, ticket))))
	if replay.Code != http.StatusBadRequest {
		t.Fatalf("ticket replay: %d", replay.Code)
	}
	u, err := user.NewRepository(pool).FindByEmail(ctx, address)
	if err != nil {
		t.Fatal(err)
	}
	linkRequest := httptest.NewRequest(http.MethodPost, "/api/auth/google/link/start", nil)
	linkRequest = linkRequest.WithContext(context.WithValue(linkRequest.Context(), contextKeyUserID, u.ID))
	linkStart := httptest.NewRecorder()
	g.StartLink(linkStart, linkRequest)
	if linkStart.Code != http.StatusOK {
		t.Fatalf("link start: %d %s", linkStart.Code, linkStart.Body.String())
	}
	var linkResponse struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(linkStart.Body.Bytes(), &linkResponse); err != nil {
		t.Fatal(err)
	}
	linkURL, err := url.Parse(linkResponse.URL)
	if err != nil {
		t.Fatal(err)
	}
	linkCookie := linkStart.Result().Cookies()[0]
	linkState, err := parseState(linkCookie.Value)
	if err != nil {
		t.Fatal(err)
	}
	if linkState.LinkUserID != u.ID || linkURL.Query().Get("state") != linkState.State {
		t.Fatal("link state is not bound to user")
	}
	nonce = linkState.Nonce
	linkCallbackRequest := httptest.NewRequest(http.MethodGet, "/api/auth/google/callback?state="+url.QueryEscape(linkState.State)+"&code=link-code", nil)
	linkCallbackRequest.AddCookie(linkCookie)
	linkCallback := httptest.NewRecorder()
	g.Callback(linkCallback, linkCallbackRequest)
	if linkCallback.Code != http.StatusFound {
		t.Fatalf("link callback: %d %s", linkCallback.Code, linkCallback.Body.String())
	}
	linkTarget, err := url.Parse(linkCallback.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if linkTarget.Path != "/auth/google/complete" || linkTarget.Query().Get("return_to") != "/account/password" {
		t.Fatalf("link callback target: %s", linkTarget.String())
	}
}
