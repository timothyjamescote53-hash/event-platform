package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/yourorg/event-platform/services/ingestion/internal/service"
)

type IngestionHandler struct{ svc *service.IngestionService }

func NewIngestionHandler(svc *service.IngestionService) *IngestionHandler {
	return &IngestionHandler{svc: svc}
}

type ingestRequest struct {
	ID        string            `json:"id"`
	TenantID  string            `json:"tenant_id"`
	UserID    string            `json:"user_id"`
	EventType string            `json:"event_type"`
	Source    string            `json:"source"`
	Timestamp int64             `json:"timestamp"`
	Payload   map[string]any    `json:"payload"`
	Labels    map[string]string `json:"labels"`
}

func (h *IngestionHandler) IngestEvent(w http.ResponseWriter, r *http.Request) {
	var req ingestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		tenantID = req.TenantID
	}

	event := &service.Event{
		ID: req.ID, TenantID: tenantID, UserID: req.UserID,
		EventType: req.EventType, Source: req.Source,
		Timestamp: req.Timestamp, Payload: req.Payload, Labels: req.Labels,
	}

	if err := h.svc.IngestEvent(r.Context(), event); err != nil {
		switch err {
		case service.ErrDuplicateEvent:
			writeJSON(w, http.StatusOK, map[string]string{"status": "duplicate", "id": event.ID})
		case service.ErrInvalidEvent, service.ErrTenantRequired:
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		default:
			slog.Error("IngestEvent failed", "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		}
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted", "id": event.ID})
}

func (h *IngestionHandler) IngestBatch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Events []ingestRequest `json:"events"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if len(req.Events) == 0 || len(req.Events) > 1000 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "events must be 1-1000"})
		return
	}

	tenantID := r.Header.Get("X-Tenant-ID")
	events := make([]*service.Event, len(req.Events))
	for i, r2 := range req.Events {
		tid := tenantID
		if tid == "" {
			tid = r2.TenantID
		}
		events[i] = &service.Event{
			ID: r2.ID, TenantID: tid, UserID: r2.UserID,
			EventType: r2.EventType, Source: r2.Source,
			Timestamp: r2.Timestamp, Payload: r2.Payload, Labels: r2.Labels,
		}
	}

	accepted, rejected, errs := h.svc.IngestBatch(r.Context(), events)
	status := http.StatusAccepted
	if accepted == 0 {
		status = http.StatusBadRequest
	}
	writeJSON(w, status, map[string]any{"accepted": accepted, "rejected": rejected, "errors": errs})
}

func (h *IngestionHandler) GetSchema(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"version": 1,
		"fields": map[string]string{
			"id": "string (auto-generated)", "tenant_id": "string (required)",
			"event_type": "string (required)", "timestamp": "int64 (unix millis)",
			"payload": "object", "labels": "map<string,string>",
		},
	})
}

func (h *IngestionHandler) Liveness(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "alive"})
}

func (h *IngestionHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (h *IngestionHandler) Metrics(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "# ingestion service metrics\ningestion_uptime_seconds %.0f\n",
		time.Since(startTime).Seconds())
}

var startTime = time.Now()

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
// ingest
// batch
// schema
// ingest
// batch
// schema
// ingest
// batch
// schema
// ingest single
// tenant header
// error responses
// batch handler
// schema handler
// health
// metrics
// uptime
// log request
// writeJSON
// ingest single
// tenant header
// error responses
// batch handler
// schema handler
// health
// metrics
// uptime
// log request
// writeJSON
// ingest single
// tenant header
// error responses
// batch handler
// schema handler
// health
// metrics
// uptime
// log request
// writeJSON
// ingest single
