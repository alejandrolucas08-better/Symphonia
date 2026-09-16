package main

import (
	"log"

	"github.com/institucional/symphonia/backend/internal/config"
	"github.com/institucional/symphonia/backend/internal/database"
	"github.com/institucional/symphonia/backend/internal/server"
)

func main() {
	cfg := config.Load()

	db, err := database.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer db.Close()

	if err := database.Migrate(db, cfg.MigrationsPath); err != nil {
		log.Fatalf("database migration failed: %v", err)
	}

	appServer := server.New(cfg, db)

	log.Printf("Symphonia backend listening on :%s", cfg.Port)
	if err := appServer.ListenAndServe(); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
