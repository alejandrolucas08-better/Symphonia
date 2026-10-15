package auth

import (
	"context"
	"errors"
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
)

type testMailer struct{ body string }

func (m *testMailer) Send(_ context.Context, _, _, body string) error { m.body = body; return nil }
func (m *testMailer) token(t *testing.T) string {
	t.Helper()
	fields := strings.Fields(m.body)
	if len(fields) == 0 {
		t.Fatal("no email was sent")
	}
	link, err := url.Parse(fields[len(fields)-1])
	if err != nil {
		t.Fatal(err)
	}
	return link.Query().Get("token")
}

func TestAccountFlowsIntegration(t *testing.T) {
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
	if err = database.RunMigrations(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err = database.RunMigrations(ctx, pool); err != nil {
		t.Fatalf("migrations are not idempotent: %v", err)
	}
	address := fmt.Sprintf("auth-%d@example.com", time.Now().UnixNano())
	defer pool.Exec(ctx, `DELETE FROM users WHERE email=$1`, address)
	googleAddress := fmt.Sprintf("google-%d@example.com", time.Now().UnixNano())
	defer pool.Exec(ctx, `DELETE FROM users WHERE email=$1`, googleAddress)
	store := NewStore(pool)
	mailer := &testMailer{}
	handler := NewHandler(user.NewRepository(pool), store, mailer, "http://localhost:5173")
	call := func(method, path, body string, handle http.HandlerFunc) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		w := httptest.NewRecorder()
		handle(w, r)
		return w
	}
	registration := fmt.Sprintf(`{"name":"Auth Test","email":%q,"password":"first-secret"}`, address)
	if got := call(http.MethodPost, "/api/register", registration, handler.Register); got.Code != http.StatusCreated || got.Header().Get("Authorization") != "" {
		t.Fatalf("register: %d %s", got.Code, got.Body.String())
	}
	if err = database.RunMigrations(ctx, pool); err != nil {
		t.Fatal(err)
	}
	loginBody := fmt.Sprintf(`{"email":%q,"password":"first-secret"}`, address)
	if got := call(http.MethodPost, "/api/login", loginBody, handler.Login); got.Code != http.StatusForbidden {
		t.Fatalf("unverified login: %d", got.Code)
	}
	verifyToken := mailer.token(t)
	verify := fmt.Sprintf(`{"token":%q}`, verifyToken)
	if got := call(http.MethodPost, "/api/auth/verify-email", verify, handler.VerifyEmail); got.Code != http.StatusOK {
		t.Fatalf("verify: %d %s", got.Code, got.Body.String())
	}
	if got := call(http.MethodPost, "/api/auth/verify-email", verify, handler.VerifyEmail); got.Code != http.StatusBadRequest {
		t.Fatalf("replayed verification: %d", got.Code)
	}
	login := call(http.MethodPost, "/api/login", loginBody, handler.Login)
	if login.Code != http.StatusOK {
		t.Fatalf("verified login: %d %s", login.Code, login.Body.String())
	}
	oldToken := strings.TrimPrefix(login.Header().Get("Authorization"), "Bearer ")
	u, err := user.NewRepository(pool).FindByEmail(ctx, address)
	if err != nil {
		t.Fatal(err)
	}
	changeTicket, err := store.Issue(ctx, u.ID, "oauth_ticket", time.Minute, 0)
	if err != nil {
		t.Fatal(err)
	}
	change := httptest.NewRequest(http.MethodPost, "/api/auth/change-password", strings.NewReader(`{"current_password":"first-secret","new_password":"second-secret"}`))
	change = change.WithContext(context.WithValue(change.Context(), contextKeyUserID, u.ID))
	changed := httptest.NewRecorder()
	handler.ChangePassword(changed, change)
	if changed.Code != http.StatusOK {
		t.Fatalf("change: %d %s", changed.Code, changed.Body.String())
	}
	if _, err = ValidateSession(ctx, store, oldToken); err == nil {
		t.Fatal("old session survived password change")
	}
	if _, err = store.ConsumeTicket(ctx, changeTicket); !errors.Is(err, ErrInvalidActionToken) {
		t.Fatalf("OAuth ticket survived password change: %v", err)
	}
	currentToken := strings.TrimPrefix(changed.Header().Get("Authorization"), "Bearer ")
	if _, err = ValidateSession(ctx, store, currentToken); err != nil {
		t.Fatalf("new session invalid: %v", err)
	}
	forgot := fmt.Sprintf(`{"email":%q}`, address)
	if got := call(http.MethodPost, "/api/auth/forgot-password", forgot, handler.ForgotPassword); got.Code != http.StatusOK {
		t.Fatalf("forgot: %d %s", got.Code, got.Body.String())
	}
	resetToken := mailer.token(t)
	resetTicket, err := store.Issue(ctx, u.ID, "oauth_ticket", time.Minute, 0)
	if err != nil {
		t.Fatal(err)
	}
	reset := fmt.Sprintf(`{"token":%q,"new_password":"third-secret"}`, resetToken)
	if got := call(http.MethodPost, "/api/auth/reset-password", reset, handler.ResetPassword); got.Code != http.StatusOK {
		t.Fatalf("reset: %d %s", got.Code, got.Body.String())
	}
	if got := call(http.MethodPost, "/api/auth/reset-password", reset, handler.ResetPassword); got.Code != http.StatusBadRequest {
		t.Fatalf("replayed reset: %d", got.Code)
	}
	if _, err = ValidateSession(ctx, store, currentToken); err == nil {
		t.Fatal("session survived password reset")
	}
	if _, err = store.ConsumeTicket(ctx, resetTicket); !errors.Is(err, ErrInvalidActionToken) {
		t.Fatalf("OAuth ticket survived password reset: %v", err)
	}
	if got := call(http.MethodPost, "/api/login", fmt.Sprintf(`{"email":%q,"password":"third-secret"}`, address), handler.Login); got.Code != http.StatusOK {
		t.Fatalf("login after reset: %d", got.Code)
	}
	if _, err = store.GoogleUser(ctx, "new-google-subject", address, "Person"); err != user.ErrEmailTaken {
		t.Fatalf("Google email collision: %v", err)
	}
	linkedSubject := fmt.Sprintf("linked-sub-%d", time.Now().UnixNano())
	if err = store.LinkGoogle(ctx, u.ID, linkedSubject, "different@example.com"); err != ErrGoogleEmailMismatch {
		t.Fatalf("mismatched link: %v", err)
	}
	if err = store.LinkGoogle(ctx, u.ID, linkedSubject, address); err != nil {
		t.Fatalf("link Google: %v", err)
	}
	if err = store.LinkGoogle(ctx, u.ID, linkedSubject, address); err != nil {
		t.Fatalf("idempotent Google link: %v", err)
	}
	linked, err := user.NewRepository(pool).FindByID(ctx, u.ID)
	if err != nil || !linked.GoogleLinked {
		t.Fatalf("linked account: %+v %v", linked, err)
	}
	viaGoogle, err := store.GoogleUser(ctx, linkedSubject, address, "Other Name")
	if err != nil || viaGoogle.ID != u.ID || !viaGoogle.HasPassword {
		t.Fatalf("linked Google login: %+v %v", viaGoogle, err)
	}
	googleSubject := fmt.Sprintf("google-sub-%d", time.Now().UnixNano())
	googleUser, err := store.GoogleUser(ctx, googleSubject, googleAddress, "Google Person")
	if err != nil || googleUser.EmailVerifiedAt == nil || googleUser.HasPassword {
		t.Fatalf("new Google account: %+v %v", googleUser, err)
	}
	again, err := store.GoogleUser(ctx, googleSubject, googleAddress, "Changed Name")
	if err != nil || again.ID != googleUser.ID {
		t.Fatalf("existing Google account: %+v %v", again, err)
	}
}
