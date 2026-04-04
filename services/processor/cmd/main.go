package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"os"
	"os/signal"
	"sort"
	"sync"
	"syscall"
	"time"
)

// ── Domain ────────────────────────────────────────────────────────────────────

type Event struct {
	ID        string            `json:"id"`
	TenantID  string            `json:"tenant_id"`
	UserID    string            `json:"user_id"`
	EventType string            `json:"event_type"`
	Source    string            `json:"source"`
	Timestamp int64             `json:"timestamp"`
	Version   int               `json:"version"`
	Payload   map[string]any    `json:"payload"`
	Labels    map[string]string `json:"labels"`
}

// ── WindowState ───────────────────────────────────────────────────────────────

type WindowState struct {
	mu          sync.RWMutex
	TenantID    string
	EventType   string
	WindowType  string
	WindowStart int64
	WindowEnd   int64
	Count       int64
	Values      []float64
	Sum         float64
}

func (w *WindowState) Add(value float64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.Count++
	w.Sum += value
	w.Values = append(w.Values, value)
}

func (w *WindowState) P99() float64 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if len(w.Values) == 0 {
		return 0
	}
	sorted := make([]float64, len(w.Values))
	copy(sorted, w.Values)
	sort.Float64s(sorted)
	idx := int(math.Ceil(float64(len(sorted))*0.99)) - 1
	if idx < 0 {
		idx = 0
	}
	return sorted[idx]
}

func (w *WindowState) Avg() float64 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.Count == 0 {
		return 0
	}
	return w.Sum / float64(w.Count)
}

// ── AnomalyDetector ───────────────────────────────────────────────────────────

type AnomalyDetector struct {
	mu      sync.RWMutex
	history map[string][]float64
}

func NewAnomalyDetector() *AnomalyDetector {
	return &AnomalyDetector{history: make(map[string][]float64)}
}

func (a *AnomalyDetector) Check(tenantID, eventType string, currentCount float64) (bool, float64) {
	key := fmt.Sprintf("%s:%s", tenantID, eventType)
	a.mu.Lock()
	defer a.mu.Unlock()

	hist := a.history[key]
	if len(hist) < 10 {
		a.history[key] = append(hist, currentCount)
		return false, 0
	}

	var sum float64
	for _, v := range hist {
		sum += v
	}
	mean := sum / float64(len(hist))

	var variance float64
	for _, v := range hist {
		d := v - mean
		variance += d * d
	}
	stddev := math.Sqrt(variance / float64(len(hist)))

	var zscore float64
	if stddev > 0 {
		zscore = math.Abs(currentCount-mean) / stddev
	}

	a.history[key] = append(hist[1:], currentCount)
	return zscore > 3.0, zscore
}

// ── WindowManager ─────────────────────────────────────────────────────────────

type WindowManager struct {
	mu      sync.RWMutex
	windows map[string]*WindowState
}

func NewWindowManager() *WindowManager {
	wm := &WindowManager{windows: make(map[string]*WindowState)}
	go wm.runExpiry()
	return wm
}

func (wm *WindowManager) ProcessEvent(event *Event) {
	now := time.UnixMilli(event.Timestamp)
	durations := []struct {
		name string
		dur  time.Duration
	}{
		{"tumbling_1m", time.Minute},
		{"tumbling_5m", 5 * time.Minute},
		{"tumbling_1h", time.Hour},
	}

	for _, w := range durations {
		start := now.Truncate(w.dur).UnixMilli()
		end := start + w.dur.Milliseconds()
		key := fmt.Sprintf("%s:%s:%s:%d", event.TenantID, event.EventType, w.name, start)

		wm.mu.Lock()
		state, exists := wm.windows[key]
		if !exists {
			state = &WindowState{
				TenantID:    event.TenantID,
				EventType:   event.EventType,
				WindowType:  w.name,
				WindowStart: start,
				WindowEnd:   end,
			}
			wm.windows[key] = state
		}
		wm.mu.Unlock()

		val := 1.0
		if v, ok := event.Payload["value"]; ok {
			if f, ok := v.(float64); ok {
				val = f
			}
		}
		state.Add(val)
	}
}

func (wm *WindowManager) runExpiry() {
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		cutoff := time.Now().Add(-2 * time.Hour).UnixMilli()
		wm.mu.Lock()
		for key, w := range wm.windows {
			if w.WindowEnd < cutoff {
				delete(wm.windows, key)
			}
		}
		wm.mu.Unlock()
	}
}

// ── AlertSignal ───────────────────────────────────────────────────────────────

type AlertSignal struct {
	TenantID     string
	EventType    string
	Message      string
	ZScore       float64
	CurrentValue float64
	Severity     string
}

// ── Processor ─────────────────────────────────────────────────────────────────

type Processor struct {
	windows  *WindowManager
	anomaly  *AnomalyDetector
	alertsCh chan<- AlertSignal
}

func NewProcessor(alertsCh chan<- AlertSignal) *Processor {
	return &Processor{
		windows:  NewWindowManager(),
		anomaly:  NewAnomalyDetector(),
		alertsCh: alertsCh,
	}
}

func (p *Processor) Process(event *Event) {
	p.windows.ProcessEvent(event)

	key := fmt.Sprintf("%s:%s:tumbling_1m:%d",
		event.TenantID, event.EventType,
		time.UnixMilli(event.Timestamp).Truncate(time.Minute).UnixMilli(),
	)

	p.windows.mu.RLock()
	state, exists := p.windows.windows[key]
	p.windows.mu.RUnlock()

	if !exists {
		return
	}

	isAnomaly, zscore := p.anomaly.Check(event.TenantID, event.EventType, float64(state.Count))
	if isAnomaly {
		select {
		case p.alertsCh <- AlertSignal{
			TenantID:     event.TenantID,
			EventType:    event.EventType,
			Message:      fmt.Sprintf("Anomaly: %s z=%.2f", event.EventType, zscore),
			ZScore:       zscore,
			CurrentValue: float64(state.Count),
			Severity:     "warning",
		}:
		default:
		}
	}
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	alertsCh := make(chan AlertSignal, 1000)
	processor := NewProcessor(alertsCh)

	go func() {
		for alert := range alertsCh {
			slog.Warn("Alert", "tenant", alert.TenantID, "type", alert.EventType, "zscore", alert.ZScore)
		}
	}()

	// Simulate processing a sample event
	sample := &Event{
		ID:        "evt-001",
		TenantID:  "acme",
		EventType: "purchase",
		Timestamp: time.Now().UnixMilli(),
		Payload:   map[string]any{"amount": 99.99},
	}
	_ = json.Marshal // keep import
	processor.Process(sample)

	slog.Info("Processor started")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Processor stopped")
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
// scaffold
// window
// add
// avg
// p99
// anomaly
// history
// multi-tenant
// window mgr
// expiry
// alert signal
// scaffold
// window
// add
// avg
// p99
// anomaly
// history
// multi-tenant
// window mgr
// expiry
// alert signal
// scaffold
// window
// add
// avg
// p99
// anomaly
// history
// multi-tenant
// window mgr
// expiry
// alert signal
// event domain
// window struct
// add method
// avg method
// p99 sort
// p99 index
// anomaly struct
// zscore mean
// zscore stddev
// 3 sigma
// history slide
// zero stddev
// window mgr
// 1m window
// 5m window
// 1h window
// process event
// payload value
// expiry
// alert signal
// emit alert
// non-blocking
// processor struct
// process method
// main workers
// alert forwarder
// context removed
// log anomaly
// log worker
// getenv
// signal handling
// event domain
// window struct
// add method
// avg method
// p99 sort
// p99 index
// anomaly struct
// zscore mean
// zscore stddev
// 3 sigma
// history slide
// zero stddev
// window mgr
// 1m window
// 5m window
// 1h window
// process event
// payload value
// expiry
// alert signal
// emit alert
// non-blocking
// processor struct
// process method
// main workers
// alert forwarder
// context removed
// log anomaly
// log worker
// getenv
// signal handling
// event domain
// window struct
// add method
// avg method
// p99 sort
// p99 index
// anomaly struct
// zscore mean
// zscore stddev
// 3 sigma
// history slide
// zero stddev
// window mgr
// 1m window
// 5m window
// 1h window
// process event
// payload value
// expiry
// alert signal
// emit alert
// non-blocking
// processor struct
// process method
// main workers
// alert forwarder
// context removed
// log anomaly
// log worker
// getenv
// signal handling
// event domain
// window struct
// add method
// avg method
// p99 sort
