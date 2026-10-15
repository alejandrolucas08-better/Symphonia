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
	authStore := auth.NewStore(pool)
	publicURL := os.Getenv("APP_PUBLIC_URL")
	if publicURL == "" && os.Getenv("APP_ENV") != "production" {
		publicURL = "http://localhost:5173"
	}
	authHandler := auth.NewHandler(userRepo, authStore, auth.MailerFromEnv(), publicURL)
	googleOAuth := auth.NewGoogleOAuth(authStore, publicURL)
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
	websocketHandler.SetSessionValidator(func(ctx context.Context, token string) (int64, error) {
		return auth.ValidateSession(ctx, authStore, token)
	})
	requireAuth := auth.MiddlewareWithStore(authStore)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", appserver.HealthHandler(pool.Ping))
	mux.HandleFunc("POST /api/register", authHandler.Register)
	mux.HandleFunc("POST /api/login", authHandler.Login)
	mux.HandleFunc("POST /api/auth/resend-verification", authHandler.ResendVerification)
	mux.HandleFunc("POST /api/auth/verify-email", authHandler.VerifyEmail)
	mux.HandleFunc("POST /api/auth/forgot-password", authHandler.ForgotPassword)
	mux.HandleFunc("POST /api/auth/reset-password", authHandler.ResetPassword)
	mux.HandleFunc("POST /api/auth/change-password", requireAuth(authHandler.ChangePassword))
	mux.HandleFunc("GET /api/auth/providers", googleOAuth.Providers)
	mux.HandleFunc("GET /api/auth/google/start", googleOAuth.Start)
	mux.HandleFunc("POST /api/auth/google/link/start", requireAuth(googleOAuth.StartLink))
	mux.HandleFunc("GET /api/auth/google/callback", googleOAuth.Callback)
	mux.HandleFunc("POST /api/auth/google/finish", authHandler.FinishGoogle)
	mux.HandleFunc("GET /api/session", requireAuth(authHandler.Session))
	mux.HandleFunc("POST /api/calls", requireAuth(callHandler.Create))
	mux.HandleFunc("GET /api/calls/{code}", requireAuth(callHandler.Get))
	mux.HandleFunc("GET /api/calls/{code}/status", requireAuth(callHandler.Get))
	mux.HandleFunc("POST /api/calls/{code}/join", requireAuth(callHandler.Join))
	mux.HandleFunc("PATCH /api/calls/{code}/language", requireAuth(callHandler.UpdateLanguage))
	mux.HandleFunc("POST /api/calls/{code}/leave", requireAuth(callHandler.Leave))
	mux.HandleFunc("POST /api/calls/{code}/end", requireAuth(callHandler.End))
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
