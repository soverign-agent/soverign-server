package grpcserver

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"sovereign-ai-compliance/doc-service/model"
	"sovereign-ai-compliance/shared/proto/doc/v1"
)

// docStatusToProto converts string document status to proto enum.
func docStatusToProto(s string) docv1.DocumentStatus {
	switch s {
	case model.StatusGenerating:
		return docv1.DocumentStatus_DOCUMENT_STATUS_GENERATING
	case model.StatusEditing:
		return docv1.DocumentStatus_DOCUMENT_STATUS_EDITING
	case model.StatusApproved:
		return docv1.DocumentStatus_DOCUMENT_STATUS_APPROVED
	case model.StatusPublished:
		return docv1.DocumentStatus_DOCUMENT_STATUS_PUBLISHED
	case model.StatusFailed:
		return docv1.DocumentStatus_DOCUMENT_STATUS_FAILED
	default:
		return docv1.DocumentStatus_DOCUMENT_STATUS_UNSPECIFIED
	}
}

// docStatusFromProto converts proto enum to string document status.
func docStatusFromProto(s docv1.DocumentStatus) string {
	switch s {
	case docv1.DocumentStatus_DOCUMENT_STATUS_GENERATING:
		return model.StatusGenerating
	case docv1.DocumentStatus_DOCUMENT_STATUS_EDITING:
		return model.StatusEditing
	case docv1.DocumentStatus_DOCUMENT_STATUS_APPROVED:
		return model.StatusApproved
	case docv1.DocumentStatus_DOCUMENT_STATUS_PUBLISHED:
		return model.StatusPublished
	case docv1.DocumentStatus_DOCUMENT_STATUS_FAILED:
		return model.StatusFailed
	default:
		return ""
	}
}

// docTypeToProto converts string doc type to proto enum.
func docTypeToProto(t string) docv1.DocType {
	switch t {
	case model.DocTypeAnnexIV:
		return docv1.DocType_DOC_TYPE_ANNEX_IV
	case model.DocTypeRiskReport:
		return docv1.DocType_DOC_TYPE_RISK_REPORT
	case model.DocTypeCompliance:
		return docv1.DocType_DOC_TYPE_COMPLIANCE
	default:
		return docv1.DocType_DOC_TYPE_UNSPECIFIED
	}
}

// docTypeFromProto converts proto enum to string doc type.
func docTypeFromProto(t docv1.DocType) string {
	switch t {
	case docv1.DocType_DOC_TYPE_ANNEX_IV:
		return model.DocTypeAnnexIV
	case docv1.DocType_DOC_TYPE_RISK_REPORT:
		return model.DocTypeRiskReport
	case docv1.DocType_DOC_TYPE_COMPLIANCE:
		return model.DocTypeCompliance
	default:
		return ""
	}
}

// exportFormatToProto converts string format to proto enum.
func exportFormatToProto(f string) docv1.ExportFormat {
	switch f {
	case model.ExportFormatPDF:
		return docv1.ExportFormat_EXPORT_FORMAT_PDF
	case model.ExportFormatDOCX:
		return docv1.ExportFormat_EXPORT_FORMAT_DOCX
	default:
		return docv1.ExportFormat_EXPORT_FORMAT_UNSPECIFIED
	}
}

// exportFormatFromProto converts proto enum to string format.
func exportFormatFromProto(f docv1.ExportFormat) string {
	switch f {
	case docv1.ExportFormat_EXPORT_FORMAT_PDF:
		return model.ExportFormatPDF
	case docv1.ExportFormat_EXPORT_FORMAT_DOCX:
		return model.ExportFormatDOCX
	default:
		return ""
	}
}

// exportStatusToProto converts string export status to proto enum.
func exportStatusToProto(s string) docv1.ExportStatus {
	switch s {
	case model.ExportStatusPending:
		return docv1.ExportStatus_EXPORT_STATUS_PENDING
	case model.ExportStatusProcessing:
		return docv1.ExportStatus_EXPORT_STATUS_PROCESSING
	case model.ExportStatusCompleted:
		return docv1.ExportStatus_EXPORT_STATUS_COMPLETED
	case model.ExportStatusFailed:
		return docv1.ExportStatus_EXPORT_STATUS_FAILED
	default:
		return docv1.ExportStatus_EXPORT_STATUS_UNSPECIFIED
	}
}

// toProtoSubSection converts model SubSection to proto.
func toProtoSubSection(s model.SubSection) *docv1.SubSection {
	return &docv1.SubSection{
		Id:      s.ID,
		Title:   s.Title,
		Content: s.Content,
		Order:   int32(s.Order),
	}
}

// fromProtoSubSection converts proto SubSection to model.
func fromProtoSubSection(s *docv1.SubSection) model.SubSection {
	if s == nil {
		return model.SubSection{}
	}
	return model.SubSection{
		ID:      s.Id,
		Title:   s.Title,
		Content: s.Content,
		Order:   int(s.Order),
	}
}

// toProtoSection converts model Section to proto.
func toProtoSection(s model.Section) *docv1.Section {
	if s.SubSections == nil {
		s.SubSections = []model.SubSection{}
	}
	subs := make([]*docv1.SubSection, len(s.SubSections))
	for i, sub := range s.SubSections {
		subs[i] = toProtoSubSection(sub)
	}
	return &docv1.Section{
		Id:          s.ID,
		Title:       s.Title,
		Content:     s.Content,
		Order:       int32(s.Order),
		Subsections: subs,
	}
}

// fromProtoSection converts proto Section to model.
func fromProtoSection(s *docv1.Section) model.Section {
	if s == nil {
		return model.Section{}
	}
	var subs []model.SubSection
	if len(s.Subsections) > 0 {
		subs = make([]model.SubSection, len(s.Subsections))
		for i, sub := range s.Subsections {
			subs[i] = fromProtoSubSection(sub)
		}
	}
	return model.Section{
		ID:          s.Id,
		Title:       s.Title,
		Content:     s.Content,
		Order:       int(s.Order),
		SubSections: subs,
	}
}

// toProtoDocumentContent converts model DocumentContent to proto.
func toProtoDocumentContent(c model.DocumentContent) *docv1.DocumentContent {
	if c.Sections == nil {
		c.Sections = []model.Section{}
	}
	if c.Metadata == nil {
		c.Metadata = map[string]string{}
	}
	secs := make([]*docv1.Section, len(c.Sections))
	for i, sec := range c.Sections {
		secs[i] = toProtoSection(sec)
	}
	return &docv1.DocumentContent{
		Sections: secs,
		Metadata: c.Metadata,
	}
}

// fromProtoDocumentContent converts proto DocumentContent to model.
func fromProtoDocumentContent(c *docv1.DocumentContent) model.DocumentContent {
	if c == nil {
		return model.DocumentContent{Sections: []model.Section{}, Metadata: map[string]string{}}
	}
	var secs []model.Section
	if len(c.Sections) > 0 {
		secs = make([]model.Section, len(c.Sections))
		for i, sec := range c.Sections {
			secs[i] = fromProtoSection(sec)
		}
	}
	meta := c.Metadata
	if meta == nil {
		meta = map[string]string{}
	}
	return model.DocumentContent{
		Sections: secs,
		Metadata: meta,
	}
}

// toProtoGeneratedDocument converts model GeneratedDocument to proto.
func toProtoGeneratedDocument(d *model.GeneratedDocument) *docv1.GeneratedDocument {
	if d == nil {
		return nil
	}
	return &docv1.GeneratedDocument{
		Id:         d.ID.String(),
		TenantId:   d.TenantID.String(),
		AiSystemId: d.AISystemID.String(),
		DocType:    docTypeToProto(d.DocType),
		Title:      d.Title,
		Content:    toProtoDocumentContent(d.Content),
		Version:    int32(d.Version),
		Status:     docStatusToProto(d.Status),
		CreatedBy:  d.CreatedBy.String(),
		CreatedAt:  timestamppb.New(d.CreatedAt),
		UpdatedAt:  timestamppb.New(d.UpdatedAt),
	}
}

// toProtoDocumentSummary converts model DocumentSummary to proto.
func toProtoDocumentSummary(d model.DocumentSummary) *docv1.DocumentSummary {
	return &docv1.DocumentSummary{
		Id:         d.ID.String(),
		AiSystemId: d.AISystemID.String(),
		DocType:    docTypeToProto(d.DocType),
		Title:      d.Title,
		Version:    int32(d.Version),
		Status:     docStatusToProto(d.Status),
		CreatedAt:  timestamppb.New(d.CreatedAt),
		UpdatedAt:  timestamppb.New(d.UpdatedAt),
	}
}

// toProtoDocumentVersion converts model DocumentVersion to proto.
func toProtoDocumentVersion(v model.DocumentVersion) *docv1.DocumentVersion {
	return &docv1.DocumentVersion{
		Id:            v.ID.String(),
		TenantId:      v.TenantID.String(),
		DocumentId:    v.DocumentID.String(),
		VersionNumber: int32(v.VersionNumber),
		Content:       toProtoDocumentContent(v.Content),
		CreatedBy:     v.CreatedBy.String(),
		CreatedAt:     timestamppb.New(v.CreatedAt),
		ChangeSummary: v.ChangeSummary,
	}
}

// toProtoExportJob converts model ExportJob to proto.
func toProtoExportJob(j *model.ExportJob) *docv1.ExportJob {
	if j == nil {
		return nil
	}
	var filePath string
	if j.FilePath != nil {
		filePath = *j.FilePath
	}
	var fileSize int64
	if j.FileSize != nil {
		fileSize = *j.FileSize
	}
	var errorMsg string
	if j.ErrorMessage != nil {
		errorMsg = *j.ErrorMessage
	}
	var completedAt *timestamppb.Timestamp
	if j.CompletedAt != nil {
		completedAt = timestamppb.New(*j.CompletedAt)
	}
	return &docv1.ExportJob{
		Id:            j.ID.String(),
		TenantId:      j.TenantID.String(),
		DocumentId:    j.DocumentID.String(),
		Format:        exportFormatToProto(j.Format),
		Status:        exportStatusToProto(j.Status),
		FilePath:      filePath,
		FileSize:      fileSize,
		ErrorMessage:  errorMsg,
		CreatedBy:     j.CreatedBy.String(),
		CreatedAt:     timestamppb.New(j.CreatedAt),
		CompletedAt:   completedAt,
	}
}

// strPtr returns a pointer to s if non-empty, else nil.
func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// timeToProto converts *time.Time to proto timestamp.
func timeToProto(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}
