// Package handler provides HTTP handlers for rag-service.
package handler

import (
	"net/http"

	"sovereign-ai-compliance/rag-service/internal/logic"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// StatsHandler handles RAG statistics HTTP requests.
type StatsHandler struct {
	logic *logic.StatsLogic
}

// NewStatsHandler creates a new StatsHandler.
func NewStatsHandler(logic *logic.StatsLogic) *StatsHandler {
	return &StatsHandler{
		logic: logic,
	}
}

// GetStats gets RAG statistics for the current tenant.
func (h *StatsHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	resp, err := h.logic.GetStats(r.Context())
	if err != nil {
		httpx.Error(w, err)
		return
	}

	httpx.OkJson(w, resp)
}
