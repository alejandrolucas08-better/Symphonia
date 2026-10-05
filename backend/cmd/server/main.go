package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/institucional/symphonia/backend/internal/auth"
	"github.com/institucional/symphonia/backend/internal/call"
	"github.com/institucional/symphonia/backend/internal/database"
	appserver "github.com/institucional/symphonia/backend/internal/server"
	"github.com/institucional/symphonia/backend/internal/translation"
	"github.com/institucional/symphonia/backend/internal/user"
	ws "github.com/institucional/symphonia/backend/internal/websocket"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = os.Getenv("BACKEND_PORT")
	}
	if port == "" {
		port = "8080"
	}
	if err := auth.ValidateConfiguration(); err != nil {
		log.Fatalf("invalid authentication configuration: %v", err)
	}

	ctx := context.Background()

	pool, err := database.Connect(ctx)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()
	if err := database.RunMigrations(ctx, pool); err != nil {
		log.Fatalf("failed to run database migrations: %v", err)
	}

	userRepo := user.NewRepository(pool)
	authHandler := auth.NewHandler(userRepo)
	callRepo := call.NewRepository(pool)
	hub := ws.NewHub()

	translator, err := translation.New(translation.LoadConfig())
	if err != nil {
		log.Fatalf("failed to initialize translation service: %v", err)
	}
	log.Printf("translation provider: %s", translator.Name())

	opener, _ := translator.(translation.SessionOpener)
	callHandler := call.NewHandler(callRepo, hub)
	websocketHandler := ws.NewHandler(callRepo, hub, opener)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", appserver.HealthHandler(pool.Ping))
	mux.HandleFunc("POST /api/register", authHandler.Register)
	mux.HandleFunc("POST /api/login", authHandler.Login)
	mux.HandleFunc("GET /api/session", auth.Chain(authHandler.Session, auth.Middleware))
	mux.HandleFunc("POST /api/calls", auth.Chain(callHandler.Create, auth.Middleware))
	mux.HandleFunc("GET /api/calls/{code}", auth.Chain(callHandler.Get, auth.Middleware))
	mux.HandleFunc("GET /api/calls/{code}/status", auth.Chain(callHandler.Get, auth.Middleware))
	mux.HandleFunc("POST /api/calls/{code}/join", auth.Chain(callHandler.Join, auth.Middleware))
	mux.HandleFunc("PATCH /api/calls/{code}/language", auth.Chain(callHandler.UpdateLanguage, auth.Middleware))
	mux.HandleFunc("POST /api/calls/{code}/leave", auth.Chain(callHandler.Leave, auth.Middleware))
	mux.HandleFunc("POST /api/calls/{code}/end", auth.Chain(callHandler.End, auth.Middleware))
	mux.Handle("GET /api/calls/{code}/ws", websocketHandler)
	if directory := os.Getenv("FRONTEND_DIST"); directory != "" {
		static, err := appserver.StaticHandler(directory)
		if err != nil {
			log.Fatalf("failed to load frontend: %v", err)
		}
		mux.Handle("GET /", static)
	}

	handler := auth.SecurityHeaders(auth.CORS(auth.LogRequest(mux)))

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	log.Printf("Symphonia backend listening on :%s", port)
	shutdownCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-shutdownCtx.Done()
		hub.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("shutdown: %v", err)
		}
	}()
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server failed: %v", err)
	}
	stop()
	<-shutdownDone
}
