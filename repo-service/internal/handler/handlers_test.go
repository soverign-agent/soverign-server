package handler

import "testing"

func TestBranchFromWebhookPayload(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    string
	}{
		{
			name:    "github branch ref",
			payload: `{"ref":"refs/heads/main"}`,
			want:    "main",
		},
		{
			name:    "gitlab branch ref",
			payload: `{"ref":"refs/heads/release/v1"}`,
			want:    "release/v1",
		},
		{
			name:    "plain ref",
			payload: `{"ref":"main"}`,
			want:    "main",
		},
		{
			name:    "invalid payload",
			payload: `{`,
			want:    "",
		},
		{
			name:    "missing ref",
			payload: `{}`,
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := branchFromWebhookPayload([]byte(tt.payload)); got != tt.want {
				t.Fatalf("branchFromWebhookPayload() = %q, want %q", got, tt.want)
			}
		})
	}
}
