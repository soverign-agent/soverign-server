// Package handler provides REST handlers for the document service.
package handler

import (
	"github.com/zeromicro/go-zero/rest"
)

// RegisterRoutes registers all document service routes.
func RegisterRoutes(server *rest.Server, documentHandler *DocumentHandler, exportHandler *ExportHandler) {
	server.AddRoutes(
		[]rest.Route{
			// Document routes
			{Method: "GET", Path: "/api/v1/generated-documents", Handler: documentHandler.ListDocuments},
			{Method: "POST", Path: "/api/v1/generated-documents", Handler: documentHandler.GenerateDocument},
			{Method: "GET", Path: "/api/v1/generated-documents/:id", Handler: documentHandler.GetDocument},
			{Method: "PUT", Path: "/api/v1/generated-documents/:id", Handler: documentHandler.UpdateDocument},
			{Method: "GET", Path: "/api/v1/generated-documents/:id/versions", Handler: documentHandler.ListVersions},
			{Method: "POST", Path: "/api/v1/generated-documents/:id/rollback", Handler: documentHandler.RollbackVersion},

			// Export routes
			{Method: "POST", Path: "/api/v1/generated-documents/:id/export", Handler: exportHandler.CreateExportJob},
			{Method: "GET", Path: "/api/v1/export-jobs/:id", Handler: exportHandler.GetExportJobStatus},
			{Method: "GET", Path: "/api/v1/export-jobs/:id/download", Handler: exportHandler.DownloadExport},
		},
	)
}
