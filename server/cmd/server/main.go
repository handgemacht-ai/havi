package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/handgemacht-ai/annotation-plugin/server/internal/controller"
	"github.com/handgemacht-ai/annotation-plugin/server/internal/db"
	"github.com/handgemacht-ai/annotation-plugin/server/internal/middleware"
	"github.com/handgemacht-ai/annotation-plugin/server/internal/repo"
	"github.com/handgemacht-ai/annotation-plugin/server/internal/service"
	"github.com/handgemacht-ai/annotation-plugin/server/internal/webhook"
)

func main() {
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8090"
	}

	dbURL := os.Getenv("SERVER_DB_URL")
	if dbURL == "" {
		log.Fatal("SERVER_DB_URL is required")
	}

	corsOrigins := os.Getenv("CORS_ORIGINS")
	webhookURL := os.Getenv("WEBHOOK_URL")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := db.Connect(ctx, dbURL)
	if err != nil {
		log.Fatalf("db connect error=%v", err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool, "migrations"); err != nil {
		log.Fatalf("migration error=%v", err)
	}

	annotationRepo := repo.NewPostgresRepo(pool)
	baseURL := "http://localhost:" + port
	annotationService := service.NewAnnotationService(annotationRepo, baseURL)
	wh := webhook.NewWebhook(webhookURL)
	ctrl := controller.NewAnnotationController(annotationService, wh)

	mux := http.NewServeMux()
	controller.RegisterRoutes(mux, ctrl)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	handler := middleware.CORS(corsOrigins, mux)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: handler,
	}

	sigCtx, sigCancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer sigCancel()

	go func() {
		log.Printf("server starting port=%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error=%v", err)
		}
	}()

	<-sigCtx.Done()
	log.Println("shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error=%v", err)
	}
}
