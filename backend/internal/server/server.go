package server

import (
	"database/sql"
	"net/http"

	"github.com/institucional/symphonia/backend/internal/config"
	"github.com/institucional/symphonia/backend/internal/health"
)

func New(cfg config.Config, db *sql.DB) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/healthz", health.Handler{DB: db})

	return &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}
}
