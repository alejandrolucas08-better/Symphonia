package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

func HealthHandler(ping func(context.Context) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		status, code := "ok", http.StatusOK
		if err := ping(ctx); err != nil {
			status, code = "unavailable", http.StatusServiceUnavailable
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": status, "service": "symphonia-backend"})
	}
}
