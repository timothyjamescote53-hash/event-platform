package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/yourorg/event-platform/services/ingestion/internal/handler"
	"github.com/yourorg/event-platform/services/ingestion/internal/repository"
	"github.com/yourorg/event-platform/services/ingestion/internal/service"
)

func main() {
	cfg := service.Config{BatchSize: 100, BatchFlushMs: 50}

	producer := repository.NewMemoryProducer()
	deduper := repository.NewMemoryDeduplicator()
	svc := service.NewIngestionService(producer, deduper, nil, cfg)

	h := handler.NewIngestionHandler(svc)

	mux := http.NewServeMux()

	// Register without method prefix for broad compatibility,
	// then check method inside a wrapper
	mux.HandleFunc("/api/v1/events/batch", methodHandler(map[string]http.HandlerFunc{
		"POST": h.IngestBatch,
	}))
	mux.HandleFunc("/api/v1/events/schema", methodHandler(map[string]http.HandlerFunc{
		"GET": h.GetSchema,
	}))
	mux.HandleFunc("/api/v1/events", methodHandler(map[string]http.HandlerFunc{
		"POST": h.IngestEvent,
	}))
	mux.HandleFunc("/healthz/live", h.Liveness)
	mux.HandleFunc("/healthz/ready", h.Readiness)
	mux.HandleFunc("/metrics", h.Metrics)

	port := getEnv("HTTP_PORT", "8080")
	srv := &http.Server{
		Addr:         net.JoinHostPort("", port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go svc.RunBatchFlusher(context.Background())

	go func() {
		slog.Info("Ingestion service started", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	svc.Flush(ctx)
	slog.Info("Stopped")
}

// methodHandler routes to the correct handler based on HTTP method
func methodHandler(handlers map[string]http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h, ok := handlers[strings.ToUpper(r.Method)]; ok {
			h(w, r)
			return
		}
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
// scaffold
// scaffold
// scaffold
// scaffold
// server
// routing
// graceful
// method handler
// batch flush ctx
// flush on shutdown
// scaffold
// server
// routing
// graceful
// method handler
// batch flush ctx
// flush on shutdown
// scaffold
// server
// routing
// graceful
// method handler
// batch flush ctx
// flush on shutdown
// scaffold
// server
// routing
