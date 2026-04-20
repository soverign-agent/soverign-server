package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"sovereign-ai-compliance/audit-service/internal/types"
	"sovereign-ai-compliance/audit-service/model"
	"sovereign-ai-compliance/audit-service/repo"
	"sovereign-ai-compliance/audit-service/temporal"
)

// SSEHandler handles Server-Sent Events for real-time audit progress streaming.
type SSEHandler struct {
	repo     repo.Repository
	temporal *temporal.Client
}

// NewSSEHandler creates a new SSEHandler.
func NewSSEHandler(repo repo.Repository, temporal *temporal.Client) *SSEHandler {
	return &SSEHandler{
		repo:     repo,
		temporal: temporal,
	}
}

// StreamProgress streams audit progress via Server-Sent Events.
func (h *SSEHandler) StreamProgress(w http.ResponseWriter, r *http.Request) {
	// Get audit ID from URL
	idStr := r.PathValue("id")
	auditID, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid audit ID: %v", err), http.StatusBadRequest)
		return
	}

	// Check audit exists
	audit, err := h.repo.GetAuditByID(r.Context(), auditID)
	if err != nil {
		http.Error(w, fmt.Sprintf("get audit: %v", err), http.StatusInternalServerError)
		return
	}
	if audit == nil {
		http.Error(w, "audit not found", http.StatusNotFound)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// If audit is already completed, send final event and close
	if audit.Status == model.AuditJobStatusCompleted || audit.Status == model.AuditJobStatusFailed || audit.Status == model.AuditJobStatusCancelled {
		sendSSEEvent(w, "progress", types.SSEProgressEvent{
			Percentage: 100,
			Step:       audit.Status,
			Message:    fmt.Sprintf("Audit %s", audit.Status),
			Timestamp:  time.Now().Unix(),
		})
		flusher.Flush()
		return
	}

	// If workflow ID is set, query Temporal for status and stream updates
	var progressCh chan types.SSEProgressEvent
	if audit.WorkflowID != nil {
		progressCh = make(chan types.SSEProgressEvent)
		go h.pollForProgress(r.Context(), *audit.WorkflowID, audit.ProgressPercentage, progressCh)
	} else {
		// Just send current progress and close
		sendSSEEvent(w, "progress", types.SSEProgressEvent{
			Percentage: audit.ProgressPercentage,
			Step:       audit.Status,
			Message:    fmt.Sprintf("Audit is %s", audit.Status),
			Timestamp:  time.Now().Unix(),
		})
		flusher.Flush()
		return
	}

	// Stream progress updates until connection closes or audit completes
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return
		case event := <-progressCh:
			sendSSEEvent(w, "progress", event)
			flusher.Flush()

			if event.Percentage >= 100 {
				return
			}
		}
	}
}

func (h *SSEHandler) pollForProgress(ctx context.Context, workflowID string, currentProgress int, out chan<- types.SSEProgressEvent) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	lastProgress := currentProgress
	lastStep := "initializing"

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Query Temporal for current status
			var result types.GetAuditStatusResult
			encoded, err := h.temporal.Client().QueryWorkflow(ctx, workflowID, "", "GetAuditStatusQuery")
			if err != nil {
				// Temporal might not be ready yet, just keep going
				continue
			}
			if err := encoded.Get(&result); err != nil {
				// Temporal might not be ready yet, just keep going
				continue
			}

			if result.Progress != lastProgress || result.CurrentStep != lastStep {
				out <- types.SSEProgressEvent{
					Percentage: result.Progress,
					Step:       result.CurrentStep,
					Message:    fmt.Sprintf("%s in progress", result.CurrentStep),
					Timestamp:  time.Now().Unix(),
				}
				lastProgress = result.Progress
				lastStep = result.CurrentStep

				if result.Progress >= 100 {
					return
				}
			}
		}
	}
}

func sendSSEEvent(w http.ResponseWriter, event string, data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return
	}

	fmt.Fprintf(w, "event: %s\n", event)
	fmt.Fprintf(w, "data: %s\n\n", jsonData)
}
