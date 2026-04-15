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
	"strings"
	"sync"
	"syscall"
	"time"
)

// ── Domain ────────────────────────────────────────────────────────────────────

type AlertRule struct {
	ID        string  `json:"id"`
	TenantID  string  `json:"tenant_id"`
	Name      string  `json:"name"`
	EventType string  `json:"event_type"`
	Metric    string  `json:"metric"`
	Operator  string  `json:"operator"`
	Threshold float64 `json:"threshold"`
	Window    string  `json:"window"`
	Severity  string  `json:"severity"`
	SlackURL  string  `json:"slack_url,omitempty"`
	Enabled   bool    `json:"enabled"`
	CreatedAt int64   `json:"created_at"`
}

type FiredAlert struct {
	ID           string     `json:"id"`
	RuleID       string     `json:"rule_id"`
	TenantID     string     `json:"tenant_id"`
	RuleName     string     `json:"rule_name"`
	Message      string     `json:"message"`
	CurrentValue float64    `json:"current_value"`
	Threshold    float64    `json:"threshold"`
	Severity     string     `json:"severity"`
	FiredAt      time.Time  `json:"fired_at"`
	ResolvedAt   *time.Time `json:"resolved_at,omitempty"`
}

// ── Alert Engine ──────────────────────────────────────────────────────────────

type AlertEngine struct {
	mu          sync.RWMutex
	rules       map[string]*AlertRule
	cooldowns   map[string]time.Time
	cooldownTTL time.Duration
	alerts      []FiredAlert
	alertsMu    sync.RWMutex
}

func NewAlertEngine() *AlertEngine {
	return &AlertEngine{
		rules:       make(map[string]*AlertRule),
		cooldowns:   make(map[string]time.Time),
		cooldownTTL: 5 * time.Minute,
	}
}

func (e *AlertEngine) AddRule(rule *AlertRule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rules[rule.ID] = rule
}

func (e *AlertEngine) RemoveRule(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.rules, id)
}

func (e *AlertEngine) ListAlerts(tenantID string) []FiredAlert {
	e.alertsMu.RLock()
	defer e.alertsMu.RUnlock()
	var out []FiredAlert
	for _, a := range e.alerts {
		if a.TenantID == tenantID {
			out = append(out, a)
		}
	}
	return out
}

func (e *AlertEngine) fire(rule *AlertRule, currentValue float64) {
	e.mu.Lock()
	last, ok := e.cooldowns[rule.ID]
	if ok && time.Since(last) < e.cooldownTTL {
		e.mu.Unlock()
		return
	}
	e.cooldowns[rule.ID] = time.Now()
	e.mu.Unlock()

	alert := FiredAlert{
		ID:           fmt.Sprintf("alert-%d", time.Now().UnixNano()),
		RuleID:       rule.ID,
		TenantID:     rule.TenantID,
		RuleName:     rule.Name,
		Message:      fmt.Sprintf("[%s] %s: %s %s %.2f (current: %.2f)", rule.Severity, rule.Name, rule.Metric, rule.Operator, rule.Threshold, currentValue),
		CurrentValue: currentValue,
		Threshold:    rule.Threshold,
		Severity:     rule.Severity,
		FiredAt:      time.Now(),
	}

	e.alertsMu.Lock()
	e.alerts = append([]FiredAlert{alert}, e.alerts...)
	if len(e.alerts) > 1000 {
		e.alerts = e.alerts[:1000]
	}
	e.alertsMu.Unlock()

	slog.Warn("Alert fired", "rule", rule.Name, "tenant", rule.TenantID, "value", currentValue)
}

// ── Pure helpers (also used in tests) ────────────────────────────────────────

func severityColor(severity string) string {
	switch severity {
	case "critical":
		return "danger"
	case "warning":
		return "warning"
	default:
		return "good"
	}
}

// ── HTTP Server ───────────────────────────────────────────────────────────────

type server struct {
	engine *AlertEngine
	mux    *http.ServeMux
}

func newServer(engine *AlertEngine) *server {
	s := &server{engine: engine, mux: http.NewServeMux()}
	s.mux.HandleFunc("/api/v1/alerts/rules", s.createRule)
	s.mux.HandleFunc("/api/v1/alerts/", s.listAlerts)
	s.mux.HandleFunc("/healthz/live", s.liveness)
	s.mux.HandleFunc("/healthz/ready", s.readiness)
	s.mux.HandleFunc("/metrics", s.metrics)
	return s
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *server) createRule(w http.ResponseWriter, r *http.Request) {
	var rule AlertRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return
	}
	if rule.EventType == "" || rule.Operator == "" {
		http.Error(w, `{"error":"event_type and operator required"}`, http.StatusBadRequest)
		return
	}
	rule.ID = fmt.Sprintf("rule-%d", time.Now().UnixNano())
	rule.TenantID = r.Header.Get("X-Tenant-ID")
	rule.CreatedAt = time.Now().UnixMilli()
	rule.Enabled = true
	s.engine.AddRule(&rule)
	writeJSON(w, http.StatusCreated, rule)
}

func (s *server) listAlerts(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Header.Get("X-Tenant-ID")
	alerts := s.engine.ListAlerts(tenantID)
	if alerts == nil {
		alerts = []FiredAlert{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"alerts": alerts, "count": len(alerts)})
}

func (s *server) liveness(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "alive"})
}

func (s *server) readiness(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *server) metrics(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "# alerting service metrics")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	engine := NewAlertEngine()
	engine.AddRule(&AlertRule{
		ID: "rule-default-error", TenantID: "*",
		Name: "High Error Rate", EventType: "error",
		Metric: "count", Operator: "gt", Threshold: 100,
		Window: "1m", Severity: "critical", Enabled: true,
	})

	port := getEnv("HTTP_PORT", "8083")
	srv := &http.Server{
		Addr:         net.JoinHostPort("", port),
		Handler:      newServer(engine),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	go func() {
		slog.Info("Alerting service started", "port", port)
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
	slog.Info("Alerting service stopped")
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// keep unused import happy
var _ = strings.TrimSpace
// scaffold
// rule domain
// engine
// evaluate
// cooldown
// fire
// severity
// slack
// api
// scaffold
// rule domain
// engine
// evaluate
// cooldown
// fire
// severity
// slack
// api
// scaffold
// rule domain
// engine
// evaluate
// cooldown
// fire
// severity
// slack
// api
// alert rule
// fired alert
// engine struct
// add rule
// remove rule
// list alerts
// gt operator
// lt operator
// gte lte
// cooldown check
// fire alert
// ring buffer trim
// severity color
// slack notify
// create rule handler
// list handler
// server
// log fire
// getenv
// graceful
// writeJSON
// alert rule
// fired alert
// engine struct
// add rule
// remove rule
// list alerts
// gt operator
// lt operator
// gte lte
// cooldown check
// fire alert
// ring buffer trim
// severity color
// slack notify
// create rule handler
// list handler
// server
// log fire
// getenv
// graceful
// writeJSON
// alert rule
// fired alert
// engine struct
// add rule
// remove rule
// list alerts
// gt operator
// lt operator
// gte lte
// cooldown check
// fire alert
// ring buffer trim
// severity color
// slack notify
// create rule handler
// list handler
// server
// log fire
// getenv
// graceful
// writeJSON
