package call

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/institucional/symphonia/backend/internal/database"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRepositoryJoinCapacity(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	defer pool.Close()
	if err := database.RunMigrations(ctx, pool); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	suffix := time.Now().UnixNano()
	userIDs := make([]int64, 3)
	for i := range userIDs {
		err := pool.QueryRow(ctx,
			`INSERT INTO users (name, email, password_hash) VALUES ($1, $2, 'test') RETURNING id`,
			fmt.Sprintf("Call test user %d", i+1), fmt.Sprintf("call-test-%d-%d@example.com", suffix, i),
		).Scan(&userIDs[i])
		if err != nil {
			t.Fatalf("create test user %d: %v", i+1, err)
		}
	}
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM calls WHERE host_user_id = $1`, userIDs[0])
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = ANY($1)`, userIDs)
	}()

	repository := NewRepository(pool)
	sequential, err := repository.Create(ctx, userIDs[0], LanguageEnglish, LanguagePortuguese)
	if err != nil {
		t.Fatalf("create sequential call: %v", err)
	}
	joined, err := repository.Join(ctx, sequential.Code, userIDs[1], LanguagePortuguese, LanguageEnglish)
	if err != nil {
		t.Fatalf("second user join: %v", err)
	}
	if joined.Status != StatusActive || len(joined.Participants) != 2 {
		t.Fatalf("joined call status/participants = %q/%d, want active/2", joined.Status, len(joined.Participants))
	}
	if _, err := repository.Join(ctx, sequential.Code, userIDs[2], LanguageSpanish, LanguageFrench); !errors.Is(err, ErrCallFull) {
		t.Fatalf("third user join error = %v, want ErrCallFull", err)
	}

	concurrent, err := repository.Create(ctx, userIDs[0], LanguageEnglish, LanguagePortuguese)
	if err != nil {
		t.Fatalf("create concurrent call: %v", err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var ready sync.WaitGroup
	ready.Add(2)
	for _, userID := range userIDs[1:] {
		go func(userID int64) {
			ready.Done()
			<-start
			_, err := repository.Join(ctx, concurrent.Code, userID, LanguageEnglish, LanguagePortuguese)
			results <- err
		}(userID)
	}
	ready.Wait()
	close(start)

	successes, fullErrors := 0, 0
	for range 2 {
		err := <-results
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrCallFull):
			fullErrors++
		default:
			t.Fatalf("concurrent join returned unexpected error: %v", err)
		}
	}
	if successes != 1 || fullErrors != 1 {
		t.Fatalf("concurrent joins = %d successes and %d full errors, want 1 and 1", successes, fullErrors)
	}
	result, err := repository.GetByCode(ctx, concurrent.Code)
	if err != nil {
		t.Fatalf("get concurrent call: %v", err)
	}
	if result.Status != StatusActive || len(result.Participants) != 2 {
		t.Fatalf("concurrent call status/participants = %q/%d, want active/2", result.Status, len(result.Participants))
	}
}
