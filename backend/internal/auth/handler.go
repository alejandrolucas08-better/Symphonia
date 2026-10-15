package auth

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/institucional/symphonia/backend/internal/user"
)

type Handler struct {
	users     *user.Repository
	store     *Store
	mailer    Mailer
	publicURL string
}

func NewHandler(users *user.Repository, store *Store, mailer Mailer, publicURL string) *Handler {
	return &Handler{users: users, store: store, mailer: mailer, publicURL: strings.TrimRight(publicURL, "/")}
}

type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userResponse struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Email         string `json:"email"`
	CreatedAt     string `json:"created_at"`
	EmailVerified bool   `json:"email_verified"`
	HasPassword   bool   `json:"has_password"`
	GoogleLinked  bool   `json:"google_linked"`
}

func sanitizeUser(u *user.User) userResponse {
	return userResponse{
		ID:            u.ID,
		Name:          u.Name,
		Email:         u.Email,
		CreatedAt:     u.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		EmailVerified: u.EmailVerifiedAt != nil,
		HasPassword:   u.HasPassword,
		GoogleLinked:  u.GoogleLinked,
	}
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	if !h.deliveryConfigured() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "email delivery is unavailable"})
		return
	}
	var req registerRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.TrimSpace(req.Email)

	if req.Name == "" || req.Email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name, email and password are required"})
		return
	}

	u, err := h.users.Create(r.Context(), req.Name, req.Email, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, user.ErrEmailTaken):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "email already registered"})
		case errors.Is(err, user.ErrInvalidEmail):
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid email format"})
		case errors.Is(err, user.ErrPasswordTooWeak):
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "password must be at least 8 characters"})
		default:
			log.Printf("register error: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		}
		return
	}

	token, err := h.store.Issue(r.Context(), u.ID, "verify_email", 24*time.Hour, 0)
	if err != nil {
		log.Printf("issue verification token: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate token"})
		return
	}
	if err := h.mailer.Send(r.Context(), u.Email, "Confirme seu email no Symphonia", "Confirme seu email abrindo este link (valido por 24 horas):\n"+h.publicURL+"/verify-email?token="+url.QueryEscape(token)); err != nil {
		log.Printf("send verification email: %v", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "email delivery is unavailable"})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"message": "verification email sent"})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	req.Email = strings.TrimSpace(req.Email)

	if req.Email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email and password are required"})
		return
	}

	u, err := h.users.FindByEmail(r.Context(), req.Email)
	if err != nil {
		if errors.Is(err, user.ErrUserNotFound) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid email or password"})
			return
		}
		log.Printf("login error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	if !u.CheckPassword(req.Password) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid email or password"})
		return
	}
	if u.EmailVerifiedAt == nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "email not verified"})
		return
	}

	token, err := GenerateTokenVersion(u.ID, u.AuthVersion)
	if err != nil {
		log.Printf("generate token: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate token"})
		return
	}

	w.Header().Set("Authorization", "Bearer "+token)
	writeJSON(w, http.StatusOK, sanitizeUser(u))
}

type emailRequest struct {
	Email string `json:"email"`
}
type actionTokenRequest struct {
	Token string `json:"token"`
}
type resetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}
type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (h *Handler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	if !h.deliveryConfigured() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "email delivery is unavailable"})
		return
	}
	var req emailRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if !user.ValidateEmail(strings.TrimSpace(req.Email)) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid email format"})
		return
	}
	u, err := h.users.FindByEmail(r.Context(), req.Email)
	if err != nil && !errors.Is(err, user.ErrUserNotFound) {
		h.internalError(w, "find account", err)
		return
	}
	if err == nil && u.EmailVerifiedAt == nil {
		token, issueErr := h.store.Issue(r.Context(), u.ID, "verify_email", 24*time.Hour, time.Minute)
		if issueErr != nil && !errors.Is(issueErr, ErrTooSoon) {
			log.Printf("issue verification token: %v", issueErr)
		}
		if issueErr == nil {
			if sendErr := h.mailer.Send(r.Context(), u.Email, "Confirme seu email no Symphonia", "Confirme seu email abrindo este link (valido por 24 horas):\n"+h.publicURL+"/verify-email?token="+url.QueryEscape(token)); sendErr != nil {
				log.Printf("send verification email: %v", sendErr)
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "if the address is pending verification, an email was sent"})
}

func (h *Handler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	var req actionTokenRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	userID, err := h.store.VerifyEmail(r.Context(), req.Token)
	if err != nil {
		h.actionError(w, "verify email", err)
		return
	}
	u, err := h.users.FindByID(r.Context(), userID)
	if err != nil {
		h.internalError(w, "find verified user", err)
		return
	}
	h.respondWithSession(w, u, http.StatusOK)
}

func (h *Handler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	if !h.deliveryConfigured() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "email delivery is unavailable"})
		return
	}
	var req emailRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if !user.ValidateEmail(strings.TrimSpace(req.Email)) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid email format"})
		return
	}
	u, err := h.users.FindByEmail(r.Context(), req.Email)
	if err != nil && !errors.Is(err, user.ErrUserNotFound) {
		h.internalError(w, "find account", err)
		return
	}
	if err == nil && u.EmailVerifiedAt != nil {
		token, issueErr := h.store.Issue(r.Context(), u.ID, "reset_password", 30*time.Minute, time.Minute)
		if issueErr != nil && !errors.Is(issueErr, ErrTooSoon) {
			log.Printf("issue reset token: %v", issueErr)
		}
		if issueErr == nil {
			if sendErr := h.mailer.Send(r.Context(), u.Email, "Redefina sua senha no Symphonia", "Redefina sua senha abrindo este link (valido por 30 minutos):\n"+h.publicURL+"/reset-password?token="+url.QueryEscape(token)); sendErr != nil {
				log.Printf("send reset email: %v", sendErr)
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "if the address has an account, an email was sent"})
}

func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req resetPasswordRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.store.ResetPassword(r.Context(), req.Token, req.NewPassword); err != nil {
		h.actionError(w, "reset password", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "password changed"})
}

func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var req changePasswordRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	userID, _ := UserIDFromContext(r.Context())
	version, err := h.store.ChangePassword(r.Context(), userID, req.CurrentPassword, req.NewPassword)
	if err != nil {
		h.actionError(w, "change password", err)
		return
	}
	token, err := GenerateTokenVersion(userID, version)
	if err != nil {
		h.internalError(w, "issue session", err)
		return
	}
	w.Header().Set("Authorization", "Bearer "+token)
	writeJSON(w, http.StatusOK, map[string]string{"message": "password changed"})
}

func (h *Handler) respondWithSession(w http.ResponseWriter, u *user.User, status int) {
	token, err := GenerateTokenVersion(u.ID, u.AuthVersion)
	if err != nil {
		h.internalError(w, "issue session", err)
		return
	}
	w.Header().Set("Authorization", "Bearer "+token)
	writeJSON(w, status, sanitizeUser(u))
}

func (h *Handler) internalError(w http.ResponseWriter, operation string, err error) {
	log.Printf("%s: %v", operation, err)
	status := http.StatusInternalServerError
	message := "internal server error"
	if errors.Is(err, ErrMailUnavailable) {
		status = http.StatusServiceUnavailable
		message = "email delivery is unavailable"
	}
	writeJSON(w, status, map[string]string{"error": message})
}

func (h *Handler) deliveryConfigured() bool {
	if h.publicURL == "" || h.mailer == nil {
		return false
	}
	origin, err := url.Parse(h.publicURL)
	if err != nil || origin.Host == "" || origin.User != nil || origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" || (origin.Scheme != "https" && !(origin.Scheme == "http" && (origin.Hostname() == "localhost" || origin.Hostname() == "127.0.0.1"))) {
		return false
	}
	if smtpMailer, ok := h.mailer.(*SMTPMailer); ok {
		return smtpMailer.Host != "" && smtpMailer.From != ""
	}
	return true
}

func (h *Handler) actionError(w http.ResponseWriter, operation string, err error) {
	switch {
	case errors.Is(err, ErrInvalidActionToken):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid or expired link"})
	case errors.Is(err, user.ErrPasswordTooWeak):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "password must be 8 to 72 bytes"})
	case errors.Is(err, ErrWrongPassword):
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "current password is incorrect"})
	case errors.Is(err, user.ErrPasswordMissing):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "account has no password"})
	default:
		h.internalError(w, operation, err)
	}
}

func (h *Handler) Session(w http.ResponseWriter, r *http.Request) {
	uid, ok := UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	u, err := h.users.FindByID(r.Context(), uid)
	if err != nil {
		if errors.Is(err, user.ErrUserNotFound) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "user not found"})
			return
		}
		log.Printf("session error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, sanitizeUser(u))
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("write json response: %v", err)
	}
}
