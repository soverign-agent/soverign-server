// Package handler provides HTTP handlers for rag-service.
package handler

import (
	"net/http"

	"sovereign-ai-compliance/rag-service/internal/logic"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// SearchHandler handles RAG search HTTP requests.
type SearchHandler struct {
	logic *logic.SearchLogic
}

// NewSearchHandler creates a new SearchHandler.
func NewSearchHandler(logic *logic.SearchLogic) *SearchHandler {
	return &SearchHandler{
		logic: logic,
	}
}

// Search performs a similarity search for the given query.
func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	var req logic.SearchRequest
	if err := httpx.Parse(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}

	resp, err := h.logic.Search(r.Context(), req)
	if err != nil {
		httpx.Error(w, err)
		return
	}

	httpx.OkJson(w, resp)
}
