package orchestrator

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"

	sharedconfig "sovereign-ai-compliance/shared/config"
	"sovereign-ai-compliance/shared/llm"
	agentv1 "sovereign-ai-compliance/shared/proto/agent/v1"
)

type fakeLLM struct {
	resp llm.StreamCompletionResponse
	err  error
	got  llm.CompletionRequest
}

func (f *fakeLLM) Complete(ctx context.Context, req llm.CompletionRequest) (llm.CompletionResponse, error) {
	return llm.CompletionResponse{}, errors.New("not used")
}

func (f *fakeLLM) StreamComplete(ctx context.Context, req llm.CompletionRequest, onDelta func(string)) (llm.StreamCompletionResponse, error) {
	f.got = req
	return f.resp, f.err
}

func (f *fakeLLM) Embed(ctx context.Context, req llm.EmbeddingRequest) (llm.EmbeddingResponse, error) {
	return llm.EmbeddingResponse{}, errors.New("not used")
}

func (f *fakeLLM) Health(ctx context.Context) error { return nil }

func TestSupervisor_Invoke(t *testing.T) {
	t.Parallel()
	successResp := llm.StreamCompletionResponse{
		Content:      "Hello tenant",
		FinishReason: "stop",
		Usage:        llm.Usage{PromptTokens: 4, CompletionTokens: 2, TotalTokens: 6},
		Timings: llm.StreamTimings{
			TTFT:          100 * time.Millisecond,
			TPOT:          20 * time.Millisecond,
			TotalDuration: 250 * time.Millisecond,
		},
	}

	cases := []struct {
		name        string
		payload     []byte
		llmResp     llm.StreamCompletionResponse
		llmErr      error
		nilClient   bool
		wantStatus  agentv1.AgentResponse_Status
		wantContent string
		wantErrMsg  string
	}{
		{
			name:        "messages payload success",
			payload:     []byte(`{"messages":[{"role":"user","content":"hi"}]}`),
			llmResp:     successResp,
			wantStatus:  agentv1.AgentResponse_STATUS_COMPLETED,
			wantContent: "Hello tenant",
		},
		{
			name:        "prompt payload success",
			payload:     []byte(`{"prompt":"sup"}`),
			llmResp:     successResp,
			wantStatus:  agentv1.AgentResponse_STATUS_COMPLETED,
			wantContent: "Hello tenant",
		},
		{
			name:        "raw text payload success",
			payload:     []byte(`hello world`),
			llmResp:     successResp,
			wantStatus:  agentv1.AgentResponse_STATUS_COMPLETED,
			wantContent: "Hello tenant",
		},
		{
			name:       "empty payload fails",
			payload:    nil,
			wantStatus: agentv1.AgentResponse_STATUS_FAILED,
			wantErrMsg: "empty payload",
		},
		{
			name:       "llm error -> failed",
			payload:    []byte(`{"prompt":"x"}`),
			llmErr:     errors.New("boom"),
			wantStatus: agentv1.AgentResponse_STATUS_FAILED,
			wantErrMsg: "boom",
		},
		{
			name:       "no llm client",
			payload:    []byte(`{"prompt":"x"}`),
			nilClient:  true,
			wantStatus: agentv1.AgentResponse_STATUS_FAILED,
			wantErrMsg: "llm client not configured",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fake := &fakeLLM{resp: tc.llmResp, err: tc.llmErr}
			var clientIface llm.Client = fake
			if tc.nilClient {
				clientIface = nil
			}

			s := NewSupervisor(nil, zap.NewNop(), sharedconfig.LLMConfig{Model: "gpt-4o"}, clientIface, nil, nil)

			req := &agentv1.AgentRequest{
				RequestId: "req-1",
				AgentId:   "agent-1",
				TenantId:  "tenant-A",
				Payload:   tc.payload,
			}
			resp, err := s.Invoke(context.Background(), req)
			if err != nil {
				t.Fatalf("Invoke returned err: %v", err)
			}
			if resp.GetStatus() != tc.wantStatus {
				t.Errorf("status=%v want=%v err=%q", resp.GetStatus(), tc.wantStatus, resp.GetErrorMessage())
			}
			if resp.GetRequestId() != "req-1" || resp.GetAgentId() != "agent-1" {
				t.Errorf("ids not echoed: %+v", resp)
			}
			if tc.wantContent != "" && string(resp.GetResult()) != tc.wantContent {
				t.Errorf("content=%q want=%q", resp.GetResult(), tc.wantContent)
			}
			if tc.wantErrMsg != "" && !contains(resp.GetErrorMessage(), tc.wantErrMsg) {
				t.Errorf("error_message=%q want substring %q", resp.GetErrorMessage(), tc.wantErrMsg)
			}
			if tc.wantStatus == agentv1.AgentResponse_STATUS_COMPLETED {
				if md := resp.GetMetadata(); md["finish_reason"] != "stop" || md["completion_tokens"] != "2" {
					t.Errorf("metadata wrong: %+v", md)
				}
				if fake.got.Model != "gpt-4o" {
					t.Errorf("model not propagated: %q", fake.got.Model)
				}
				if len(fake.got.Messages) == 0 {
					t.Errorf("messages empty")
				}
			}
		})
	}
}

func contains(s, sub string) bool {
	if sub == "" {
		return true
	}
	return len(s) >= len(sub) && (s == sub || (len(s) > 0 && (indexOf(s, sub) >= 0)))
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
