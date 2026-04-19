// Package handler provides HTTP handlers for rag-service.
package handler

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest"
)

// RegisterRoutes registers all the RAG service routes.
func RegisterRoutes(
	server *rest.Server,
	documentsHandler *DocumentsHandler,
	searchHandler *SearchHandler,
	statsHandler *StatsHandler,
) {
	// Document management routes
	server.AddRoute(rest.Route{
		Method: http.MethodPost,
		Path:   "/api/v1/documents/upload",
		Handler: func(w http.ResponseWriter, r *http.Request) {
			documentsHandler.Upload(w, r)
		},
	})

	server.AddRoute(rest.Route{
		Method: http.MethodGet,
		Path:   "/api/v1/documents",
		Handler: func(w http.ResponseWriter, r *http.Request) {
			documentsHandler.List(w, r)
		},
	})

	server.AddRoute(rest.Route{
		Method: http.MethodDelete,
		Path:   "/api/v1/documents/:id",
		Handler: func(w http.ResponseWriter, r *http.Request) {
			documentsHandler.Delete(w, r)
		},
	})

	server.AddRoute(rest.Route{
		Method: http.MethodPost,
		Path:   "/api/v1/documents/:id/reprocess",
		Handler: func(w http.ResponseWriter, r *http.Request) {
			documentsHandler.Reprocess(w, r)
		},
	})

	// Search route
	server.AddRoute(rest.Route{
		Method: http.MethodPost,
		Path:   "/api/v1/search",
		Handler: func(w http.ResponseWriter, r *http.Request) {
			searchHandler.Search(w, r)
		},
	})

	// Stats route
	server.AddRoute(rest.Route{
		Method: http.MethodGet,
		Path:   "/api/v1/stats",
		Handler: func(w http.ResponseWriter, r *http.Request) {
			statsHandler.GetStats(w, r)
		},
	})
}
