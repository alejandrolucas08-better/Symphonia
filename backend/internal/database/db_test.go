package database

import (
	"context"
	"testing"

	"github.com/institucional/symphonia/backend/migrations"
)

func TestProductionRequiresDatabaseURL(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "")
	if _, err := Connect(context.Background()); err == nil {
		t.Fatal("production must not use local default credentials")
	}
}

func TestMigrationsAreEmbedded(t *testing.T) {
	for _, name := range []string{"001_create_users.sql", "002_create_calls.sql", "003_add_users_updated_at.sql", "004_migrate_legacy_uuid_schema.sql"} {
		data, err := migrations.Files.ReadFile(name)
		if err != nil || len(data) == 0 {
			t.Fatalf("migration %s not embedded: %v", name, err)
		}
	}
}
