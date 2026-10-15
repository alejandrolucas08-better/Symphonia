package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/institucional/symphonia/backend/internal/user"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidActionToken  = errors.New("invalid or expired link")
	ErrTooSoon             = errors.New("please wait before requesting another email")
	ErrWrongPassword       = errors.New("current password is incorrect")
	ErrGoogleEmailMismatch = errors.New("Google email does not match account")
	ErrGoogleAlreadyLinked = errors.New("Google account is already linked")
)

type Store struct{ db *pgxpool.Pool }

func NewStore(db *pgxpool.Pool) *Store { return &Store{db: db} }

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (s *Store) Issue(ctx context.Context, userID int64, purpose string, ttl, cooldown time.Duration) (string, error) {
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	var id int64
	if err = tx.QueryRow(ctx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, userID).Scan(&id); err != nil {
		return "", err
	}
	if cooldown > 0 {
		var recent bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM auth_tokens WHERE user_id=$1 AND purpose=$2 AND created_at > NOW()-($3 * INTERVAL '1 second'))`, userID, purpose, cooldown.Seconds()).Scan(&recent)
		if err != nil {
			return "", err
		}
		if recent {
			return "", ErrTooSoon
		}
	}
	if _, err = tx.Exec(ctx, `DELETE FROM auth_tokens WHERE user_id=$1 AND purpose=$2`, userID, purpose); err != nil {
		return "", err
	}
	hash := sha256.Sum256([]byte(token))
	if _, err = tx.Exec(ctx, `INSERT INTO auth_tokens (token_hash,user_id,purpose,expires_at) VALUES ($1,$2,$3,NOW()+($4 * INTERVAL '1 second'))`, hash[:], userID, purpose, ttl.Seconds()); err != nil {
		return "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", err
	}
	return token, nil
}

func consumeToken(ctx context.Context, tx pgx.Tx, token, purpose string) (int64, error) {
	if len(token) != 43 {
		return 0, ErrInvalidActionToken
	}
	hash := sha256.Sum256([]byte(token))
	var userID int64
	err := tx.QueryRow(ctx, `DELETE FROM auth_tokens WHERE token_hash=$1 AND purpose=$2 AND expires_at > NOW() RETURNING user_id`, hash[:], purpose).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrInvalidActionToken
	}
	return userID, err
}

func (s *Store) VerifyEmail(ctx context.Context, token string) (int64, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	userID, err := consumeToken(ctx, tx, token, "verify_email")
	if err != nil {
		return 0, err
	}
	if _, err = tx.Exec(ctx, `UPDATE users SET email_verified_at=NOW(), updated_at=NOW() WHERE id=$1`, userID); err != nil {
		return 0, err
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return userID, nil
}

func (s *Store) ResetPassword(ctx context.Context, token, password string) error {
	if len(token) != 43 {
		return ErrInvalidActionToken
	}
	if len(password) < 8 || len(password) > 72 {
		return user.ErrPasswordTooWeak
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	userID, err := consumeToken(ctx, tx, token, "reset_password")
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE users SET password_hash=$1, auth_version=auth_version+1, updated_at=NOW() WHERE id=$2`, string(hash), userID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM auth_tokens WHERE user_id=$1 AND purpose IN ('reset_password','oauth_ticket')`, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) ChangePassword(ctx context.Context, userID int64, current, next string) (int, error) {
	if len(next) < 8 || len(next) > 72 {
		return 0, user.ErrPasswordTooWeak
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	var oldHash *string
	err = tx.QueryRow(ctx, `SELECT password_hash FROM users WHERE id=$1 FOR UPDATE`, userID).Scan(&oldHash)
	if err != nil {
		return 0, err
	}
	if oldHash == nil {
		return 0, user.ErrPasswordMissing
	}
	if bcrypt.CompareHashAndPassword([]byte(*oldHash), []byte(current)) != nil {
		return 0, ErrWrongPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(next), bcrypt.DefaultCost)
	if err != nil {
		return 0, err
	}
	var version int
	err = tx.QueryRow(ctx, `UPDATE users SET password_hash=$1,auth_version=auth_version+1,updated_at=NOW() WHERE id=$2 RETURNING auth_version`, string(hash), userID).Scan(&version)
	if err != nil {
		return 0, err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM auth_tokens WHERE user_id=$1 AND purpose IN ('reset_password','oauth_ticket')`, userID); err != nil {
		return 0, err
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return version, nil
}

func (s *Store) ConsumeTicket(ctx context.Context, token string) (int64, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	userID, err := consumeToken(ctx, tx, token, "oauth_ticket")
	if err != nil {
		return 0, err
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return userID, nil
}

func (s *Store) SessionVersion(ctx context.Context, userID int64) (int, error) {
	var version int
	if err := s.db.QueryRow(ctx, `SELECT auth_version FROM users WHERE id=$1`, userID).Scan(&version); err != nil {
		return 0, err
	}
	return version, nil
}

func (s *Store) GoogleUser(ctx context.Context, subject, email, name string) (*user.User, error) {
	if subject == "" || len(subject) > 255 || !user.ValidateEmail(email) {
		return nil, errors.New("invalid Google identity")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = strings.Split(email, "@")[0]
	}
	if len([]rune(name)) > 60 {
		name = string([]rune(name)[:60])
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	u := &user.User{}
	err = tx.QueryRow(ctx, `SELECT u.id,u.name,u.email,COALESCE(u.password_hash,''),u.created_at,u.updated_at,u.email_verified_at,u.auth_version
		FROM oauth_identities i JOIN users u ON u.id=i.user_id WHERE i.provider='google' AND i.subject=$1`, subject).
		Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt, &u.EmailVerifiedAt, &u.AuthVersion)
	if err == nil {
		u.HasPassword = u.PasswordHash != ""
		u.GoogleLinked = true
		return u, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("find Google identity: %w", err)
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE LOWER(email)=LOWER($1))`, email).Scan(&exists); err != nil {
		return nil, err
	}
	if exists {
		return nil, user.ErrEmailTaken
	}
	err = tx.QueryRow(ctx, `INSERT INTO users (name,email,password_hash,email_verified_at) VALUES ($1,LOWER($2),NULL,NOW()) RETURNING id,name,email,created_at,updated_at,email_verified_at,auth_version`, name, email).
		Scan(&u.ID, &u.Name, &u.Email, &u.CreatedAt, &u.UpdatedAt, &u.EmailVerifiedAt, &u.AuthVersion)
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO oauth_identities (provider,subject,user_id) VALUES ('google',$1,$2)`, subject, u.ID); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	u.GoogleLinked = true
	return u, nil
}

func (s *Store) LinkGoogle(ctx context.Context, userID int64, subject, email string) error {
	if subject == "" || !user.ValidateEmail(email) {
		return ErrInvalidToken
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var accountEmail string
	var verified bool
	err = tx.QueryRow(ctx, `SELECT email,email_verified_at IS NOT NULL FROM users WHERE id=$1 FOR UPDATE`, userID).Scan(&accountEmail, &verified)
	if err != nil {
		return err
	}
	if !verified || !strings.EqualFold(accountEmail, email) {
		return ErrGoogleEmailMismatch
	}
	if _, err = tx.Exec(ctx, `INSERT INTO oauth_identities (provider,subject,user_id) VALUES ('google',$1,$2) ON CONFLICT DO NOTHING`, subject, userID); err != nil {
		return err
	}
	var linkedUserID int64
	err = tx.QueryRow(ctx, `SELECT user_id FROM oauth_identities WHERE provider='google' AND subject=$1`, subject).Scan(&linkedUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrGoogleAlreadyLinked
	}
	if err != nil {
		return err
	}
	if linkedUserID != userID {
		return ErrGoogleAlreadyLinked
	}
	return tx.Commit(ctx)
}
