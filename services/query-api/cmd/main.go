package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

// ── In-memory window store (replaces Redis in stdlib version) ─────────────────

type WindowMetric struct {
	TenantID    string  `json:"tenant_id"`
	EventType   string  `json:"event_type"`
	WindowType  string  `json:"window_type"`
	WindowStart int64   `json:"window_start"`
	WindowEnd   int64   `json:"window_end"`
	Count       int64   `json:"count"`
	Sum         float64 `json:"sum"`
	Avg         float64 `json:"avg"`
	P99         float64 `json:"p99"`
}

// ── Query Service ─────────────────────────────────────────────────────────────

type QueryService struct{}

func (q *QueryService) GetEventCount(_ context.Context, _, _ string, from, to int64) (int64, float64, error) {
	// In production this queries ClickHouse; here we return a stub
	count := int64(0)
	dur := float64(to-from) / 1000.0
	rate := 0.0
	if dur > 0 {
		rate = float64(count) / dur
	}
	return count, rate, nil
}

func (q *QueryService) GetWindowMetrics(_ context.Context, _, _, _ string) ([]WindowMetric, error) {
	return []WindowMetric{}, nil
}

func (q *QueryService) GetRevenueLast(_ context.Context, _ string, minutes int) (float64, error) {
	return 0.0, nil
}

// ── Handlers ──────────────────────────────────────────────────────────────────

type QueryHandler struct{ svc *QueryService }

func (h *QueryHandler) getEventCount(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "X-Tenant-ID header required"})
		return
	}
	eventType := r.URL.Query().Get("event_type")
	from := parseMillis(r.URL.Query().Get("from"), time.Now().Add(-time.Hour).UnixMilli())
	to := parseMillis(r.URL.Query().Get("to"), time.Now().UnixMilli())

	count, rate, err := h.svc.GetEventCount(r.Context(), tenantID, eventType, from, to)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"tenant_id": tenantID, "event_type": eventType,
		"count": count, "rate_per_second": rate,
		"from": from, "to": to,
	})
}

func (h *QueryHandler) getMetrics(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Header.Get("X-Tenant-ID")
	eventType := r.URL.Query().Get("event_type")
	window := r.URL.Query().Get("window")
	if window == "" {
		window = "tumbling_1m"
	}
	metrics, err := h.svc.GetWindowMetrics(r.Context(), tenantID, eventType, window)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"metrics": metrics, "count": len(metrics)})
}

func (h *QueryHandler) getRevenue(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Header.Get("X-Tenant-ID")
	minutes := clampMinutes(parseIntQ(r.URL.Query().Get("minutes"), 5))
	revenue, err := h.svc.GetRevenueLast(r.Context(), tenantID, minutes)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"tenant_id": tenantID, "revenue": revenue, "window_minutes": minutes,
	})
}

func (h *QueryHandler) getDashboard(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Header.Get("X-Tenant-ID")
	writeJSON(w, http.StatusOK, map[string]any{"tenant_id": tenantID, "status": "ok"})
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func clampMinutes(m int) int {
	if m <= 0 || m >= 1440 {
		return 5
	}
	return m
}

func parseMillis(s string, fallback int64) int64 {
	if v, err := strconv.ParseInt(s, 10, 64); err == nil {
		return v
	}
	return fallback
}

func parseIntQ(s string, fallback int) int {
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	return fallback
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	svc := &QueryService{}
	h := &QueryHandler{svc: svc}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/analytics/events/count", h.getEventCount)
	mux.HandleFunc("/api/v1/analytics/events/metrics", h.getMetrics)
	mux.HandleFunc("/api/v1/analytics/revenue", h.getRevenue)
	mux.HandleFunc("/api/v1/analytics/dashboard", h.getDashboard)
	mux.HandleFunc("/healthz/live", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "alive"})
	})
	mux.HandleFunc("/healthz/ready", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "# query-api metrics")
	})

	port := getEnv("HTTP_PORT", "8082")
	srv := &http.Server{
		Addr:         net.JoinHostPort("", port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	go func() {
		slog.Info("Query API started", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	slog.Info("Query API stopped")
}
// scaffold
// query svc
// count
// metrics
// revenue
// dashboard
// clamp
// scaffold
// query svc
// count
// metrics
// revenue
// dashboard
// clamp
// scaffold
// query svc
// count
// metrics
// revenue
// dashboard
// clamp
// query svc
// event count
// window metrics
// revenue
// handler
// count handler
// metrics handler
// revenue handler
// dashboard handler
// clamp
// parse helpers
// routes
// health
// boundary fix
// log error
// getenv
// graceful
// writeJSON
// query svc
// event count
// window metrics
// revenue
// handler
// count handler
// metrics handler
// revenue handler
// dashboard handler
// clamp
// parse helpers
// routes
// health
// boundary fix
// log error
// getenv
// graceful
// writeJSON
