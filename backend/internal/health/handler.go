package health

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type Handler struct {
	DB *sql.DB
}

type Response struct {
	Status   string `json:"status"`
	Service  string `json:"service"`
	Database string `json:"database"`
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	statusCode := http.StatusOK
	response := Response{
		Status:   "ok",
		Service:  "symphonia-backend",
		Database: "ok",
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.DB.PingContext(ctx); err != nil {
		statusCode = http.StatusServiceUnavailable
		response.Status = "unhealthy"
		response.Database = "unavailable"
		log.Printf("health check database ping failed: %v", err)
	}

	writeJSON(w, statusCode, response)
}

func writeJSON(w http.ResponseWriter, statusCode int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("failed to encode JSON response: %v", err)
	}
}
