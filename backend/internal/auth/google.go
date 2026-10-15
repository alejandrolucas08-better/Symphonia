package auth

import (
	"context"
	"crypto"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/institucional/symphonia/backend/internal/user"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type GoogleOAuth struct {
	config    oauth2.Config
	publicURL string
	store     *Store
	client    *http.Client
	jwksURL   string
}

func NewGoogleOAuth(store *Store, publicURL string) *GoogleOAuth {
	publicURL = strings.TrimRight(publicURL, "/")
	return &GoogleOAuth{
		config: oauth2.Config{
			ClientID: os.Getenv("GOOGLE_OAUTH_CLIENT_ID"), ClientSecret: os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET"),
			RedirectURL: publicURL + "/api/auth/google/callback", Scopes: []string{"openid", "email", "profile"}, Endpoint: google.Endpoint,
		},
		publicURL: publicURL, store: store, client: &http.Client{Timeout: 10 * time.Second},
		jwksURL: "https://www.googleapis.com/oauth2/v3/certs",
	}
}

func (g *GoogleOAuth) Enabled() bool {
	u, err := url.Parse(g.publicURL)
	return g.config.ClientID != "" && g.config.ClientSecret != "" && err == nil && u.Host != "" && u.User == nil && u.Path == "" && u.RawQuery == "" && u.Fragment == "" && (u.Scheme == "https" || (u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1")))
}

type oauthState struct {
	State       string `json:"state"`
	Verifier    string `json:"verifier"`
	Nonce       string `json:"nonce"`
	ReturnTo    string `json:"return_to"`
	Expires     int64  `json:"expires"`
	LinkUserID  int64  `json:"link_user_id,omitempty"`
	LinkVersion int    `json:"link_version,omitempty"`
}

func safeOAuthReturnTo(path string) string {
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") || strings.HasPrefix(path, "/\\") || strings.ContainsAny(path, "\r\n") {
		return "/home"
	}
	return path
}

func signState(value oauthState) (string, error) {
	b, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(b)
	mac := hmac.New(sha256.New, signingKey())
	mac.Write([]byte(encoded))
	return encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func parseState(raw string) (oauthState, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 2 {
		return oauthState{}, ErrInvalidToken
	}
	mac := hmac.New(sha256.New, signingKey())
	mac.Write([]byte(parts[0]))
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(mac.Sum(nil), sig) {
		return oauthState{}, ErrInvalidToken
	}
	b, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return oauthState{}, ErrInvalidToken
	}
	var state oauthState
	if json.Unmarshal(b, &state) != nil || state.Expires < time.Now().Unix() || state.State == "" || state.Verifier == "" || state.Nonce == "" {
		return oauthState{}, ErrInvalidToken
	}
	return state, nil
}

func (g *GoogleOAuth) Providers(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"google": g.Enabled()})
}

func (g *GoogleOAuth) Start(w http.ResponseWriter, r *http.Request) {
	if !g.Enabled() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Google sign-in is unavailable"})
		return
	}
	authorizationURL, err := g.begin(w, r, 0, 0, safeOAuthReturnTo(r.URL.Query().Get("return_to")))
	if err != nil {
		http.Error(w, "authentication unavailable", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, authorizationURL, http.StatusFound)
}

func (g *GoogleOAuth) StartLink(w http.ResponseWriter, r *http.Request) {
	if !g.Enabled() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Google sign-in is unavailable"})
		return
	}
	userID, _ := UserIDFromContext(r.Context())
	version, err := g.store.SessionVersion(r.Context(), userID)
	if err != nil {
		http.Error(w, "authentication unavailable", http.StatusInternalServerError)
		return
	}
	authorizationURL, err := g.begin(w, r, userID, version, "/account/password")
	if err != nil {
		http.Error(w, "authentication unavailable", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": authorizationURL})
}

func (g *GoogleOAuth) begin(w http.ResponseWriter, r *http.Request, linkUserID int64, linkVersion int, returnTo string) (string, error) {
	state, err := randomToken()
	if err != nil {
		return "", err
	}
	verifier, err := randomToken()
	if err != nil {
		return "", err
	}
	nonce, err := randomToken()
	if err != nil {
		return "", err
	}
	value, err := signState(oauthState{State: state, Verifier: verifier, Nonce: nonce, ReturnTo: returnTo, Expires: time.Now().Add(10 * time.Minute).Unix(), LinkUserID: linkUserID, LinkVersion: linkVersion})
	if err != nil {
		return "", err
	}
	http.SetCookie(w, &http.Cookie{Name: "symphonia_oauth_state", Value: value, Path: "/api/auth/google", HttpOnly: true, Secure: strings.HasPrefix(g.publicURL, "https://"), SameSite: http.SameSiteLaxMode, MaxAge: 600})
	return g.config.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier), oauth2.SetAuthURLParam("nonce", nonce)), nil
}

func (g *GoogleOAuth) Callback(w http.ResponseWriter, r *http.Request) {
	if !g.Enabled() {
		http.Error(w, "authentication unavailable", http.StatusServiceUnavailable)
		return
	}
	cookie, err := r.Cookie("symphonia_oauth_state")
	http.SetCookie(w, &http.Cookie{Name: "symphonia_oauth_state", Path: "/api/auth/google", HttpOnly: true, Secure: strings.HasPrefix(g.publicURL, "https://"), SameSite: http.SameSiteLaxMode, MaxAge: -1})
	if err != nil {
		g.failure(w, r, "google_login_failed")
		return
	}
	state, err := parseState(cookie.Value)
	if err != nil || !hmac.Equal([]byte(state.State), []byte(r.URL.Query().Get("state"))) || r.URL.Query().Get("code") == "" || r.URL.Query().Get("error") != "" {
		g.failure(w, r, "google_login_failed")
		return
	}
	if state.LinkUserID > 0 {
		version, versionErr := g.store.SessionVersion(r.Context(), state.LinkUserID)
		if versionErr != nil || version != state.LinkVersion {
			g.failure(w, r, "google_login_failed")
			return
		}
	}
	ctx := context.WithValue(r.Context(), oauth2.HTTPClient, g.client)
	token, err := g.config.Exchange(ctx, r.URL.Query().Get("code"), oauth2.VerifierOption(state.Verifier))
	if err != nil {
		log.Printf("Google code exchange failed: %v", err)
		g.failure(w, r, "google_login_failed")
		return
	}
	idToken, _ := token.Extra("id_token").(string)
	identity, err := g.verifyIDToken(r.Context(), idToken, state.Nonce)
	if err != nil {
		log.Printf("Google ID token verification failed: %v", err)
		g.failure(w, r, "google_login_failed")
		return
	}
	var userID int64
	if state.LinkUserID > 0 {
		err = g.store.LinkGoogle(r.Context(), state.LinkUserID, identity.Subject, identity.Email)
		userID = state.LinkUserID
		if errors.Is(err, ErrGoogleEmailMismatch) || errors.Is(err, ErrGoogleAlreadyLinked) {
			g.linkFailure(w, r, err)
			return
		}
	} else {
		var u *user.User
		u, err = g.store.GoogleUser(r.Context(), identity.Subject, identity.Email, identity.Name)
		if errors.Is(err, user.ErrEmailTaken) {
			g.failure(w, r, "google_email_exists")
			return
		}
		if err == nil {
			userID = u.ID
		}
	}
	if err != nil {
		log.Printf("Google account login failed: %v", err)
		g.failure(w, r, "google_login_failed")
		return
	}
	ticket, err := g.store.Issue(r.Context(), userID, "oauth_ticket", time.Minute, 0)
	if err != nil {
		log.Printf("Google ticket issue failed: %v", err)
		g.failure(w, r, "google_login_failed")
		return
	}
	target, _ := url.Parse(g.publicURL + "/auth/google/complete")
	q := target.Query()
	q.Set("ticket", ticket)
	q.Set("return_to", state.ReturnTo)
	target.RawQuery = q.Encode()
	http.Redirect(w, r, target.String(), http.StatusFound)
}

func (g *GoogleOAuth) failure(w http.ResponseWriter, r *http.Request, code string) {
	http.Redirect(w, r, g.publicURL+"/login?error="+url.QueryEscape(code), http.StatusFound)
}

func (g *GoogleOAuth) linkFailure(w http.ResponseWriter, r *http.Request, err error) {
	code := "google_link_failed"
	if errors.Is(err, ErrGoogleEmailMismatch) {
		code = "google_email_mismatch"
	}
	if errors.Is(err, ErrGoogleAlreadyLinked) {
		code = "google_already_linked"
	}
	http.Redirect(w, r, g.publicURL+"/account/password?google_error="+code, http.StatusFound)
}

func (h *Handler) FinishGoogle(w http.ResponseWriter, r *http.Request) {
	var req actionTokenRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	userID, err := h.store.ConsumeTicket(r.Context(), req.Token)
	if err != nil {
		h.actionError(w, "consume Google ticket", err)
		return
	}
	u, err := h.users.FindByID(r.Context(), userID)
	if err != nil {
		h.internalError(w, "find Google user", err)
		return
	}
	h.respondWithSession(w, u, http.StatusOK)
}

type googleIdentity struct {
	Issuer        string `json:"iss"`
	Audience      string `json:"aud"`
	Subject       string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Nonce         string `json:"nonce"`
	Expires       int64  `json:"exp"`
}

type jwk struct {
	KeyID     string `json:"kid"`
	Type      string `json:"kty"`
	Algorithm string `json:"alg"`
	N         string `json:"n"`
	E         string `json:"e"`
}

func (g *GoogleOAuth) verifyIDToken(ctx context.Context, raw, nonce string) (googleIdentity, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return googleIdentity{}, ErrInvalidToken
	}
	var header struct {
		Algorithm string `json:"alg"`
		KeyID     string `json:"kid"`
	}
	head, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return googleIdentity{}, err
	}
	if json.Unmarshal(head, &header) != nil || header.Algorithm != "RS256" || header.KeyID == "" {
		return googleIdentity{}, ErrInvalidToken
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.jwksURL, nil)
	if err != nil {
		return googleIdentity{}, err
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return googleIdentity{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return googleIdentity{}, fmt.Errorf("Google JWKS HTTP %d", resp.StatusCode)
	}
	var keys struct {
		Keys []jwk `json:"keys"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&keys); err != nil {
		return googleIdentity{}, err
	}
	var publicKey *rsa.PublicKey
	for _, key := range keys.Keys {
		if key.KeyID != header.KeyID || key.Type != "RSA" || (key.Algorithm != "" && key.Algorithm != "RS256") {
			continue
		}
		n, nErr := base64.RawURLEncoding.DecodeString(key.N)
		e, eErr := base64.RawURLEncoding.DecodeString(key.E)
		if nErr != nil || eErr != nil || len(e) == 0 || len(e) > 4 {
			return googleIdentity{}, ErrInvalidToken
		}
		exponent := 0
		for _, b := range e {
			exponent = exponent*256 + int(b)
		}
		if exponent < 3 {
			return googleIdentity{}, ErrInvalidToken
		}
		publicKey = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: exponent}
		break
	}
	if publicKey == nil {
		return googleIdentity{}, ErrInvalidToken
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return googleIdentity{}, err
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, digest[:], sig) != nil {
		return googleIdentity{}, ErrInvalidToken
	}
	b, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return googleIdentity{}, err
	}
	var identity googleIdentity
	if json.Unmarshal(b, &identity) != nil || (identity.Issuer != "https://accounts.google.com" && identity.Issuer != "accounts.google.com") || identity.Audience != g.config.ClientID || identity.Subject == "" || !identity.EmailVerified || !user.ValidateEmail(identity.Email) || identity.Nonce != nonce || identity.Expires <= time.Now().Unix() {
		return googleIdentity{}, ErrInvalidToken
	}
	return identity, nil
}
