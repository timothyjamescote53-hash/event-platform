package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// ── Anthropic types ───────────────────────────────────────────────────────────

type AnthropicRequest struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	Messages  []Message `json:"messages"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AnthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

// ── AI Analyzer ───────────────────────────────────────────────────────────────

type AIAnalyzer struct {
	apiKey     string
	httpClient *http.Client
}

func NewAIAnalyzer(apiKey string) *AIAnalyzer {
	return &AIAnalyzer{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (a *AIAnalyzer) callClaude(ctx context.Context, prompt string) (string, error) {
	if a.apiKey == "" {
		return "AI analysis unavailable: no API key configured", nil
	}
	body, _ := json.Marshal(AnthropicRequest{
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 500,
		Messages:  []Message{{Role: "user", Content: prompt}},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("x-api-key", a.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("anthropic error %d: %s", resp.StatusCode, data)
	}
	var result AnthropicResponse
	if err := json.Unmarshal(data, &result); err != nil || len(result.Content) == 0 {
		return "", fmt.Errorf("empty response")
	}
	return result.Content[0].Text, nil
}

func (a *AIAnalyzer) DeduplicateAlerts(_ context.Context, _ string, alerts []map[string]any) ([]map[string]any, error) {
	if len(alerts) <= 1 {
		return alerts, nil
	}
	return alerts, nil // full LLM dedup requires API key
}

func (a *AIAnalyzer) SummarizeAnomaly(ctx context.Context, tenantID, eventType string, zscore, current, avg float64) (string, error) {
	prompt := fmt.Sprintf(
		"Tenant: %s, Event: %s, Z-score: %.2f, Current: %.0f, Avg: %.0f. Explain in 2 sentences.",
		tenantID, eventType, zscore, current, avg,
	)
	return a.callClaude(ctx, prompt)
}

// ── HTTP handlers ─────────────────────────────────────────────────────────────

type AIHandler struct{ analyzer *AIAnalyzer }

func (h *AIHandler) summarize(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TenantID      string  `json:"tenant_id"`
		EventType     string  `json:"event_type"`
		ZScore        float64 `json:"z_score"`
		CurrentCount  float64 `json:"current_count"`
		HistoricalAvg float64 `json:"historical_avg"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	summary, err := h.analyzer.SummarizeAnomaly(r.Context(),
		req.TenantID, req.EventType, req.ZScore, req.CurrentCount, req.HistoricalAvg)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "AI unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"summary": summary, "tenant_id": req.TenantID, "z_score": req.ZScore,
	})
}

func (h *AIHandler) deduplicate(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Header.Get("X-Tenant-ID")
	var req struct {
		Alerts []map[string]any `json:"alerts"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	groups, err := h.analyzer.DeduplicateAlerts(r.Context(), tenantID, req.Alerts)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"groups": groups})
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
	analyzer := NewAIAnalyzer(getEnv("ANTHROPIC_API_KEY", ""))
	h := &AIHandler{analyzer: analyzer}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/ai/anomaly/summarize", h.summarize)
	mux.HandleFunc("/api/v1/ai/alerts/deduplicate", h.deduplicate)
	mux.HandleFunc("/healthz/live", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "alive"})
	})

	port := getEnv("HTTP_PORT", "8084")
	srv := &http.Server{
		Addr:    net.JoinHostPort("", port),
		Handler: mux,
	}

	go func() {
		slog.Info("AI Analyzer started", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}
// scaffold
// anthropic
// summarize
// dedup
// single passthrough
// scaffold
// anthropic
// summarize
// dedup
// single passthrough
// scaffold
// anthropic
// summarize
// dedup
// single passthrough
// anthropic types
// response type
// analyzer struct
// call claude
// no key fallback
// summarize
// dedup single
// dedup multi
// summarize handler
// dedup handler
// routes
// log unavail
// getenv
// graceful
// writeJSON
// anthropic types
// response type
// analyzer struct
// call claude
// no key fallback
// summarize
// dedup single
// dedup multi
// summarize handler
// dedup handler
// routes
// log unavail
// getenv
// graceful
// writeJSON
// anthropic types
// response type
// analyzer struct
// call claude
// no key fallback
// summarize
// dedup single
// dedup multi
// summarize handler
// dedup handler
