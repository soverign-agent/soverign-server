package grpcserver

import (
	"database/sql"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"sovereign-ai-compliance/repo-service/model"
	"sovereign-ai-compliance/shared/proto/repo/v1"
)

// providerToProto converts model Provider string to proto Provider enum.
func providerToProto(p model.Provider) repov1.Provider {
	switch p {
	case model.ProviderGitHub:
		return repov1.Provider_PROVIDER_GITHUB
	case model.ProviderGitLab:
		return repov1.Provider_PROVIDER_GITLAB
	case model.ProviderSelfHosted:
		return repov1.Provider_PROVIDER_SELF_HOSTED
	default:
		return repov1.Provider_PROVIDER_UNSPECIFIED
	}
}

// providerFromProto converts proto Provider enum to model Provider string.
func providerFromProto(p repov1.Provider) model.Provider {
	switch p {
	case repov1.Provider_PROVIDER_GITHUB:
		return model.ProviderGitHub
	case repov1.Provider_PROVIDER_GITLAB:
		return model.ProviderGitLab
	case repov1.Provider_PROVIDER_SELF_HOSTED:
		return model.ProviderSelfHosted
	default:
		return ""
	}
}

// toProtoSafeRepository converts model.SafeRepository to proto SafeRepository.
func toProtoSafeRepository(r model.SafeRepository) *repov1.SafeRepository {
	return &repov1.SafeRepository{
		Id:            r.ID.String(),
		TenantId:      r.TenantID.String(),
		Name:          r.Name,
		Url:           r.URL,
		Provider:      providerToProto(r.Provider),
		WebhookId:     nullString(r.WebhookID),
		DefaultBranch: r.DefaultBranch,
		LastScannedAt: nullTimeToProto(r.LastScannedAt),
		CreatedAt:     timestamppb.New(r.CreatedAt),
		UpdatedAt:     timestamppb.New(r.UpdatedAt),
	}
}

// toProtoScanResult converts model.ScanResult to proto ScanResult.
func toProtoScanResult(r model.ScanResult) *repov1.ScanResult {
	return &repov1.ScanResult{
		Id:           r.ID.String(),
		RepositoryId: r.RepositoryID.String(),
		TenantId:     r.TenantID.String(),
		Branch:       r.Branch,
		CommitHash:   r.CommitHash,
		AiUses:       string(r.AIUses),
		DataFlows:    string(r.DataFlows),
		SensitiveData: string(r.SensitiveData),
		TotalFiles:   int32(r.TotalFiles),
		ScannedFiles: int32(r.ScannedFiles),
		StartedAt:    timestamppb.New(r.StartedAt),
		CompletedAt:  nullTimeToProto(r.CompletedAt),
		CreatedAt:    timestamppb.New(r.CreatedAt),
	}
}

// nullTimeToProto converts sql.NullTime to *timestamppb.Timestamp.
func nullTimeToProto(t sql.NullTime) *timestamppb.Timestamp {
	if !t.Valid {
		return nil
	}
	return timestamppb.New(t.Time)
}

// nullString converts sql.NullString to string.
func nullString(s sql.NullString) string {
	if !s.Valid {
		return ""
	}
	return s.String
}

// timeToProto converts *time.Time to *timestamppb.Timestamp.
func timeToProto(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}

// strPtr returns a pointer to s if non-empty, else nil.
func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
