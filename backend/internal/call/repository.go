package call

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrCallNotFound   = errors.New("call not found")
	ErrCallFull       = errors.New("call is full")
	ErrCallEnded      = errors.New("call has ended")
	ErrNotParticipant = errors.New("user is not an active participant")
	ErrNotHost        = errors.New("user is not the call host")
)

const codeAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func randomCode() (string, error) {
	code := make([]byte, 8)
	limit := big.NewInt(int64(len(codeAlphabet)))
	for i := range code {
		index, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return "", fmt.Errorf("generate call code: %w", err)
		}
		code[i] = codeAlphabet[index.Int64()]
	}
	return string(code), nil
}

func (r *Repository) Create(ctx context.Context, userID int64, spoken, heard Language) (*Call, error) {
	for attempt := 0; attempt < 5; attempt++ {
		code, err := randomCode()
		if err != nil {
			return nil, err
		}
		tx, err := r.db.Begin(ctx)
		if err != nil {
			return nil, fmt.Errorf("begin create call: %w", err)
		}

		var callID int64
		err = tx.QueryRow(ctx,
			`INSERT INTO calls (code, host_user_id) VALUES ($1, $2) RETURNING id`,
			code, userID,
		).Scan(&callID)
		if err != nil {
			_ = tx.Rollback(ctx)
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				continue
			}
			return nil, fmt.Errorf("insert call: %w", err)
		}

		_, err = tx.Exec(ctx,
			`INSERT INTO call_participants (call_id, user_id, spoken_language, heard_language)
			 VALUES ($1, $2, $3, $4)`, callID, userID, spoken, heard)
		if err != nil {
			_ = tx.Rollback(ctx)
			return nil, fmt.Errorf("insert host participant: %w", err)
		}
		created, err := loadCall(ctx, tx, code)
		if err != nil {
			_ = tx.Rollback(ctx)
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit create call: %w", err)
		}
		return created, nil
	}
	return nil, errors.New("could not generate a unique call code")
}

func (r *Repository) GetByCode(ctx context.Context, code string) (*Call, error) {
	return loadCall(ctx, r.db, code)
}

func (r *Repository) Join(ctx context.Context, code string, userID int64, spoken, heard Language) (*Call, error) {
	return r.changeLocked(ctx, code, func(tx pgx.Tx, callID, _ int64, status Status) error {
		if status == StatusEnded {
			return ErrCallEnded
		}

		var active bool
		err := tx.QueryRow(ctx,
			`SELECT left_at IS NULL FROM call_participants WHERE call_id = $1 AND user_id = $2`,
			callID, userID).Scan(&active)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("find participant: %w", err)
		}

		var activeCount int
		if err := tx.QueryRow(ctx,
			`SELECT COUNT(*) FROM call_participants WHERE call_id = $1 AND left_at IS NULL`,
			callID).Scan(&activeCount); err != nil {
			return fmt.Errorf("count participants: %w", err)
		}
		if !active && activeCount >= 2 {
			return ErrCallFull
		}

		if active {
			_, err = tx.Exec(ctx,
				`UPDATE call_participants SET spoken_language = $3, heard_language = $4
				 WHERE call_id = $1 AND user_id = $2`, callID, userID, spoken, heard)
		} else {
			_, err = tx.Exec(ctx,
				`INSERT INTO call_participants (call_id, user_id, spoken_language, heard_language)
				 VALUES ($1, $2, $3, $4)
				 ON CONFLICT (call_id, user_id) DO UPDATE SET
				 spoken_language = EXCLUDED.spoken_language,
				 heard_language = EXCLUDED.heard_language,
				 joined_at = NOW(), left_at = NULL`, callID, userID, spoken, heard)
			activeCount++
		}
		if err != nil {
			return fmt.Errorf("save participant: %w", err)
		}

		newStatus := StatusWaiting
		if activeCount == 2 {
			newStatus = StatusActive
		}
		_, err = tx.Exec(ctx, `UPDATE calls SET status = $2, updated_at = NOW() WHERE id = $1`, callID, newStatus)
		return err
	})
}

func (r *Repository) UpdateLanguage(ctx context.Context, code string, userID int64, spoken, heard Language) (*Call, error) {
	return r.changeLocked(ctx, code, func(tx pgx.Tx, callID, _ int64, status Status) error {
		if status == StatusEnded {
			return ErrCallEnded
		}
		result, err := tx.Exec(ctx,
			`UPDATE call_participants SET spoken_language = $3, heard_language = $4
			 WHERE call_id = $1 AND user_id = $2 AND left_at IS NULL`, callID, userID, spoken, heard)
		if err != nil {
			return fmt.Errorf("update participant languages: %w", err)
		}
		if result.RowsAffected() == 0 {
			return ErrNotParticipant
		}
		_, err = tx.Exec(ctx, `UPDATE calls SET updated_at = NOW() WHERE id = $1`, callID)
		return err
	})
}

func (r *Repository) Leave(ctx context.Context, code string, userID int64) (*Call, error) {
	return r.changeLocked(ctx, code, func(tx pgx.Tx, callID, _ int64, status Status) error {
		if status == StatusEnded {
			return ErrCallEnded
		}
		result, err := tx.Exec(ctx,
			`UPDATE call_participants SET left_at = NOW()
			 WHERE call_id = $1 AND user_id = $2 AND left_at IS NULL`, callID, userID)
		if err != nil {
			return fmt.Errorf("leave call: %w", err)
		}
		if result.RowsAffected() == 0 {
			return ErrNotParticipant
		}
		_, err = tx.Exec(ctx,
			`UPDATE calls SET status = 'waiting', updated_at = NOW() WHERE id = $1`, callID)
		return err
	})
}

func (r *Repository) End(ctx context.Context, code string, userID int64) (*Call, error) {
	return r.changeLocked(ctx, code, func(tx pgx.Tx, callID, hostUserID int64, status Status) error {
		if userID != hostUserID {
			return ErrNotHost
		}
		if status == StatusEnded {
			return ErrCallEnded
		}
		if _, err := tx.Exec(ctx,
			`UPDATE call_participants SET left_at = NOW() WHERE call_id = $1 AND left_at IS NULL`, callID); err != nil {
			return fmt.Errorf("remove call participants: %w", err)
		}
		_, err := tx.Exec(ctx,
			`UPDATE calls SET status = 'ended', ended_at = NOW(), updated_at = NOW() WHERE id = $1`, callID)
		return err
	})
}

type querier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func loadCall(ctx context.Context, db querier, code string) (*Call, error) {
	result := &Call{Participants: []Participant{}}
	err := db.QueryRow(ctx,
		`SELECT id, code, host_user_id, status, created_at, updated_at, ended_at
		 FROM calls WHERE code = $1`, code,
	).Scan(&result.ID, &result.Code, &result.HostUserID, &result.Status,
		&result.CreatedAt, &result.UpdatedAt, &result.EndedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrCallNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get call: %w", err)
	}

	rows, err := db.Query(ctx,
		`SELECT p.user_id, u.name, p.spoken_language, p.heard_language, p.joined_at
		 FROM call_participants p JOIN users u ON u.id = p.user_id
		 WHERE p.call_id = $1 AND p.left_at IS NULL
		 ORDER BY p.joined_at, p.user_id`, result.ID)
	if err != nil {
		return nil, fmt.Errorf("get call participants: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var participant Participant
		if err := rows.Scan(&participant.UserID, &participant.Name, &participant.SpokenLanguage,
			&participant.HeardLanguage, &participant.JoinedAt); err != nil {
			return nil, fmt.Errorf("scan call participant: %w", err)
		}
		result.Participants = append(result.Participants, participant)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate call participants: %w", err)
	}
	return result, nil
}

func (r *Repository) changeLocked(ctx context.Context, code string, change func(pgx.Tx, int64, int64, Status) error) (*Call, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin call update: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var callID, hostUserID int64
	var status Status
	err = tx.QueryRow(ctx,
		`SELECT id, host_user_id, status FROM calls WHERE code = $1 FOR UPDATE`, code,
	).Scan(&callID, &hostUserID, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrCallNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock call: %w", err)
	}
	if err := change(tx, callID, hostUserID, status); err != nil {
		return nil, err
	}
	updated, err := loadCall(ctx, tx, code)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit call update: %w", err)
	}
	return updated, nil
}
